package trace

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// claudeTranscriptPayload 仅抽取投影所需字段；未知字段原样留在 TraceEvent.Payload。
type claudeTranscriptPayload struct {
	UUID        string `json:"uuid"`
	PromptID    string `json:"promptId"`
	IsSidechain bool   `json:"isSidechain"`
	Message     *struct {
		Role    string                 `json:"role"`
		Content sonic.NoCopyRawMessage `json:"content"`
	} `json:"message"`
}

type claudeSidechainFlag struct {
	IsSidechain bool `json:"isSidechain"`
}

type claudeContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Input     sonic.NoCopyRawMessage `json:"input"`
	ToolUseID string                 `json:"tool_use_id"`
	Content   sonic.NoCopyRawMessage `json:"content"`
}

// buildClaudeConversation 从 Claude transcript + hook 记录投影对话视图。
// turn 归组：transcript 真实输入记录的 promptId→uuid 建 alias；hook 事件按 prompt_id 归并。
func buildClaudeConversation(records []*TraceEvent) *Conversation {
	alias := map[string]string{}
	for _, record := range records {
		if record.Source != constant.TraceRecordSourceRollout ||
			record.Event != constant.TraceClaudeEventUserPrompt {
			continue
		}
		var payload claudeTranscriptPayload
		if sonic.Unmarshal(record.Payload, &payload) == nil &&
			payload.PromptID != "" && payload.UUID != "" {
			alias[payload.PromptID] = payload.UUID
		}
	}

	conversation := &Conversation{Turns: []*ConversationTurn{}}
	state := &claudeProjectionState{
		turns:        map[string]*ConversationTurn{},
		seenMessages: map[string]*ConversationItem{},
		tools:        map[string]*ConversationItem{},
		currentTurn:  constant.TraceConversationDefaultTurn,
	}
	for _, record := range records {
		if claudeSidechain(record) || claudeSubagentHook(record) {
			continue
		}
		turnID := resolveClaudeTurn(record, alias, state.currentTurn)
		state.appendItems(conversation, turnID, projectClaudeItems(record, state.tools))
		if isClaudeTurnOpener(record) {
			state.currentTurn = turnID
		}
	}
	return conversation
}

// claudeProjectionState 投影过程中的归组与去重状态。
type claudeProjectionState struct {
	turns        map[string]*ConversationTurn
	seenMessages map[string]*ConversationItem
	tools        map[string]*ConversationItem
	currentTurn  string
}

func (s *claudeProjectionState) appendItems(conversation *Conversation, turnID string, items []*ConversationItem) {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Kind == constant.TraceConversationKindMessage && item.Content == "" {
			continue
		}
		if item.Kind == constant.TraceConversationKindMessage && dedupeMessage(s.seenMessages, item) {
			continue
		}
		if item.Kind == constant.TraceConversationKindToolCall && item.CallID != "" && dedupeToolCall(s.tools, item) {
			continue
		}
		appendToTurn(conversation, s.turns, turnID, item)
	}
}

func resolveClaudeTurn(record *TraceEvent, alias map[string]string, currentTurn string) string {
	if isClaudeTurnOpener(record) {
		var payload claudeTranscriptPayload
		if sonic.Unmarshal(record.Payload, &payload) == nil && payload.UUID != "" {
			return payload.UUID
		}
	}
	if record.TurnID != "" {
		if aliasID, ok := alias[record.TurnID]; ok {
			return aliasID
		}
		return "prompt:" + record.TurnID // transcript 行未入库时的稳定兜底
	}
	return currentTurn
}

func isClaudeTurnOpener(record *TraceEvent) bool {
	return record.Source == constant.TraceRecordSourceRollout &&
		record.Event == constant.TraceClaudeEventUserPrompt
}

func claudeSidechain(record *TraceEvent) bool {
	if record.Source != constant.TraceRecordSourceRollout {
		return false
	}
	var payload claudeSidechainFlag
	return sonic.Unmarshal(record.Payload, &payload) == nil && payload.IsSidechain
}

// claudeSubagentHook 子代理内触发的 hook（payload 携带 agent_id）不进主对话投影。
func claudeSubagentHook(record *TraceEvent) bool {
	if record.Source != constant.TraceRecordSourceHook {
		return false
	}
	return hookString(record, constant.TracePayloadFieldAgentID) != ""
}

func projectClaudeItems(record *TraceEvent, tools map[string]*ConversationItem) []*ConversationItem {
	switch record.Event {
	case constant.TraceClaudeEventUserPrompt:
		return claudeUserPromptItems(record)
	case constant.TraceClaudeEventToolResult:
		claudePairToolResults(record, tools)
		return nil
	case constant.TraceClaudeEventAssistantMessage:
		return claudeAssistantItems(record)
	case constant.TraceConversationEventUserPrompt: // hook UserPromptSubmit
		return []*ConversationItem{hookMessage(record, constant.TraceConversationRoleUser, constant.TracePayloadFieldPrompt)}
	case constant.TraceConversationEventStop: // hook Stop
		return []*ConversationItem{hookMessage(record, constant.TraceConversationRoleAssistant, constant.TracePayloadFieldLastMessage)}
	case constant.TraceConversationEventPreToolUse:
		return []*ConversationItem{hookToolCall(record)}
	case constant.TraceConversationEventPostToolUse:
		pairToolOutput(tools, hookCallID(record), record, hookRaw(record, constant.TracePayloadFieldToolResponse))
		return nil
	default:
		return nil
	}
}

func claudeUserPromptItems(record *TraceEvent) []*ConversationItem {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return nil
	}
	var text string
	if sonic.Unmarshal(payload.Message.Content, &text) != nil || text == "" {
		return nil
	}
	return []*ConversationItem{{
		Kind:      constant.TraceConversationKindMessage,
		Role:      constant.TraceConversationRoleUser,
		Content:   text,
		Source:    record.Source,
		RecordIDs: []uint{record.ID},
	}}
}

func claudeAssistantItems(record *TraceEvent) []*ConversationItem {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return nil
	}
	var blocks []claudeContentBlock
	if sonic.Unmarshal(payload.Message.Content, &blocks) != nil {
		return nil
	}
	items := []*ConversationItem{}
	for _, block := range blocks {
		switch block.Type {
		case constant.TraceClaudeBlockText:
			if block.Text == "" {
				continue
			}
			items = append(items, &ConversationItem{
				Kind:      constant.TraceConversationKindMessage,
				Role:      constant.TraceConversationRoleAssistant,
				Content:   block.Text,
				Source:    record.Source,
				RecordIDs: []uint{record.ID},
			})
		case constant.TraceClaudeBlockToolUse:
			arguments := ""
			if len(block.Input) > 0 {
				arguments = string(bytes.TrimSpace(block.Input))
			}
			items = append(items, &ConversationItem{
				Kind:      constant.TraceConversationKindToolCall,
				Role:      constant.TraceConversationRoleAssistant,
				ToolName:  block.Name,
				CallID:    block.ID,
				Arguments: arguments,
				Source:    record.Source,
				RecordIDs: []uint{record.ID},
			})
		default: // thinking / 其他块：投影 v1 跳过
		}
	}
	return items
}

// claudePairToolResults 把 user 记录内的 tool_result 块逐块回填到既有 tool_call。
func claudePairToolResults(record *TraceEvent, tools map[string]*ConversationItem) {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return
	}
	var blocks []claudeContentBlock
	if sonic.Unmarshal(payload.Message.Content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		if block.Type != constant.TraceClaudeBlockToolResult || block.ToolUseID == "" {
			continue
		}
		pairToolOutput(tools, block.ToolUseID, record, claudeToolResultText(block.Content))
	}
}

// claudeToolResultText tool_result 的 content：字符串取值，其余类型保留原始 JSON。
func claudeToolResultText(raw sonic.NoCopyRawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if sonic.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(bytes.TrimSpace(raw))
}
