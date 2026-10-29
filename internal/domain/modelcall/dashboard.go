package modelcall

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// DashboardStats dashboard 统计块：一个时间窗（或一个时间桶）内的全部 KPI 原始量。
//
// 口径（spec §4）：Requests 与失败计数统计全部调用；Token/费用/延迟/速度只统计成功调用；
// 分位数与速度在无样本时为 nil（不得用 0 冒充）。
type DashboardStats struct {
	Requests              int64    `gorm:"column:requests"`
	Success               int64    `gorm:"column:success"`
	RateLimitFailures     int64    `gorm:"column:rate_limit_failures"`
	ServerFailures        int64    `gorm:"column:server_failures"`
	ConnectionFailures    int64    `gorm:"column:connection_failures"`
	InputTokens           int64    `gorm:"column:input_tokens"`
	OutputTokens          int64    `gorm:"column:output_tokens"`
	CacheReadTokens       int64    `gorm:"column:cache_read_tokens"`
	CacheWriteTokens      int64    `gorm:"column:cache_write_tokens"`
	CostMicro             int64    `gorm:"column:cost_micro"`
	PricedSuccess         int64    `gorm:"column:priced_success"`
	LatencyP50Ms          *float64 `gorm:"column:latency_p50_ms"`
	LatencyP95Ms          *float64 `gorm:"column:latency_p95_ms"`
	LatencyP99Ms          *float64 `gorm:"column:latency_p99_ms"`
	OutputTokensPerSecond *float64 `gorm:"column:output_tokens_per_second"`
	AvgStreamDurationMs   *float64 `gorm:"column:avg_stream_duration_ms"`
}

// DashboardStatsPoint 按时间桶的统计块
type DashboardStatsPoint struct {
	Time           time.Time `gorm:"column:time"`
	DashboardStats `gorm:"embedded"`
}

// DashboardUptimePoint 可用性状态条的一个时间桶（只统计请求数与失败拆分）
type DashboardUptimePoint struct {
	Time time.Time `gorm:"column:time"`
	// End 桶终点（补桶时算出，不来自 SQL）
	End                time.Time `gorm:"-"`
	Requests           int64     `gorm:"column:requests"`
	Success            int64     `gorm:"column:success"`
	RateLimitFailures  int64     `gorm:"column:rate_limit_failures"`
	ServerFailures     int64     `gorm:"column:server_failures"`
	ConnectionFailures int64     `gorm:"column:connection_failures"`
}

// UptimeBucketWidth 可用性状态条的桶宽：区间秒数整除桶数后加 1 秒，保证 buckets 个桶能覆盖整个区间。
func UptimeBucketWidth(start, end time.Time, buckets int) time.Duration {
	if buckets <= 0 {
		return 0
	}
	seconds := max(0, int64(end.Sub(start)/time.Second))
	return time.Duration(seconds/int64(buckets)+1) * time.Second
}

// BuildUptimeBuckets 把稀疏查询结果补齐为恰好 buckets 个桶（无请求的桶为零值）。
//
// 桶边界以 start 为原点按桶宽等分、末桶在 end 处截断；GROUP BY 不会为没有调用的时间槽生成行。
func BuildUptimeBuckets(points []*DashboardUptimePoint, start, end time.Time, buckets int) []*DashboardUptimePoint {
	if buckets <= 0 {
		return []*DashboardUptimePoint{}
	}
	width := UptimeBucketWidth(start, end, buckets)
	byTime := make(map[int64]*DashboardUptimePoint, len(points))
	for _, p := range points {
		byTime[p.Time.Unix()] = p
	}
	result := make([]*DashboardUptimePoint, 0, buckets)
	for i := range buckets {
		bucketStart := start.Add(time.Duration(i) * width)
		if bucketStart.After(end) {
			bucketStart = end
		}
		bucketEnd := bucketStart.Add(width)
		if bucketEnd.After(end) {
			bucketEnd = end
		}
		bucket := &DashboardUptimePoint{Time: bucketStart, End: bucketEnd}
		if p, ok := byTime[bucketStart.Unix()]; ok {
			bucket.Requests = p.Requests
			bucket.Success = p.Success
			bucket.RateLimitFailures = p.RateLimitFailures
			bucket.ServerFailures = p.ServerFailures
			bucket.ConnectionFailures = p.ConnectionFailures
		}
		result = append(result, bucket)
	}
	return result
}

// DashboardModelPoint 模型 × 时间桶的用量（Tokens/Cost 仅成功调用）
type DashboardModelPoint struct {
	ModelID   string    `gorm:"column:model_id"`
	Time      time.Time `gorm:"column:time"`
	Requests  int64     `gorm:"column:requests"`
	Tokens    int64     `gorm:"column:tokens"`
	CostMicro int64     `gorm:"column:cost_micro"`
}

// DashboardErrorRow 失败调用按 (状态码, 截断后错误信息) 分组的计数
type DashboardErrorRow struct {
	StatusCode int    `gorm:"column:status_code"`
	Message    string `gorm:"column:message"`
	Count      int64  `gorm:"column:count"`
}

// DashboardEndpointRow 单个 Endpoint 的健康度
type DashboardEndpointRow struct {
	Endpoint              string   `gorm:"column:endpoint"`
	Requests              int64    `gorm:"column:requests"`
	Failures              int64    `gorm:"column:failures"`
	LatencyP95Ms          *float64 `gorm:"column:latency_p95_ms"`
	OutputTokensPerSecond *float64 `gorm:"column:output_tokens_per_second"`
	CostMicro             int64    `gorm:"column:cost_micro"`
}

// DashboardGroupRow 按维度分组的用量（Key 为分组键的文本形式）
type DashboardGroupRow struct {
	Key              string `gorm:"column:group_key"`
	Requests         int64  `gorm:"column:requests"`
	InputTokens      int64  `gorm:"column:input_tokens"`
	OutputTokens     int64  `gorm:"column:output_tokens"`
	CacheReadTokens  int64  `gorm:"column:cache_read_tokens"`
	CacheWriteTokens int64  `gorm:"column:cache_write_tokens"`
	CostMicro        int64  `gorm:"column:cost_micro"`
}

// DashboardHeatmapCell 星期 × 小时的调用量（Weekday 为 PG ISODOW：1=周一 … 7=周日）
type DashboardHeatmapCell struct {
	Weekday int   `gorm:"column:weekday"`
	Hour    int   `gorm:"column:hour"`
	Count   int64 `gorm:"column:count"`
}

// DashboardRepository dashboard 聚合查询仓储（只读，基于 model_call_audit）
//
// 范围语义统一：apiKeyIDs 为 nil 表示不过滤（admin/demo）；非 nil 空切片表示名下无 Key，
// 必须短路返回空结果，不得退化为全量查询。
//
//	@author centonhuang
//	@update 2026-10-28 10:00:00
type DashboardRepository interface {
	// QueryStats 时间窗总量统计（恒返回非 nil）
	QueryStats(ctx context.Context, apiKeyIDs []uint, start, end time.Time) (*DashboardStats, error)
	// QueryStatsSeries 按时间桶统计（仅返回有数据的桶，补桶由应用层完成）
	QueryStatsSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*DashboardStatsPoint, error)
	// QueryUptimeBuckets 可用性状态条的定长桶（恰好 buckets 个，无请求的桶补零）。
	// 补桶在仓储内完成：读缓存按分钟对齐窗口共享结果，桶边界必须随缓存内容一起固化。
	QueryUptimeBuckets(ctx context.Context, apiKeyIDs []uint, start, end time.Time, buckets int) ([]*DashboardUptimePoint, error)
	// QueryModelSeries 模型 × 时间桶用量
	QueryModelSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*DashboardModelPoint, error)
	// QueryTopErrors 失败调用热门错误
	QueryTopErrors(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*DashboardErrorRow, error)
	// QueryEndpointHealth 按 Endpoint 的健康度（按请求数降序）
	QueryEndpointHealth(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*DashboardEndpointRow, error)
	// QueryGroupUsage 按维度分组用量（支持 model / apikey / harness；user 维度由应用层按 apikey 聚合）
	QueryGroupUsage(ctx context.Context, apiKeyIDs []uint, start, end time.Time, dimension enum.DashboardDimension, rulesFingerprint string) ([]*DashboardGroupRow, error)
	// QueryHeatmap 星期 × 小时调用量；location 必须是调用方已校验的 IANA 时区名
	QueryHeatmap(ctx context.Context, apiKeyIDs []uint, start, end time.Time, location string) ([]*DashboardHeatmapCell, error)
}
