package constant

import "time"

const (
	LogMiddlewareSamplingInterval = 5 * time.Minute

	LogInfoMaxSizeMB  = 100
	LogInfoMaxBackups = 3
	LogInfoMaxAgeDays = 7
	LogErrMaxSizeMB   = 500
	LogErrMaxBackups  = 3
	LogErrMaxAgeDays  = 30

	LogInfoFileName  = "aris-proxy-api.log"
	LogErrFileName   = "aris-proxy-api-error.log"
	LogPanicFileName = "aris-proxy-api-panic.log"

	CLSLevelDebug     = "DEBUG"
	CLSLevelInfo      = "INFO"
	CLSLevelWarn      = "WARN"
	CLSLevelError     = "ERROR"
	CLSFieldMessage   = "message"
	CLSFieldLevel     = "level"
	CLSFieldTimestamp = "timestamp"
	CLSFieldCaller    = "caller"
	CLSFieldStack     = "stack"

	CLSProducerCloseTimeoutMs = 10000
)
