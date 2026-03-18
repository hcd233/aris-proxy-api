package dto

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// UnifiedMessage 统一消息格式，用于跨 Provider 的消息存储
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedMessage struct {
	Role             enum.Role          `json:"role" doc:"消息角色"`
	Content          any                `json:"content,omitempty" doc:"消息内容(字符串或内容块数组)"`
	ReasoningContent string             `json:"reasoning_content,omitempty" doc:"推理/思考内容"`
	Name             string             `json:"name,omitempty" doc:"参与者名称"`
	ToolCalls        []*UnifiedToolCall `json:"tool_calls,omitempty" doc:"工具调用列表"`
	ToolCallID       string             `json:"tool_call_id,omitempty" doc:"工具调用ID(工具结果消息)"`
	Refusal          string             `json:"refusal,omitempty" doc:"拒绝消息"`
}

// UnifiedToolCall 统一工具调用
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedToolCall struct {
	ID        string `json:"id,omitempty" doc:"工具调用ID"`
	Name      string `json:"name" doc:"工具/函数名称"`
	Arguments string `json:"arguments" doc:"工具参数(JSON字符串)"`
}

// FromOpenAIMessage 从 OpenAI ChatCompletionMessageParam 转换为 UnifiedMessage
//
//	@param msg *ChatCompletionMessageParam
//	@return *UnifiedMessage
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromOpenAIMessage(msg *ChatCompletionMessageParam) *UnifiedMessage {
	um := &UnifiedMessage{
		Role:             msg.Role,
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		Name:             msg.Name,
		ToolCallID:       msg.ToolCallID,
		Refusal:          msg.Refusal,
	}

	// 转换 OpenAI ToolCalls -> UnifiedToolCall
	if len(msg.ToolCalls) > 0 {
		um.ToolCalls = make([]*UnifiedToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			utc := &UnifiedToolCall{
				ID: tc.ID,
			}
			if tc.Function != nil {
				utc.Name = tc.Function.Name
				utc.Arguments = tc.Function.Arguments
			} else if tc.Custom != nil {
				utc.Name = tc.Custom.Name
				utc.Arguments = tc.Custom.Input
			}
			um.ToolCalls = append(um.ToolCalls, utc)
		}
	}

	return um
}

// FromAnthropicMessage 从 Anthropic 请求消息转换为 UnifiedMessage
//
//	@param msg *AnthropicMessageParam
//	@return *UnifiedMessage
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromAnthropicMessage(msg *AnthropicMessageParam) *UnifiedMessage {
	um := &UnifiedMessage{
		Role: msg.Role,
	}

	// Content 是 json.RawMessage，可能是 JSON string 或 []AnthropicContentBlock
	if len(msg.Content) == 0 {
		return um
	}

	// 先尝试作为 string 解析
	var s string
	if err := sonic.Unmarshal(msg.Content, &s); err == nil {
		um.Content = s
		return um
	}

	// 尝试解析为 []AnthropicContentBlock
	var blocks []AnthropicContentBlock
	if err := sonic.Unmarshal(msg.Content, &blocks); err != nil {
		// 无法解析，保留原始 JSON
		um.Content = string(msg.Content)
		return um
	}
	extractAnthropicBlocks(um, blocks)

	return um
}

// FromAnthropicResponse 从 Anthropic 响应消息转换为 UnifiedMessage
//
//	@param msg *AnthropicMessage
//	@return *UnifiedMessage
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromAnthropicResponse(msg *AnthropicMessage) *UnifiedMessage {
	um := &UnifiedMessage{
		Role: msg.Role,
	}

	// 解析 Content []json.RawMessage -> []AnthropicContentBlock
	blocks := make([]AnthropicContentBlock, 0, len(msg.Content))
	for _, raw := range msg.Content {
		var block AnthropicContentBlock
		if err := sonic.Unmarshal(raw, &block); err != nil {
			continue
		}
		blocks = append(blocks, block)
	}

	extractAnthropicBlocks(um, blocks)
	return um
}

// extractAnthropicBlocks 从 Anthropic content blocks 中提取统一字段
//
//	@param um *UnifiedMessage
//	@param blocks []AnthropicContentBlock
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func extractAnthropicBlocks(um *UnifiedMessage, blocks []AnthropicContentBlock) {
	var (
		textParts      []string
		thinkingParts  []string
		toolCalls      []*UnifiedToolCall
		toolResultID   string
		toolResultBody string
		hasOtherBlocks bool
	)

	for i := range blocks {
		block := &blocks[i]
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "thinking":
			thinkingParts = append(thinkingParts, block.Thinking)
		case "tool_use":
			args, _ := sonic.MarshalString(block.Input)
			toolCalls = append(toolCalls, &UnifiedToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		case "tool_result":
			toolResultID = block.ToolUseID
			toolResultBody = block.Text
		default:
			// redacted_thinking, image 等
			hasOtherBlocks = true
		}
	}

	// 设置 ReasoningContent
	if len(thinkingParts) > 0 {
		um.ReasoningContent = strings.Join(thinkingParts, "\n")
	}

	// 设置 ToolCalls
	if len(toolCalls) > 0 {
		um.ToolCalls = toolCalls
	}

	// 设置 ToolCallID 和 Content
	if toolResultID != "" {
		um.ToolCallID = toolResultID
		um.Content = toolResultBody
	} else if !hasOtherBlocks && len(toolCalls) == 0 {
		// 纯文本消息：将文本合并为字符串
		if len(textParts) > 0 {
			um.Content = strings.Join(textParts, "\n")
		}
	} else {
		// 混合类型（文本 + tool_use、image 等）：保留原始 blocks 格式
		um.Content = blocks
	}
}
