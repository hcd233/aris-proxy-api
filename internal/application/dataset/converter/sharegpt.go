// Package converter provides UnifiedMessage → ShareGPT format conversion.
package converter

import (
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

// ShareGPTConversation 一条 ShareGPT 训练样本
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ShareGPTConversation struct {
	Conversations []ShareGPTMessage `json:"conversations"`
	Tools         []ShareGPTTool    `json:"tools,omitempty"`
}

// ShareGPTMessage ShareGPT 对话消息
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ShareGPTMessage struct {
	From         string            `json:"from"`
	Value        string            `json:"value"`
	FunctionCall *ShareGPTFuncCall `json:"function_call,omitempty"`
	Name         string            `json:"name,omitempty"`
}

// ShareGPTFuncCall ShareGPT 工具调用
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ShareGPTFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ShareGPTTool ShareGPT 工具定义
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ShareGPTTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ConvertSession 将一条会话的投影转换为 ShareGPT 对话记录。
// 全量保留：多轮对话、思维链（<think> 标签）、工具调用和工具响应。
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func ConvertSession(
	messages []*session.MessageDetailProjection,
	tools []*session.ToolDetailProjection,
) *ShareGPTConversation {
	conv := &ShareGPTConversation{
		Conversations: make([]ShareGPTMessage, 0, len(messages)),
	}

	for _, msg := range messages {
		if msg == nil || msg.Message == nil {
			continue
		}
		conv.Conversations = append(conv.Conversations, convertMessage(msg.Message))
	}

	if len(tools) > 0 {
		conv.Tools = make([]ShareGPTTool, 0, len(tools))
		for _, t := range tools {
			if t == nil || t.Tool == nil {
				continue
			}
			conv.Tools = append(conv.Tools, ShareGPTTool{
				Name:        t.Tool.Name,
				Description: t.Tool.Description,
			})
		}
	}

	return conv
}

func convertMessage(m *vo.UnifiedMessage) ShareGPTMessage {
	from := roleToShareGPTFrom(m.Role)
	value := buildValue(m)

	msg := ShareGPTMessage{
		From:  from,
		Value: value,
	}

	if len(m.ToolCalls) > 0 && m.Role == enum.RoleAssistant {
		tc := m.ToolCalls[0]
		msg.FunctionCall = &ShareGPTFuncCall{
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}

	if m.Role == enum.RoleTool {
		msg.Name = m.ToolCallID
	}

	return msg
}

func roleToShareGPTFrom(role enum.Role) string {
	switch role {
	case enum.RoleSystem, enum.RoleDeveloper:
		return constant.ShareGPTFromSystem
	case enum.RoleUser:
		return constant.ShareGPTFromUser
	case enum.RoleAssistant:
		return constant.ShareGPTFromAssistant
	case enum.RoleTool, enum.RoleFunction:
		return constant.ShareGPTFromFunction
	default:
		return constant.ShareGPTFromUser
	}
}

func buildValue(m *vo.UnifiedMessage) string {
	var parts []string

	if m.ReasoningContent != "" {
		parts = append(parts, constant.ThinkTagOpen+m.ReasoningContent+constant.ThinkTagClose)
	}

	textContent := extractText(m.Content)
	if textContent != "" {
		parts = append(parts, textContent)
	}

	if m.Refusal != "" {
		parts = append(parts, m.Refusal)
	}

	return strings.Join(parts, constant.DoubleNewline)
}

func extractText(c *vo.UnifiedContent) string {
	if c == nil {
		return ""
	}
	if c.Text != "" {
		return c.Text
	}
	var textParts []string
	for _, p := range c.Parts {
		if p.Type == constant.ContentPartTypeText && p.Text != "" {
			textParts = append(textParts, p.Text)
		}
	}
	return strings.Join(textParts, "\n")
}

// MarshalJSONLine 将 ShareGPT 对话序列化为一行 JSON（不含换行符）
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func MarshalJSONLine(c *ShareGPTConversation) ([]byte, error) {
	return sonic.Marshal(c)
}
