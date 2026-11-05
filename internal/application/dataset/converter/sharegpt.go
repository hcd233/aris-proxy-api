// Package converter provides UnifiedMessage → ShareGPT format conversion.
package converter

import (
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/session"
)

// ShareGPTConversation 一条 ShareGPT 训练样本
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ShareGPTConversation struct {
	Conversations []ShareGPTMessage `json:"conversations"`
	Tools         []ShareGPTTool    `json:"tools,omitempty"`
	// Images 按出现顺序排列，与各 value 中的 <image> 占位符一一对应（LLaMA-Factory 约定）
	Images []string `json:"images,omitempty"`
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
//	@update 2026-07-06 10:00:00
type ShareGPTTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  *vo.JSONSchemaProperty `json:"parameters,omitempty"`
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
	return ConvertSessionWithImages(messages, tools, nil)
}

// ConvertSessionWithImages 同 ConvertSession，并按 resolve 导出图片
//
//	resolve 把图片部分解析为导出值，返回 false 时跳过这张图及其占位符；
//	为 nil 时不输出图片与占位符，结果与 ConvertSession 一致。
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func ConvertSessionWithImages(
	messages []*session.MessageDetailProjection,
	tools []*session.ToolDetailProjection,
	resolve func(part *vo.UnifiedContentPart) (string, bool),
) *ShareGPTConversation {
	conv := &ShareGPTConversation{
		Conversations: make([]ShareGPTMessage, 0, len(messages)),
	}

	for _, msg := range messages {
		if msg == nil || msg.Message == nil {
			continue
		}
		conv.Conversations = append(conv.Conversations, convertMessage(msg.Message, resolve, &conv.Images))
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
				Parameters:  t.Tool.Parameters,
			})
		}
	}

	return conv
}

func convertMessage(m *vo.UnifiedMessage, resolve func(part *vo.UnifiedContentPart) (string, bool), images *[]string) ShareGPTMessage {
	from := roleToShareGPTFrom(m.Role)
	value := buildValue(m, resolve, images)

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

func buildValue(m *vo.UnifiedMessage, resolve func(part *vo.UnifiedContentPart) (string, bool), images *[]string) string {
	var parts []string

	if m.ReasoningContent != "" {
		parts = append(parts, constant.ThinkTagOpen+m.ReasoningContent+constant.ThinkTagClose)
	}

	textContent := extractText(m.Content, resolve, images)
	if textContent != "" {
		parts = append(parts, textContent)
	}

	if m.Refusal != "" {
		parts = append(parts, m.Refusal)
	}

	return strings.Join(parts, constant.DoubleNewline)
}

func extractText(c *vo.UnifiedContent, resolve func(part *vo.UnifiedContentPart) (string, bool), images *[]string) string {
	if c == nil {
		return ""
	}
	if c.Text != "" {
		return c.Text
	}
	var textParts []string
	for _, p := range c.Parts {
		switch {
		case p.Type == constant.ContentPartTypeText && p.Text != "":
			textParts = append(textParts, p.Text)
		case p.Type == enum.ContentPartTypeImageURL && resolve != nil:
			if url, ok := resolve(p); ok {
				textParts = append(textParts, constant.ShareGPTImagePlaceholder)
				*images = append(*images, url)
			}
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
