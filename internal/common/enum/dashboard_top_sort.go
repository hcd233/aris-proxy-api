package enum

// DashboardTopSort dashboard Top 排行的排序指标
type DashboardTopSort = string

const (
	// DashboardTopSortCost 按已计价费用降序（默认）
	DashboardTopSortCost DashboardTopSort = "cost"
	// DashboardTopSortRequests 按请求数降序
	DashboardTopSortRequests DashboardTopSort = "requests"
)
