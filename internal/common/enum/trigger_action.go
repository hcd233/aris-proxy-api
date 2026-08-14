package enum

// TriggerAction 触发词命中处理动作
type TriggerAction = string

const (
	TriggerActionDeny    TriggerAction = "deny"    // 命中即拦截，返回协议内容拦截消息（200 content_filter/refusal）
	TriggerActionOmit    TriggerAction = "omit"    // 命中忽略，放行转发但跳过 session/message/tool 存储
	TriggerActionCapture TriggerAction = "capture" // 命中捕获：保存触发消息之前的对话历史，不请求上游，返回固定回复
)

// TriggerActions 全部合法动作。命令层白名单必须基于该集合判断，
// 新增动作时同步补充此处与 DTO enum，避免三层口径不一致。
var TriggerActions = []TriggerAction{TriggerActionDeny, TriggerActionOmit, TriggerActionCapture}
