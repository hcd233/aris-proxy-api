package enum

// BlockedAction 敏感词命中处理动作
type BlockedAction = string

const (
	BlockedActionDeny BlockedAction = "deny" // 命中即拦截，返回 403
	BlockedActionOmit BlockedAction = "omit" // 命中忽略，不记录 session/message/tool
)
