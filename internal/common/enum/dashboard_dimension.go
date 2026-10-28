package enum

// DashboardDimension dashboard Top 排行的分组维度
type DashboardDimension = string

const (
	// DashboardDimensionModel 按业务模型
	DashboardDimensionModel DashboardDimension = "model"
	// DashboardDimensionUser 按用户（由 API Key 归属聚合，仅 admin/demo 可用）
	DashboardDimensionUser DashboardDimension = "user"
	// DashboardDimensionAPIKey 按 API Key
	DashboardDimensionAPIKey DashboardDimension = "apikey"
	// DashboardDimensionHarness 按 agent harness（客户端）
	DashboardDimensionHarness DashboardDimension = "harness"
)
