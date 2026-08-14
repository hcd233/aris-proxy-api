package enum

// TriggerAction 触发词命中处理动作
type TriggerAction = string

const (
	TriggerActionDeny    TriggerAction = "deny"    // 命中即拦截，返回 403
	TriggerActionOmit    TriggerAction = "omit"    // 命中忽略，不记录 session/message/tool
	TriggerActionCapture TriggerAction = "capture" // 命中捕获：保存触发消息之前的对话历史，不请求上游，返回固定回复
)
