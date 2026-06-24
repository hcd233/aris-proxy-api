package constant

const (
	MetricServiceName              = "aris-proxy-api"
	MetricNamespace                = "http"
	MetricSSEActiveConnectionsName = "sse_active_connections"
	MetricSSEActiveConnectionsHelp = "Number of active SSE streaming connections"
	MetricLabelProvider            = "provider"
)

var PrometheusRequestDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75,
	1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600, 1800,
}
