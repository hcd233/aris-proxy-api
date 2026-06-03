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
	Model  string        `json:"model" doc:"模型名"`
	Points []*TrendPoint `json:"points" doc:"时间序列点"`
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
	Model  string       `json:"model" doc:"模型名"`
	Points []*RatePoint `json:"points" doc:"时间序列点"`
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
	Model  string            `json:"model" doc:"模型名"`
	Points []*TokenRatePoint `json:"points" doc:"时间序列点"`
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
	Model  string                    `json:"model" doc:"模型名"`
	Points []*FirstTokenLatencyPoint `json:"points" doc:"时间序列点"`
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
	Model               string `json:"model" doc:"模型名"`
	InputTokens         int    `json:"inputTokens" doc:"输入 Token 总数"`
	OutputTokens        int    `json:"outputTokens" doc:"输出 Token 总数"`
	CacheReadTokens     int    `json:"cacheReadTokens" doc:"缓存读取 Token 总数"`
	CacheCreationTokens int    `json:"cacheCreationTokens" doc:"缓存创建 Token 总数"`
}
