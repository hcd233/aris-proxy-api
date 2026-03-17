package dto

import (
	"encoding/json"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// UnifiedMessage 统一消息格式，用于跨 Provider 的消息存储
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type UnifiedMessage struct {
	Provider    enum.ProviderType `json:"provider" doc:"消息来源提供者"`
	Role        enum.Role         `json:"role" doc:"消息角色"`
	TextContent string            `json:"text_content" doc:"提取的纯文本内容"`
	RawContent  json.RawMessage   `json:"raw_content" doc:"原始完整JSON"`
}

// FromOpenAIMessage 从 OpenAI ChatCompletionMessageParam 转换为 UnifiedMessage
//
//	@param msg *ChatCompletionMessageParam
//	@return *UnifiedMessage
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func FromOpenAIMessage(msg *ChatCompletionMessageParam) *UnifiedMessage {
	textContent := extractOpenAITextContent(msg)
	rawContent, _ := sonic.Marshal(msg)
	return &UnifiedMessage{
		Provider:    enum.ProviderOpenAI,
		Role:        msg.Role,
		TextContent: textContent,
		RawContent:  rawContent,
	}
}

// FromAnthropicMessage 从 Anthropic 消息数据转换为 UnifiedMessage
//
//	@param role enum.Role
//	@param rawContent json.RawMessage 原始消息JSON（包含完整的 Anthropic content blocks）
//	@return *UnifiedMessage
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func FromAnthropicMessage(role enum.Role, rawContent json.RawMessage) *UnifiedMessage {
	textContent := extractAnthropicTextContent(rawContent)
	return &UnifiedMessage{
		Provider:    enum.ProviderAnthropic,
		Role:        role,
		TextContent: textContent,
		RawContent:  rawContent,
	}
}

// extractOpenAITextContent 从 OpenAI 消息中提取纯文本内容
func extractOpenAITextContent(msg *ChatCompletionMessageParam) string {
	if msg.Content == nil {
		return ""
	}
	// Content 可能是 string 或 array
	switch v := msg.Content.(type) {
	case string:
		return v
	default:
		// 尝试序列化后解析数组格式
		data, err := sonic.Marshal(v)
		if err != nil {
			return ""
		}
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := sonic.Unmarshal(data, &parts); err != nil {
			return string(data)
		}
		var texts []string
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
}

// extractAnthropicTextContent 从 Anthropic 原始 JSON 中提取纯文本内容
func extractAnthropicTextContent(rawContent json.RawMessage) string {
	// Anthropic 消息结构: {"role":"...","content":"..." 或 "content":[...]}
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := sonic.Unmarshal(rawContent, &msg); err != nil || len(msg.Content) == 0 {
		return ""
	}

	// content 可能是字符串
	var strContent string
	if err := sonic.Unmarshal(msg.Content, &strContent); err == nil {
		return strContent
	}

	// content 是 ContentBlock 数组
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := sonic.Unmarshal(msg.Content, &blocks); err != nil {
		return ""
	}
	var texts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}
