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

// Conversation 是 Trace 的只读对话投影。
type Conversation struct {
	Turns []*ConversationTurn
}

// ConversationTurn 是一个 Codex turn。
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

// BuildConversation 从原始 TraceEvent 生成不持久化的对话投影。
func BuildConversation(records []*TraceEvent) *Conversation {
	conversation := &Conversation{Turns: []*ConversationTurn{}}
	turns := map[string]*ConversationTurn{}
	seenMessages := map[string]bool{}
	tools := map[string]*ConversationItem{}
	for _, record := range records {
		turnID := record.TurnID
		turn := turns[turnID]
		if turn == nil {
			turn = &ConversationTurn{TurnID: turnID, Items: []*ConversationItem{}}
			turns[turnID] = turn
			conversation.Turns = append(conversation.Turns, turn)
		}
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
			if tool := tools[record.CallID]; tool != nil {
				tool.Output = rolloutString(record, constant.TracePayloadFieldOutput)
				tool.RecordIDs = append(tool.RecordIDs, record.ID)
				continue
			}
		}
		if item == nil || item.Content == "" && item.Kind == constant.TraceConversationKindMessage {
			continue
		}
		if item.Kind == constant.TraceConversationKindMessage {
			key := item.Role + constant.TraceConversationMessageKeySeparator + item.Content
			if seenMessages[key] {
				continue
			}
			seenMessages[key] = true
		}
		turn.Items = append(turn.Items, item)
		if item.Kind == constant.TraceConversationKindToolCall && item.CallID != "" {
			tools[item.CallID] = item
		}
	}
	return conversation
}

func hookMessage(record *TraceEvent, role, field string) *ConversationItem {
	return &ConversationItem{Kind: constant.TraceConversationKindMessage, Role: role, Content: hookString(record, field), Source: record.Source, RecordIDs: []uint{record.ID}}
}

func rolloutMessage(record *TraceEvent, role, field string) *ConversationItem {
	return &ConversationItem{Kind: constant.TraceConversationKindMessage, Role: role, Content: rolloutString(record, field), Source: record.Source, RecordIDs: []uint{record.ID}}
}

func rolloutToolCall(record *TraceEvent) *ConversationItem {
	return &ConversationItem{
		Kind: constant.TraceConversationKindToolCall, Role: constant.TraceConversationRoleAssistant, ToolName: rolloutString(record, constant.TracePayloadFieldName),
		CallID: record.CallID, Arguments: rolloutString(record, constant.TracePayloadFieldArguments), Source: record.Source, RecordIDs: []uint{record.ID},
	}
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
