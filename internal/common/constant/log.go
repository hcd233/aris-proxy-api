package constant

import "time"

const (
	LogMiddlewareSamplingInterval = 5 * time.Minute

	// LogMiddlewareBodyMaxBytes 日志记录请求/响应体的字节上限。
	// 超过上限的 body 只记录截断后的字符串，不再整体 JSON 解析：
	// 310KB 的 session/message/list 响应完整解析 + 再编码约需 8~20ms/req，
	// 是压在边缘大响应接口上的主要 CPU 成本（见 docs/loadtest/QPS-OPTIMIZATION.md）。
	LogMiddlewareBodyMaxBytes = 2048

	LogInfoMaxSizeMB  = 100
	LogInfoMaxBackups = 3
	LogInfoMaxAgeDays = 7
	LogErrMaxSizeMB   = 500
	LogErrMaxBackups  = 3
	LogErrMaxAgeDays  = 30

	LogInfoFileName  = "aris-proxy-api.log"
	LogErrFileName   = "aris-proxy-api-error.log"
	LogPanicFileName = "aris-proxy-api-panic.log"

	CLSFieldMessage   = "message"
	CLSFieldLevel     = "level"
	CLSFieldTimestamp = "timestamp"
	CLSFieldCaller    = "caller"
	CLSFieldStack     = "stack"

	CLSProducerCloseTimeoutMs = 10000
)
