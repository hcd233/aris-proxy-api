package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// 本文件承载对话投影共享结构：类型、turn 归组与去重 helper。

// Conversation 是 Trace 的只读对话投影。
type Conversation struct {
	Turns []*ConversationTurn
}

// ConversationTurn 是一个 agent turn。
type ConversationTurn struct {
	TurnID string
	Items  []*ConversationItem
}

// ConversationItem 是消息、工具调用或工具结果。
type ConversationItem struct {
	Kind      string
	Role      string
	Content   string
	ToolName  string
	CallID    string
	Arguments string
	Output    string
	Source    string
	RecordIDs []uint
}

func appendToTurn(conversation *Conversation, turns map[string]*ConversationTurn, turnID string, item *ConversationItem) {
	turn := turns[turnID]
	if turn == nil {
		turn = &ConversationTurn{TurnID: turnID, Items: []*ConversationItem{}}
		turns[turnID] = turn
		conversation.Turns = append(conversation.Turns, turn)
	}
	turn.Items = append(turn.Items, item)
}

// pairToolOutput 把工具输出回填到已存在的 tool_call 上；找不到对应 tool_call 时忽略。
func pairToolOutput(tools map[string]*ConversationItem, callID string, record *TraceEvent, output string) {
	tool := tools[callID]
	if tool == nil {
		return
	}
	tool.Output = output
	tool.RecordIDs = append(tool.RecordIDs, record.ID)
}

// dedupeMessage 按 role+content 去重消息；rollout 来源的消息会升级覆盖 hook 版本。
// 返回 true 表示该条记录不应再追加为新对话项。
func dedupeMessage(seenMessages map[string]*ConversationItem, item *ConversationItem) bool {
	key := item.Role + constant.TraceConversationMessageKeySeparator + item.Content
	existing := seenMessages[key]
	if existing == nil {
		seenMessages[key] = item
		return false
	}
	if isRolloutUpgrade(existing, item) {
		*existing = *item
	}
	return true
}

// dedupeToolCall 按 CallID 去重工具调用；rollout 来源会升级覆盖 hook 版本并合并 RecordIDs。
// 返回 true 表示该条记录不应再追加为新对话项。
func dedupeToolCall(tools map[string]*ConversationItem, item *ConversationItem) bool {
	existing := tools[item.CallID]
	if existing == nil {
		tools[item.CallID] = item
		return false
	}
	if isRolloutUpgrade(existing, item) {
		item.RecordIDs = append(item.RecordIDs, existing.RecordIDs...)
		*existing = *item
	}
	return true
}

func isRolloutUpgrade(existing, item *ConversationItem) bool {
	return existing.Source == constant.TraceRecordSourceHook && item.Source == constant.TraceRecordSourceRollout
}

func hookString(record *TraceEvent, field string) string {
	var payload map[string]sonic.NoCopyRawMessage
	if sonic.Unmarshal(record.Payload, &payload) != nil {
		return ""
	}
	var value string
	if err := sonic.Unmarshal(payload[field], &value); err != nil {
		return ""
	}
	return value
}

// hookRaw 读取 hook payload 字段，字符串字段取值，其余类型保留原始 JSON 文本。
func hookRaw(record *TraceEvent, field string) string {
	var payload map[string]sonic.NoCopyRawMessage
	if sonic.Unmarshal(record.Payload, &payload) != nil {
		return ""
	}
	raw := payload[field]
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := sonic.Unmarshal(raw, &value); err == nil {
		return value
	}
	return string(raw)
}
