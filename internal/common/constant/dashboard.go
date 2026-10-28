package constant

// Dashboard 聚合查询常量（spec: docs/superpowers/specs/2026-10-28-dashboard-redesign-design.md §4）。
//
// 口径：成功 = upstream_status_code = 200；Token/费用/延迟/速度只统计成功调用；
// 速度只统计 stream_duration_ms > 0 的流式成功调用（非流式时长为 0，混入会高估速度）。
const (
	// DashboardMaxBuckets 单次查询允许的最大时间桶数（防止 minute 粒度查长区间拖垮分位数聚合）
	DashboardMaxBuckets = 1500
	// TimezoneLocal Go 的进程本地时区名（LoadLocation 接受但 PostgreSQL 不认识）
	TimezoneLocal = "Local"
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

	// DashboardModelSeriesSelect 模型 × 时间桶用量投影（前缀时间桶表达式）
	DashboardModelSeriesSelect = " AS time, model_id, COUNT(*) AS requests, " +
		"COALESCE(SUM(input_tokens + output_tokens + cache_read_input_tokens + cache_creation_input_tokens)" +
		dashboardSuccessFilter + ", 0) AS tokens, " +
		"COALESCE(SUM(cost_micro)" + dashboardPricedFilter + ", 0) AS cost_micro"
	// DashboardModelSeriesGroup 模型序列分组/排序
	DashboardModelSeriesGroup = "model_id, time"

	// DashboardTopErrorsSelect 失败调用热门错误投影
	DashboardTopErrorsSelect = "upstream_status_code AS status_code, LEFT(error_message, 200) AS message, COUNT(*) AS count"
	// DashboardConditionFailure 失败调用条件
	DashboardConditionFailure = "upstream_status_code <> 200"
	// DashboardTopErrorsGroup 热门错误分组
	DashboardTopErrorsGroup = "status_code, message"
	// DashboardOrderCountDesc 按计数降序
	DashboardOrderCountDesc = "count DESC"

	// DashboardEndpointSelect Endpoint 健康度投影
	DashboardEndpointSelect = "endpoint, COUNT(*) AS requests, " +
		"COUNT(*) FILTER (WHERE upstream_status_code <> 200) AS failures, " +
		"percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_latency_ms)" + dashboardSuccessFilter + " AS latency_p95_ms, " +
		DashboardOutputSpeedExpr + " AS output_tokens_per_second, " +
		"COALESCE(SUM(cost_micro)" + dashboardPricedFilter + ", 0) AS cost_micro"
	// DashboardEndpointGroup Endpoint 分组
	DashboardEndpointGroup = "endpoint"
	// DashboardOrderRequestsDesc 按请求数降序
	DashboardOrderRequestsDesc = "requests DESC"

	// DashboardGroupKeyModel 模型维度分组键
	DashboardGroupKeyModel = "model_id"
	// DashboardGroupKeyAPIKey API Key 维度分组键（转文本，与其他维度统一为字符串键）
	DashboardGroupKeyAPIKey = "CAST(api_key_id AS TEXT)"
	// DashboardGroupUsageSelect 分组用量投影的后半段（前缀 "<分组键> AS group_key, "）
	DashboardGroupUsageSelect = " AS group_key, " + dashboardUsageAggregates

	// dashboardUsageAggregates 用量聚合列（请求数统计全部调用；token/费用仅成功、已计价）
	dashboardUsageAggregates = "COUNT(*) AS requests, " +
		"COALESCE(SUM(input_tokens)" + dashboardSuccessFilter + ", 0) AS input_tokens, " +
		"COALESCE(SUM(output_tokens)" + dashboardSuccessFilter + ", 0) AS output_tokens, " +
		"COALESCE(SUM(cache_read_input_tokens)" + dashboardSuccessFilter + ", 0) AS cache_read_tokens, " +
		"COALESCE(SUM(cache_creation_input_tokens)" + dashboardSuccessFilter + ", 0) AS cache_write_tokens, " +
		"COALESCE(SUM(cost_micro)" + dashboardPricedFilter + ", 0) AS cost_micro"
	// DashboardUAUsageSelect harness 维度内层：先按 user_agent 预聚合（UA 去重后通常只有几十个），
	// 使「UA → 最长前缀规则」关联子查询只对去重 UA 执行，而不是对每一行审计执行
	// （生产实测 30d 区间 312ms，逐行子查询占绝大部分）。
	DashboardUAUsageSelect = "user_agent, " + dashboardUsageAggregates
	// DashboardUAUsageGroup harness 维度内层分组
	DashboardUAUsageGroup = "user_agent"
	// DashboardUAUsageFrom harness 维度外层数据源（? 绑定内层子查询）
	DashboardUAUsageFrom = "(?) AS ua_usage"
	// DashboardUAUsageOuterSelect harness 维度外层投影的后半段（前缀规则分组表达式）
	DashboardUAUsageOuterSelect = " AS group_key, SUM(requests) AS requests, " +
		"SUM(input_tokens) AS input_tokens, SUM(output_tokens) AS output_tokens, " +
		"SUM(cache_read_tokens) AS cache_read_tokens, SUM(cache_write_tokens) AS cache_write_tokens, " +
		"SUM(cost_micro) AS cost_micro"

	// DashboardGroupUsageGroup 分组用量分组
	DashboardGroupUsageGroup = "group_key"
	// DashboardGroupUsageOrder 分组用量排序：费用优先、请求数次之
	DashboardGroupUsageOrder = "cost_micro DESC, requests DESC"

	// DashboardHeatmapSelect 星期 × 小时投影；两个 ? 均绑定已校验的 IANA 时区名
	DashboardHeatmapSelect = "CAST(EXTRACT(ISODOW FROM created_at AT TIME ZONE ?) AS INTEGER) AS weekday, " +
		"CAST(EXTRACT(HOUR FROM created_at AT TIME ZONE ?) AS INTEGER) AS hour, COUNT(*) AS count"
	// DashboardHeatmapGroup 热力图分组
	DashboardHeatmapGroup = "weekday, hour"

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
