package enum

// BlockedAction 敏感词命中处理动作
type BlockedAction = string

const (
	BlockedActionDeny  BlockedAction = "deny"  // 命中即拦截，返回 403
	BlockedActionAllow BlockedAction = "allow" // 命中放行，但不记录 session/message/tool
)
