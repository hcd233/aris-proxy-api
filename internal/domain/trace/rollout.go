package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// RolloutRecord 是 Codex rollout JSONL 的轻量解析结果。
type RolloutRecord struct {
	RecordType string
	Event      string
	TurnID     string
	CallID     string
	Arguments  string
	Raw        []byte
	Unknown    bool
}

// ParseRolloutRecord 解析一条 rollout envelope，未知类型保留原始数据。
func ParseRolloutRecord(raw []byte) (RolloutRecord, error) {
	var envelope map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return RolloutRecord{}, err
	}
	record := RolloutRecord{Raw: append([]byte(nil), raw...)}
	if err := sonic.Unmarshal(envelope["type"], &record.RecordType); err != nil {
		return RolloutRecord{}, err
	}

	knownTypes := map[string]bool{
		constant.TraceRolloutTypeSessionMeta:  true,
		constant.TraceRolloutTypeTurnContext:  true,
		constant.TraceRolloutTypeResponseItem: true,
		constant.TraceRolloutTypeEventMsg:     true,
	}
	if !knownTypes[record.RecordType] {
		record.Unknown = true
		record.Event = record.RecordType
		return record, nil
	}
	var payload map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(envelope["payload"], &payload); err != nil {
		return record, err
	}
	if value := payload[constant.TracePayloadFieldType]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.Event); err != nil {
			return record, err
		}
	}
	if value := payload[constant.TracePayloadFieldTurnID]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.TurnID); err != nil {
			return record, err
		}
	}
	if value := payload[constant.TracePayloadFieldCallID]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.CallID); err != nil {
			return record, err
		}
	}
	if record.Event == constant.TraceConversationEventFunctionCall {
		if err := sonic.Unmarshal(payload[constant.TracePayloadFieldArguments], &record.Arguments); err != nil {
			return record, err
		}
	}
	return record, nil
}

// buildCodexConversation 从原始 TraceEvent 生成不持久化的 codex 对话投影。
func buildCodexConversation(records []*TraceEvent) *Conversation {
	conversation := &Conversation{Turns: []*ConversationTurn{}}
	turns := map[string]*ConversationTurn{}
	seenMessages := map[string]*ConversationItem{}
	tools := map[string]*ConversationItem{}
	for _, record := range records {
		item := projectConversationItem(record, tools)
		if item == nil {
			continue
		}
		if item.Kind == constant.TraceConversationKindMessage && dedupeMessage(seenMessages, item) {
			continue
		}
		if item.Kind == constant.TraceConversationKindToolCall && item.CallID != "" && dedupeToolCall(tools, item) {
			continue
		}
		appendToTurn(conversation, turns, record.TurnID, item)
	}
	return conversation
}

// projectConversationItem 把单条原始记录投影为对话项；配对类事件（function/tool 输出）
// 直接回填已存在的 tool_call，本身不产生新的对话项。
func projectConversationItem(record *TraceEvent, tools map[string]*ConversationItem) *ConversationItem {
	var item *ConversationItem
	switch record.Event {
	case constant.TraceConversationEventUserPrompt:
		item = hookMessage(record, constant.TraceConversationRoleUser, constant.TracePayloadFieldPrompt)
	case constant.TraceConversationEventStop:
		item = hookMessage(record, constant.TraceConversationRoleAssistant, constant.TracePayloadFieldLastMessage)
	case constant.TraceConversationEventUserMessage:
		item = rolloutMessage(record, constant.TraceConversationRoleUser, constant.TracePayloadFieldMessage)
	case constant.TraceConversationEventAgentMessage:
		item = rolloutMessage(record, constant.TraceConversationRoleAssistant, constant.TracePayloadFieldMessage)
	case constant.TraceConversationEventFunctionCall:
		item = rolloutToolCall(record)
	case constant.TraceConversationEventFunctionOutput:
		pairToolOutput(tools, record.CallID, record, rolloutString(record, constant.TracePayloadFieldOutput))
	case constant.TraceConversationEventPreToolUse:
		item = hookToolCall(record)
	case constant.TraceConversationEventPostToolUse:
		pairToolOutput(tools, hookCallID(record), record, hookRaw(record, constant.TracePayloadFieldToolResponse))
	}
	if item == nil || item.Kind == constant.TraceConversationKindMessage && item.Content == "" {
		return nil
	}
	return item
}

func hookMessage(record *TraceEvent, role, field string) *ConversationItem {
	return &ConversationItem{Kind: constant.TraceConversationKindMessage, Role: role, Content: hookString(record, field), Source: record.Source, RecordIDs: []uint{record.ID}}
}

func rolloutMessage(record *TraceEvent, role, field string) *ConversationItem {
	return &ConversationItem{Kind: constant.TraceConversationKindMessage, Role: role, Content: rolloutString(record, field), Source: record.Source, RecordIDs: []uint{record.ID}}
}

func hookToolCall(record *TraceEvent) *ConversationItem {
	return &ConversationItem{
		Kind: constant.TraceConversationKindToolCall, Role: constant.TraceConversationRoleAssistant,
		ToolName: hookString(record, constant.TracePayloadFieldToolName), CallID: hookCallID(record),
		Arguments: hookRaw(record, constant.TracePayloadFieldToolInput), Source: record.Source, RecordIDs: []uint{record.ID},
	}
}

func hookCallID(record *TraceEvent) string {
	if callID := hookString(record, constant.TracePayloadFieldToolUseID); callID != "" {
		return callID
	}
	return record.CallID
}

func rolloutToolCall(record *TraceEvent) *ConversationItem {
	return &ConversationItem{
		Kind: constant.TraceConversationKindToolCall, Role: constant.TraceConversationRoleAssistant, ToolName: rolloutString(record, constant.TracePayloadFieldName),
		CallID: record.CallID, Arguments: rolloutString(record, constant.TracePayloadFieldArguments), Source: record.Source, RecordIDs: []uint{record.ID},
	}
}

func rolloutString(record *TraceEvent, field string) string {
	var envelope map[string]sonic.NoCopyRawMessage
	if sonic.Unmarshal(record.Payload, &envelope) != nil {
		return ""
	}
	var payload map[string]sonic.NoCopyRawMessage
	if sonic.Unmarshal(envelope["payload"], &payload) != nil {
		return ""
	}
	var value string
	if err := sonic.Unmarshal(payload[field], &value); err != nil {
		return ""
	}
	return value
}
