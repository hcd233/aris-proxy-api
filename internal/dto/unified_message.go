package dto

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ==================== Unified Content Types ====================

// UnifiedContent 统一消息内容（替代 any），纯文本时仅使用 Text，多部分内容时使用 Parts
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedContent struct {
	Text  string                `json:"-"`
	Parts []*UnifiedContentPart `json:"-"`
}

// UnmarshalJSON 自定义反序列化：兼容旧数据（string / array / object）
func (c *UnifiedContent) UnmarshalJSON(data []byte) error {
	// 1. 尝试作为字符串
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	// 2. 尝试作为 Parts 数组
	return sonic.Unmarshal(data, &c.Parts)
}

// MarshalJSON 自定义序列化：Parts 优先，否则输出字符串
func (c UnifiedContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

// UnifiedContentPart 统一内容部分
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedContentPart struct {
	Type        string `json:"type"`                   // text/image_url/input_audio/file/refusal
	Text        string `json:"text,omitempty"`         // type=text 或 type=refusal
	ImageURL    string `json:"image_url,omitempty"`    // type=image_url: URL 或 base64
	ImageDetail string `json:"image_detail,omitempty"` // type=image_url: 细节级别
	AudioData   string `json:"audio_data,omitempty"`   // type=input_audio
	AudioFormat string `json:"audio_format,omitempty"` // type=input_audio
	FileData    string `json:"file_data,omitempty"`    // type=file
	FileID      string `json:"file_id,omitempty"`      // type=file
	Filename    string `json:"filename,omitempty"`     // type=file
}

// ==================== Unified Message ====================

// UnifiedMessage 统一消息格式，用于跨 Provider 的消息存储
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedMessage struct {
	Role             enum.Role          `json:"role" doc:"消息角色"`
	Content          *UnifiedContent    `json:"content,omitempty" doc:"消息内容"`
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

// ==================== Conversion: OpenAI -> Unified ====================

// FromOpenAIMessage 从 OpenAIChatCompletionMessageParam 转换为 UnifiedMessage
//
//	@param msg *OpenAIChatCompletionMessageParam
//	@return *UnifiedMessage
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromOpenAIMessage(msg *OpenAIChatCompletionMessageParam) (*UnifiedMessage, error) {
	um := &UnifiedMessage{
		Role:             msg.Role,
		ReasoningContent: msg.ReasoningContent,
		Name:             msg.Name,
		ToolCallID:       msg.ToolCallID,
		Refusal:          msg.Refusal,
	}

	// 转换 Content: *OpenAIMessageContent -> *UnifiedContent
	if msg.Content != nil {
		content, err := convertOpenAIContent(msg.Content)
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "convert openai content")
		}
		um.Content = content
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

	return um, nil
}

// convertOpenAIContent 将 OpenAIMessageContent 转换为 UnifiedContent
func convertOpenAIContent(mc *OpenAIMessageContent) (*UnifiedContent, error) {
	if len(mc.Parts) > 0 {
		parts := make([]*UnifiedContentPart, 0, len(mc.Parts))
		for i, p := range mc.Parts {
			part, err := convertOpenAIContentPart(p)
			if err != nil {
				return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert content part[%d]", i)
			}
			parts = append(parts, part)
		}
		return &UnifiedContent{Parts: parts}, nil
	}
	return &UnifiedContent{Text: mc.Text}, nil
}

// convertOpenAIContentPart 将 OpenAIChatCompletionContentPart 转换为 UnifiedContentPart
func convertOpenAIContentPart(p *OpenAIChatCompletionContentPart) (*UnifiedContentPart, error) {
	switch p.Type {
	case enum.ContentPartTypeText:
		return &UnifiedContentPart{Type: enum.ContentPartTypeText, Text: p.Text}, nil
	case enum.ContentPartTypeRefusal:
		return &UnifiedContentPart{Type: enum.ContentPartTypeRefusal, Text: p.Refusal}, nil
	case enum.ContentPartTypeImageURL:
		if p.ImageURL == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "image_url part missing image_url field")
		}
		return &UnifiedContentPart{
			Type:        enum.ContentPartTypeImageURL,
			ImageURL:    p.ImageURL.URL,
			ImageDetail: string(p.ImageURL.Detail),
		}, nil
	case enum.ContentPartTypeInputAudio:
		if p.InputAudio == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "input_audio part missing input_audio field")
		}
		return &UnifiedContentPart{
			Type:        enum.ContentPartTypeInputAudio,
			AudioData:   p.InputAudio.Data,
			AudioFormat: string(p.InputAudio.Format),
		}, nil
	case enum.ContentPartTypeFile:
		if p.File == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "file part missing file field")
		}
		return &UnifiedContentPart{
			Type:     enum.ContentPartTypeFile,
			FileData: p.File.FileData,
			FileID:   p.File.FileID,
			Filename: p.File.Filename,
		}, nil
	default:
		return nil, ierr.Newf(ierr.ErrDTOConvert, "unknown content part type: %q", p.Type)
	}
}

// ==================== Conversion: Anthropic -> Unified ====================

// FromAnthropicMessage 从 Anthropic 请求消息转换为 UnifiedMessage
//
//	@param msg *AnthropicMessageParam
//	@return *UnifiedMessage
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromAnthropicMessage(msg *AnthropicMessageParam) (*UnifiedMessage, error) {
	um := &UnifiedMessage{
		Role: msg.Role,
	}

	if msg.Content == nil {
		return um, nil
	}

	// Content 是 *AnthropicMessageContent，可能是纯字符串或 ContentBlock 数组
	if msg.Content.Text != "" && len(msg.Content.Blocks) == 0 {
		// 纯字符串内容
		um.Content = &UnifiedContent{Text: msg.Content.Text}
		return um, nil
	}

	if len(msg.Content.Blocks) > 0 {
		if err := extractAnthropicBlocks(um, msg.Content.Blocks); err != nil {
			return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "extract anthropic blocks from request")
		}
		return um, nil
	}

	// 空内容
	return um, nil
}

// FromAnthropicResponse 从 Anthropic 响应消息转换为 UnifiedMessage
//
//	@param msg *AnthropicMessage
//	@return *UnifiedMessage
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromAnthropicResponse(msg *AnthropicMessage) (*UnifiedMessage, error) {
	um := &UnifiedMessage{
		Role: msg.Role,
	}

	if len(msg.Content) == 0 {
		return um, nil
	}

	if err := extractAnthropicBlocks(um, msg.Content); err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "extract anthropic blocks from response")
	}
	return um, nil
}

// extractAnthropicBlocks 从 Anthropic content blocks 中提取统一字段
//
//	@param um *UnifiedMessage
//	@param blocks []*AnthropicContentBlock
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func extractAnthropicBlocks(um *UnifiedMessage, blocks []*AnthropicContentBlock) error {
	var (
		textParts         []string
		thinkingParts     []string
		toolCalls         []*UnifiedToolCall
		toolResultID      string
		toolResultContent *UnifiedContent
	)

	for i, block := range blocks {
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			textParts = append(textParts, block.Text)

		case enum.AnthropicContentBlockTypeThinking:
			thinkingParts = append(thinkingParts, block.Thinking)

		case enum.AnthropicContentBlockTypeRedactedThinking:
			// redacted_thinking 块不包含用户可见的内容，跳过（data 字段是加密数据）
			continue

		case enum.AnthropicContentBlockTypeToolUse, enum.AnthropicContentBlockTypeServerToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				return ierr.Wrapf(ierr.ErrDTOMarshal, err, "marshal tool_use input for block[%d]", i)
			}
			toolCalls = append(toolCalls, &UnifiedToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})

		case enum.AnthropicContentBlockTypeToolResult:
			toolResultID = block.ToolUseID
			if block.Content != nil {
				// tool_result 的 content 可以是字符串或 ContentBlock 数组
				if block.Content.Text != "" && len(block.Content.Blocks) == 0 {
					toolResultContent = &UnifiedContent{Text: block.Content.Text}
				} else if len(block.Content.Blocks) > 0 {
					// 嵌套的 content blocks，提取文本
					var nestedTexts []string
					for _, nested := range block.Content.Blocks {
						if nested.Type == enum.AnthropicContentBlockTypeText {
							nestedTexts = append(nestedTexts, nested.Text)
						}
						// 其他类型（image 等）也可以在这里扩展
					}
					if len(nestedTexts) > 0 {
						toolResultContent = &UnifiedContent{Text: strings.Join(nestedTexts, "\n")}
					}
				}
			}

		case enum.AnthropicContentBlockTypeWebSearchToolResult, enum.AnthropicContentBlockTypeCodeExecutionToolResult, enum.AnthropicContentBlockTypeWebFetchToolResult:
			// 服务器工具结果，content 中包含搜索/执行结果等，跳过详细存储
			continue

		default:
			return ierr.Newf(ierr.ErrDTOConvert, "unknown anthropic content block type: %q at block[%d]", block.Type, i)
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
		um.Content = toolResultContent
	} else {
		// 非 tool_result 消息：合并文本
		if len(textParts) > 0 {
			um.Content = &UnifiedContent{Text: strings.Join(textParts, "\n")}
		}
	}

	return nil
}
