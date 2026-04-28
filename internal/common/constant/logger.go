package constant

const (
	// LogLevelDebug 日志级别 DEBUG
	LogLevelDebug = "DEBUG"
	// LogLevelInfo 日志级别 INFO
	LogLevelInfo = "INFO"
	// LogLevelWarn 日志级别 WARN
	LogLevelWarn = "WARN"
	// LogLevelError 日志级别 ERROR
	LogLevelError = "ERROR"
	// LogLevelDPanic 日志级别 DPANIC
	LogLevelDPanic = "DPANIC"
	// LogLevelPanic 日志级别 PANIC
	LogLevelPanic = "PANIC"
	// LogLevelFatal 日志级别 FATAL
	LogLevelFatal = "FATAL"

	// LogTimeKey zap encoder time key
	LogTimeKey = "timestamp"
	// LogLevelKey zap encoder level key
	LogLevelKey = "level"
	// LogNameKey zap encoder name key
	LogNameKey = "logger"
	// LogCallerKey zap encoder caller key
	LogCallerKey = "caller"
	// LogMessageKey zap encoder message key
	LogMessageKey = "message"
	// LogStacktraceKey zap encoder stacktrace key
	LogStacktraceKey = "stacktrace"
)
