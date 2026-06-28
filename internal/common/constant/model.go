package constant

// 模型默认上下文窗口与最大输出长度（tokens），用于创建模型时未显式指定的兜底值。
const (
	DefaultModelContextLength   = 128000
	DefaultModelMaxOutputTokens = 64000
)
