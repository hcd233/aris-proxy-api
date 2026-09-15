package constant

// 模型默认上下文窗口与最大输出长度（tokens），用于创建模型时未显式指定的兜底值。
const (
	DefaultModelContextLength   = 128000
	DefaultModelMaxOutputTokens = 64000
)

// DefaultModelCapabilities 模型默认能力（仅文本输入），用于创建模型时未显式指定的兜底值
var DefaultModelCapabilities = []string{"text"}

const (
	// ModelUpdateConflictMessage 乐观锁冲突提示：并发改名或模型行已不存在时，
	// 历史同步的目标不可信，事务整体回滚。
	ModelUpdateConflictMessage = "Model was modified concurrently. Please retry."
)
