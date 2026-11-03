package enum

// DashboardMetric dashboard 排序与占比的指标口径（Top 排行、模型用量图共用）
type DashboardMetric = string

const (
	// DashboardMetricCost 按费用（微单位累加）
	DashboardMetricCost DashboardMetric = "cost"
	// DashboardMetricRequests 按请求数
	DashboardMetricRequests DashboardMetric = "requests"
	// DashboardMetricTokens 按 token 总量
	DashboardMetricTokens DashboardMetric = "tokens"
)
