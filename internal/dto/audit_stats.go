package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

type ModelTrendReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type ModelTrendRsp struct {
	CommonRsp
	Data []*ModelTrendItem `json:"data,omitempty" doc:"各模型的调用趋势"`
}

type ModelTrendItem struct {
	ModelID string        `json:"modelId" doc:"业务模型ID"`
	Points  []*TrendPoint `json:"points" doc:"时间序列点"`
}

type TrendPoint struct {
	Time  time.Time `json:"time" doc:"时间桶"`
	Count int       `json:"count" doc:"调用次数"`
}

type RequestRateReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type RequestRateRsp struct {
	CommonRsp
	Data []*RequestRateItem `json:"data,omitempty" doc:"各模型的请求成功率"`
}

type RequestRateItem struct {
	ModelID string       `json:"modelId" doc:"业务模型ID"`
	Points  []*RatePoint `json:"points" doc:"时间序列点"`
}

type RatePoint struct {
	Time        time.Time `json:"time" doc:"时间桶"`
	Total       int       `json:"total" doc:"总请求数"`
	Success     int       `json:"success" doc:"成功数"`
	Failed      int       `json:"failed" doc:"失败数"`
	SuccessRate float64   `json:"successRate" doc:"成功率 0-1"`
}

type TokenThroughputReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type TokenThroughputRsp struct {
	CommonRsp
	Data []*TokenThroughputPoint `json:"data,omitempty" doc:"Token 吞吐量时间序列"`
}

type TokenThroughputPoint struct {
	Time                time.Time `json:"time" doc:"时间桶"`
	InputTokens         int       `json:"inputTokens" doc:"输入 Token 数"`
	OutputTokens        int       `json:"outputTokens" doc:"输出 Token 数"`
	CacheCreationTokens int       `json:"cacheCreationTokens" doc:"缓存创建 Token 数"`
	CacheReadTokens     int       `json:"cacheReadTokens" doc:"缓存读取 Token 数"`
}

// — Token Rate —

type TokenRateReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type TokenRateRsp struct {
	CommonRsp
	Data []*TokenRateItem `json:"data,omitempty" doc:"各模型的输出 Token 速率"`
}

type TokenRateItem struct {
	ModelID string            `json:"modelId" doc:"业务模型ID"`
	Points  []*TokenRatePoint `json:"points" doc:"时间序列点"`
}

type TokenRatePoint struct {
	Time                  time.Time `json:"time" doc:"时间桶"`
	OutputTokensPerSecond float64   `json:"outputTokensPerSecond" doc:"输出 Token 速率 (tokens/s)"`
}

// — First Token Latency —

type FirstTokenLatencyReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type FirstTokenLatencyRsp struct {
	CommonRsp
	Data []*FirstTokenLatencyItem `json:"data,omitempty" doc:"各模型的首 Token 延迟"`
}

type FirstTokenLatencyItem struct {
	ModelID string                    `json:"modelId" doc:"业务模型ID"`
	Points  []*FirstTokenLatencyPoint `json:"points" doc:"时间序列点"`
}

type FirstTokenLatencyPoint struct {
	Time             time.Time `json:"time" doc:"时间桶"`
	AverageLatencyMs float64   `json:"averageLatencyMs" doc:"平均首 Token 延迟 (ms)"`
}

// — Model Usage —

type ModelUsageReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type ModelUsageRsp struct {
	CommonRsp
	Data []*ModelUsageItem `json:"data,omitempty" doc:"各模型的 Token 聚合用量"`
}

type ModelUsageItem struct {
	ModelID             string `json:"modelId" doc:"业务模型ID"`
	InputTokens         int    `json:"inputTokens" doc:"输入 Token 总数"`
	OutputTokens        int    `json:"outputTokens" doc:"输出 Token 总数"`
	CacheReadTokens     int    `json:"cacheReadTokens" doc:"缓存读取 Token 总数"`
	CacheCreationTokens int    `json:"cacheCreationTokens" doc:"缓存创建 Token 总数"`
}

// AuditCostSummaryReq 估算费用合计与趋势查询
type AuditCostSummaryReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

// AuditCostSummaryRsp 估算费用合计与趋势响应（按币种分组）
type AuditCostSummaryRsp struct {
	CommonRsp
	Totals []*AuditCostTotalItem   `json:"totals,omitempty" doc:"费用合计（按币种）"`
	Series []*AuditCostSeriesPoint `json:"series,omitempty" doc:"费用趋势（时间桶 × 币种）"`
}

// AuditCostTotalItem 费用合计行（展示单位）
type AuditCostTotalItem struct {
	Currency enum.Currency `json:"currency" doc:"币种"`
	Cost     float64       `json:"cost" doc:"估算费用（展示单位）"`
}

// AuditCostSeriesPoint 费用趋势点（展示单位）
type AuditCostSeriesPoint struct {
	BucketTime time.Time     `json:"bucketTime" doc:"时间桶（RFC3339 UTC）"`
	Currency   enum.Currency `json:"currency" doc:"币种"`
	Cost       float64       `json:"cost" doc:"估算费用（展示单位）"`
}

// AuditCostDistributionReq 估算费用分布查询
type AuditCostDistributionReq struct {
	GroupBy   string    `query:"groupBy" required:"true" enum:"user,api_key,model" doc:"分组维度（user 仅管理员）"`
	StartTime time.Time `query:"startTime" required:"true"`
	EndTime   time.Time `query:"endTime" required:"true"`
	Limit     int       `query:"limit" minimum:"1" maximum:"100" default:"10" doc:"每币种返回行数"`
}

// AuditCostDistributionRsp 估算费用分布响应（每行 = 分组 × 币种）
type AuditCostDistributionRsp struct {
	CommonRsp
	GroupBy string                       `json:"groupBy" doc:"分组维度"`
	Items   []*AuditCostDistributionItem `json:"items,omitempty" doc:"分布行（币种内费用降序）"`
}

// AuditCostDistributionItem 费用分布行（展示单位）
type AuditCostDistributionItem struct {
	ID       string        `json:"id" doc:"分组键（用户ID/API Key ID/模型ID）"`
	Name     string        `json:"name" doc:"展示名"`
	Currency enum.Currency `json:"currency" doc:"币种"`
	Cost     float64       `json:"cost" doc:"估算费用（展示单位）"`
}
