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
