package dto

// MetricsJSONRsp Prometheus 指标 JSON 响应
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsJSONRsp struct {
	CommonRsp
	Metrics []MetricFamilyItem `json:"metrics,omitempty" doc:"Metric families"`
}

// MetricFamilyItem 指标族
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricFamilyItem struct {
	Name    string             `json:"name" doc:"Metric name"`
	Type    string             `json:"type" doc:"Metric type"`
	Help    string             `json:"help" doc:"Metric help text"`
	Samples []MetricSampleItem `json:"samples,omitempty" doc:"Metric samples"`
}

// MetricSampleItem 指标样本
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricSampleItem struct {
	Labels map[string]string `json:"labels,omitempty" doc:"Sample labels"`
	Value  float64           `json:"value" doc:"Sample value"`
}
