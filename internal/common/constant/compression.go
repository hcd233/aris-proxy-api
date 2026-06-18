package constant

const (
	// ── 压缩策略名 ──
	CompressionStrategySmartCrusher     = "smart_crusher"
	CompressionStrategyLogCompressor    = "log_compressor"
	CompressionStrategySearchCompressor = "search_compressor"
	CompressionStrategyPassthrough      = "passthrough"
	CompressionStrategySkippedTooSmall  = "skipped:too_small"

	// ── SmartCrusher 参数 ──
	CompressionSmartCrusherLosslessRatio = 0.7
	CompressionSmartCrusherMaxItems      = 20
	CompressionSmartCrusherErrorKeywords = "error,exception,fail,fatal,panic,critical,timeout"

	// ── LogCompressor 参数 ──
	CompressionLogKeepLevels   = "ERROR,WARN,FATAL,PANIC,CRITICAL"
	CompressionLogMaxInfoLines = 3

	// ── SearchCompressor 参数 ──
	CompressionSearchMaxPerFile = 5

	// ── JSON 字段名 ──
	CompressionJSONKeyMessages       = "messages"
	CompressionJSONKeyContent        = "content"
	CompressionJSONKeyRole           = "role"
	CompressionJSONKeyTool           = "tool"
	CompressionJSONKeyType           = "type"
	CompressionJSONKeyToolResult     = "tool_result"
	CompressionJSONKeyText           = "text"
	CompressionJSONKeyInput          = "input"
	CompressionJSONKeyOutput         = "output"
	CompressionJSONKeyFuncCallOutput = "function_call_output"

	// ── 日志模板占位符 ──
	CompressionLogTemplateTS   = "<TS>"
	CompressionLogTemplatePATH = "<PATH>"
	CompressionLogTemplateID   = "<ID>"
	CompressionLogTemplateN    = "<N>"

	// ── 格式化模板 ──
	CompressionSearchLineFormat       = "%s:%s:%s"
	CompressionSearchTruncationFormat = "  ...(省略 %d 行)..."
	CompressionSearchSummaryFormat    = "共 %d 个文件, %d 处匹配"
	CompressionLogDedupFormat         = "...(去重 %d 行重复日志)..."
	CompressionSmartCrusherOmitFormat = "%s\n...省略 %d 行..."
	CompressionCSVSpecialChars        = ",\"\n"

	// ── 编程语言名（detector 用）──
	CompressionLangGo     = "go"
	CompressionLangPython = "python"
	CompressionLangRust   = "rust"
	CompressionLangJava   = "java"
	CompressionLangJS     = "js"
)
