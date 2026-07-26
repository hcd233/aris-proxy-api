package constant

const (
	// ProxyErrorTemplate ProxyError 无 cause 时的错误消息模板
	ProxyErrorTemplate = "proxy error: status %d"
	// ProxyErrorWithCauseTemplate ProxyError 携带 cause 时的错误消息模板
	ProxyErrorWithCauseTemplate = "proxy error: status %d: %s"
	// UnknownProxyResultTypeTemplate adapter 收到未知 Result 类型时的错误消息模板
	UnknownProxyResultTypeTemplate = "unknown proxy result type %T"
)
