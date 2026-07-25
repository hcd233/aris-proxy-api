package constant

const (
	// ProxyErrorTemplate ProxyError 无 cause 时的错误消息模板
	ProxyErrorTemplate = "proxy error: status %d"
	// ProxyErrorWithCauseTemplate ProxyError 携带 cause 时的错误消息模板
	ProxyErrorWithCauseTemplate = "proxy error: status %d: %s"
)
