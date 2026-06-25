package constant

import "time"

const (
	MetricServiceName = "aris-proxy-api"

	// MetricNamespaceHTTP HTTP 指标命名空间（最终指标名形如 http_request_duration_seconds）
	MetricNamespaceHTTP = "http"

	// MetricNameRequestDuration 请求时延直方图（不含 namespace）
	MetricNameRequestDuration = "request_duration_seconds"
	// MetricNameRequestsInProgress 在途请求数 gauge（不含 namespace）
	MetricNameRequestsInProgress = "requests_in_progress"

	MetricSSEActiveConnectionsName = "sse_active_connections"
	MetricSSEActiveConnectionsHelp = "Number of active SSE streaming connections"
	MetricRequestsInProgressHelp   = "Number of HTTP requests currently being served"
	MetricRequestDurationHelp      = "HTTP request latency in seconds"
	MetricLabelProvider            = "provider"

	// —— flusher 从 registry.Gather() 抽取快照时用的完整指标名 ——
	MetricFullRequestDuration = "http_request_duration_seconds"
	MetricFullInProgress      = "http_requests_in_progress"
	MetricFullGoGoroutines    = "go_goroutines"
	MetricFullGoHeapAlloc     = "go_memstats_alloc_bytes"
	MetricFullProcessCPU      = "process_cpu_seconds_total"
	MetricFullSSEActive       = "sse_active_connections"
)

var PrometheusRequestDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75,
	1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600, 1800,
}

const (
	// RuntimeMetricsFlushInterval 每个 pod 采集并写入快照的间隔（= 时序分辨率）
	RuntimeMetricsFlushInterval = 5 * time.Second
	// RuntimeMetricsRetention Redis 中运行时快照的留存窗口
	RuntimeMetricsRetention = 24 * time.Hour
	// RuntimeMetricsInstanceTTL 实例注册表中超过此时长未心跳的死实例会被清理
	RuntimeMetricsInstanceTTL = 24 * time.Hour

	// RuntimeMetricsUnknownInstance hostname 获取失败时的兜底实例标识
	RuntimeMetricsUnknownInstance = "unknown"
)

// —— 运行时指标 range 档 key ——
const (
	RuntimeMetricsRange15m = "15m"
	RuntimeMetricsRange1h  = "1h"
	RuntimeMetricsRange6h  = "6h"
	RuntimeMetricsRange24h = "24h"
)

// —— 各 range 档对应的窗口长度与桶宽 ——
const (
	RuntimeMetricsWindow15m = 15 * time.Minute
	RuntimeMetricsBucket15m = 15 * time.Second
	RuntimeMetricsWindow1h  = 1 * time.Hour
	RuntimeMetricsBucket1h  = 1 * time.Minute
	RuntimeMetricsWindow6h  = 6 * time.Hour
	RuntimeMetricsBucket6h  = 5 * time.Minute
	RuntimeMetricsWindow24h = 24 * time.Hour
	RuntimeMetricsBucket24h = 15 * time.Minute
)

// —— 聚合计算常量 ——
const (
	// RuntimeMetricsBytesPerMB 字节转 MB 的除数
	RuntimeMetricsBytesPerMB = 1024 * 1024
	// RuntimeMetricsP95Percentile P95 分位
	RuntimeMetricsP95Percentile = 0.95
	// RuntimeMetricsMsPerSecond 秒转毫秒
	RuntimeMetricsMsPerSecond = 1000
	// RuntimeMetricsPercentToRatio 比率转百分比
	RuntimeMetricsPercentToRatio = 100
	// RuntimeMetricsRoundScale 保留两位小数的缩放因子
	RuntimeMetricsRoundScale = 100
	// RuntimeMetricsRoundHalf 四舍五入的半值
	RuntimeMetricsRoundHalf = 0.5
)
