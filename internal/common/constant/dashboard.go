package constant

// Dashboard 聚合查询常量（spec: docs/superpowers/specs/2026-10-28-dashboard-redesign-design.md §4）。
//
// 口径：成功 = upstream_status_code = 200；Token/费用/延迟/速度只统计成功调用；
// 速度只统计 stream_duration_ms > 0 的流式成功调用（非流式时长为 0，混入会高估速度）。
const (
	// DashboardMaxBuckets 单次查询允许的最大时间桶数（防止 minute 粒度查长区间拖垮分位数聚合）
	DashboardMaxBuckets = 1500
	// DashboardErrorMessageMaxLen 热门错误分组前对 error_message 的截断长度
	DashboardErrorMessageMaxLen = 200

	// dashboardSuccessFilter 成功调用过滤子句（拼接在聚合函数后）
	dashboardSuccessFilter = " FILTER (WHERE upstream_status_code = 200)"
	// dashboardPricedFilter 已计价成功调用过滤子句
	dashboardPricedFilter = " FILTER (WHERE upstream_status_code = 200 AND cost_micro IS NOT NULL)"
	// dashboardStreamFilter 流式成功调用过滤子句
	dashboardStreamFilter = " FILTER (WHERE upstream_status_code = 200 AND stream_duration_ms > 0)"

	// DashboardStatsSelect 统计块投影（QueryStats 无分组；QueryStatsSeries 前缀时间桶后分组）
	DashboardStatsSelect = "COUNT(*) AS requests, " +
		"COUNT(*)" + dashboardSuccessFilter + " AS success, " +
		"COUNT(*) FILTER (WHERE upstream_status_code = 429) AS rate_limit_failures, " +
		"COUNT(*) FILTER (WHERE upstream_status_code BETWEEN 500 AND 599) AS server_failures, " +
		"COUNT(*) FILTER (WHERE upstream_status_code = -1) AS connection_failures, " +
		"COALESCE(SUM(input_tokens)" + dashboardSuccessFilter + ", 0) AS input_tokens, " +
		"COALESCE(SUM(output_tokens)" + dashboardSuccessFilter + ", 0) AS output_tokens, " +
		"COALESCE(SUM(cache_read_input_tokens)" + dashboardSuccessFilter + ", 0) AS cache_read_tokens, " +
		"COALESCE(SUM(cache_creation_input_tokens)" + dashboardSuccessFilter + ", 0) AS cache_write_tokens, " +
		"COALESCE(SUM(cost_micro)" + dashboardPricedFilter + ", 0) AS cost_micro, " +
		"COUNT(*)" + dashboardPricedFilter + " AS priced_success, " +
		"percentile_cont(0.5) WITHIN GROUP (ORDER BY first_token_latency_ms)" + dashboardSuccessFilter + " AS latency_p50_ms, " +
		"percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_latency_ms)" + dashboardSuccessFilter + " AS latency_p95_ms, " +
		"percentile_cont(0.99) WITHIN GROUP (ORDER BY first_token_latency_ms)" + dashboardSuccessFilter + " AS latency_p99_ms, " +
		DashboardOutputSpeedExpr + " AS output_tokens_per_second, " +
		"AVG(stream_duration_ms)" + dashboardStreamFilter + " AS avg_stream_duration_ms"

	// DashboardOutputSpeedExpr 输出速度（tok/s）表达式；无流式样本时为 NULL
	DashboardOutputSpeedExpr = "SUM(output_tokens)" + dashboardStreamFilter + " * 1000.0 / " +
		"NULLIF(SUM(stream_duration_ms)" + dashboardStreamFilter + ", 0)"

	// DashboardTimeBucketAlias 时间桶列别名
	DashboardTimeBucketAlias = " AS time, "
	// DashboardGroupByTime 按时间桶分组/排序
	DashboardGroupByTime = "time"

	ReadCachePurposeDashboardStats    = "dashboard:stats"
	ReadCachePurposeDashboardSeries   = "dashboard:series"
	ReadCachePurposeDashboardModel    = "dashboard:model-series"
	ReadCachePurposeDashboardErrors   = "dashboard:errors"
	ReadCachePurposeDashboardEndpoint = "dashboard:endpoint"
	ReadCachePurposeDashboardGroup    = "dashboard:group"
	ReadCachePurposeDashboardHeatmap  = "dashboard:heatmap"
)
