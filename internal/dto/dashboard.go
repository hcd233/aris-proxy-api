package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// DashboardRangeReq dashboard 时间窗 + 时间桶请求
type DashboardRangeReq struct {
	StartTime   time.Time        `query:"startTime" required:"true" doc:"区间起点（RFC3339）"`
	EndTime     time.Time        `query:"endTime" required:"true" doc:"区间终点（RFC3339）"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week" doc:"时间桶粒度"`
}

// DashboardWindowReq dashboard 时间窗 + 条数请求
type DashboardWindowReq struct {
	StartTime time.Time `query:"startTime" required:"true" doc:"区间起点（RFC3339）"`
	EndTime   time.Time `query:"endTime" required:"true" doc:"区间终点（RFC3339）"`
	Limit     int       `query:"limit" minimum:"1" maximum:"10" default:"5" doc:"返回条数"`
}

// DashboardTopReq dashboard Top 排行请求
type DashboardTopReq struct {
	StartTime time.Time               `query:"startTime" required:"true" doc:"区间起点（RFC3339）"`
	EndTime   time.Time               `query:"endTime" required:"true" doc:"区间终点（RFC3339）"`
	Limit     int                     `query:"limit" minimum:"1" maximum:"10" default:"5" doc:"返回条数"`
	Dimension enum.DashboardDimension `query:"dimension" required:"true" enum:"model,user,apikey,harness" doc:"分组维度（user 仅 admin/demo 可用）"`
}

// DashboardHeatmapReq dashboard 活跃时段热力图请求
type DashboardHeatmapReq struct {
	StartTime time.Time `query:"startTime" required:"true" doc:"区间起点（RFC3339）"`
	EndTime   time.Time `query:"endTime" required:"true" doc:"区间终点（RFC3339）"`
	TZ        string    `query:"tz" required:"true" minLength:"1" maxLength:"64" doc:"IANA 时区名，如 Asia/Shanghai"`
}

// DashboardFailures 失败请求按原因拆分
type DashboardFailures struct {
	RateLimit  int64 `json:"rateLimit" doc:"429 限流"`
	Server     int64 `json:"server" doc:"5xx"`
	Connection int64 `json:"connection" doc:"连接失败（状态码 -1）"`
	Other      int64 `json:"other" doc:"其他非 200"`
}

// DashboardTokens 成功请求的四维 token（input 为净输入，不含缓存）
type DashboardTokens struct {
	Input      int64 `json:"input" doc:"净输入 token"`
	Output     int64 `json:"output" doc:"输出 token"`
	CacheRead  int64 `json:"cacheRead" doc:"缓存命中 token"`
	CacheWrite int64 `json:"cacheWrite" doc:"缓存写入 token"`
}

// DashboardLatency 首 token 延迟分位数（毫秒；无成功样本时省略）
type DashboardLatency struct {
	P50 *float64 `json:"p50,omitempty" doc:"P50（ms）"`
	P95 *float64 `json:"p95,omitempty" doc:"P95（ms）"`
	P99 *float64 `json:"p99,omitempty" doc:"P99（ms）"`
}

// DashboardStatsItem 统计块
type DashboardStatsItem struct {
	Requests              int64              `json:"requests" doc:"全部请求数"`
	Success               int64              `json:"success" doc:"成功请求数（上游 200）"`
	Failures              *DashboardFailures `json:"failures" doc:"失败拆分"`
	Tokens                *DashboardTokens   `json:"tokens" doc:"token 拆分（仅成功请求）"`
	Cost                  float64            `json:"cost" doc:"已计价费用（展示单位）"`
	PricedSuccess         int64              `json:"pricedSuccess" doc:"已计价的成功请求数"`
	Latency               *DashboardLatency  `json:"latency" doc:"首 token 延迟分位数"`
	OutputTokensPerSecond *float64           `json:"outputTokensPerSecond,omitempty" doc:"输出速度 tok/s（仅流式成功请求）"`
	AvgStreamDurationMs   *float64           `json:"avgStreamDurationMs,omitempty" doc:"平均流式时长（ms）"`
}

// DashboardStatsPointItem 时间桶统计
type DashboardStatsPointItem struct {
	Time time.Time `json:"time" doc:"时间桶（UTC）"`
	DashboardStatsItem
}

// DashboardPeriodItem 一个统计周期
type DashboardPeriodItem struct {
	Summary *DashboardStatsItem        `json:"summary" doc:"周期总量"`
	Series  []*DashboardStatsPointItem `json:"series" doc:"已补齐空桶的时间序列"`
}

// DashboardOverview 本期与上一等长周期
type DashboardOverview struct {
	Currency enum.Currency        `json:"currency" doc:"费用币种"`
	Current  *DashboardPeriodItem `json:"current" doc:"本期"`
	Previous *DashboardPeriodItem `json:"previous" doc:"上一等长周期"`
}

// DashboardOverviewRsp dashboard 总览响应
type DashboardOverviewRsp struct {
	CommonRsp
	Data *DashboardOverview `json:"data,omitempty" doc:"总览数据"`
}

// DashboardModelPointItem 模型 × 时间桶用量
type DashboardModelPointItem struct {
	ModelID  string    `json:"modelId" doc:"业务模型ID"`
	Time     time.Time `json:"time" doc:"时间桶（UTC）"`
	Requests int64     `json:"requests" doc:"请求数"`
	Tokens   int64     `json:"tokens" doc:"token 总量（仅成功请求）"`
	Cost     float64   `json:"cost" doc:"已计价费用"`
}

// DashboardModelSeriesRsp 模型序列响应
type DashboardModelSeriesRsp struct {
	CommonRsp
	Data []*DashboardModelPointItem `json:"data" doc:"模型 × 时间桶用量"`
}

// DashboardErrorItem 热门错误
type DashboardErrorItem struct {
	StatusCode int    `json:"statusCode" doc:"上游状态码"`
	Message    string `json:"message" doc:"错误信息（截断到 200 字符）"`
	Count      int64  `json:"count" doc:"出现次数"`
}

// DashboardErrorListRsp 热门错误响应
type DashboardErrorListRsp struct {
	CommonRsp
	Data []*DashboardErrorItem `json:"data" doc:"热门错误"`
}

// DashboardEndpointItem Endpoint 健康度
type DashboardEndpointItem struct {
	Endpoint              string   `json:"endpoint" doc:"Endpoint 名（demo 脱敏）"`
	Requests              int64    `json:"requests" doc:"请求数"`
	ErrorRate             float64  `json:"errorRate" doc:"错误率 0-1"`
	P95LatencyMs          *float64 `json:"p95LatencyMs,omitempty" doc:"首 token P95（ms）"`
	OutputTokensPerSecond *float64 `json:"outputTokensPerSecond,omitempty" doc:"输出速度 tok/s"`
	Cost                  float64  `json:"cost" doc:"已计价费用"`
}

// DashboardEndpointListRsp Endpoint 健康度响应
type DashboardEndpointListRsp struct {
	CommonRsp
	Data []*DashboardEndpointItem `json:"data" doc:"按请求数降序"`
}

// DashboardTopItem Top 排行项
type DashboardTopItem struct {
	Key          string   `json:"key" doc:"分组键"`
	Name         string   `json:"name" doc:"展示名（demo 脱敏）"`
	Requests     int64    `json:"requests" doc:"请求数"`
	Tokens       int64    `json:"tokens" doc:"token 总量（仅成功请求）"`
	CacheHitRate *float64 `json:"cacheHitRate,omitempty" doc:"缓存命中率 0-1"`
	Cost         float64  `json:"cost" doc:"已计价费用"`
	CostShare    float64  `json:"costShare" doc:"费用占比 0-1"`
}

// DashboardTop Top 排行
type DashboardTop struct {
	Items         []*DashboardTopItem `json:"items" doc:"前 N 名"`
	OtherRequests int64               `json:"otherRequests" doc:"其余请求合计"`
	OtherCost     float64             `json:"otherCost" doc:"其余费用合计"`
}

// DashboardTopListRsp Top 排行响应
type DashboardTopListRsp struct {
	CommonRsp
	Data *DashboardTop `json:"data,omitempty" doc:"排行数据"`
}

// DashboardHeatmapCell 热力图格子
type DashboardHeatmapCell struct {
	Weekday int   `json:"weekday" doc:"星期（0=周一 … 6=周日）"`
	Hour    int   `json:"hour" doc:"小时 0-23"`
	Count   int64 `json:"count" doc:"请求数"`
}

// DashboardHeatmapRsp 热力图响应
type DashboardHeatmapRsp struct {
	CommonRsp
	Data []*DashboardHeatmapCell `json:"data" doc:"有数据的格子"`
}
