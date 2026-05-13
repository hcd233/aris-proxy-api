package dto

import (
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ==================== Conversion: OpenAI -> Unified ====================

// FromOpenAIMessage 从 OpenAI ChatCompletionMessageParam 转换为 UnifiedMessage
//
//	@param msg *OpenAIChatCompletionMessageParam
//	@return *UnifiedMessage
//	@return error
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func FromOpenAIMessage(msg *OpenAIChatCompletionMessageParam) (*vo.UnifiedMessage, error) {
	um := &vo.UnifiedMessage{
		Role: msg.Role,
	}
	if msg.ReasoningContent != nil {
		um.ReasoningContent = *msg.ReasoningContent
	}
	if msg.Name != nil {
		um.Name = *msg.Name
	}
	if msg.ToolCallID != nil {
		um.ToolCallID = *msg.ToolCallID
	}
	if msg.Refusal != nil {
		um.Refusal = *msg.Refusal
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
		um.ToolCalls = make([]*vo.UnifiedToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			utc := &vo.UnifiedToolCall{}
			if tc.ID != nil {
				utc.ID = *tc.ID
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

// convertOpenAIContent 将 OpenAI MessageContent 转换为 UnifiedContent
func convertOpenAIContent(mc *OpenAIMessageContent) (*vo.UnifiedContent, error) {
	if len(mc.Parts) > 0 {
		parts := make([]*vo.UnifiedContentPart, 0, len(mc.Parts))
		for i, p := range mc.Parts {
			part, err := convertOpenAIContentPart(p)
			if err != nil {
				return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert content part[%d]", i)
			}
			parts = append(parts, part)
		}
		return &vo.UnifiedContent{Parts: parts}, nil
	}
	return &vo.UnifiedContent{Text: mc.Text}, nil
}

// convertOpenAIContentPart 将 OpenAI ChatCompletionContentPart 转换为 UnifiedContentPart
func convertOpenAIContentPart(p *OpenAIChatCompletionContentPart) (*vo.UnifiedContentPart, error) {
	switch p.Type {
	case enum.ContentPartTypeText:
		text := ""
		if p.Text != nil {
			text = *p.Text
		}
		return &vo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: text}, nil
	case enum.ContentPartTypeRefusal:
		refusal := ""
		if p.Refusal != nil {
			refusal = *p.Refusal
		}
		return &vo.UnifiedContentPart{Type: enum.ContentPartTypeRefusal, Text: refusal}, nil
	case enum.ContentPartTypeImageURL:
		if p.ImageURL == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "image_url part missing image_url field")
		}
		return &vo.UnifiedContentPart{
			Type:        enum.ContentPartTypeImageURL,
			ImageURL:    p.ImageURL.URL,
			ImageDetail: string(p.ImageURL.Detail),
		}, nil
	case enum.ContentPartTypeInputAudio:
		if p.InputAudio == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "input_audio part missing input_audio field")
		}
		return &vo.UnifiedContentPart{
			Type:        enum.ContentPartTypeInputAudio,
			AudioData:   p.InputAudio.Data,
			AudioFormat: string(p.InputAudio.Format),
		}, nil
	case enum.ContentPartTypeFile:
		if p.File == nil {
			return nil, ierr.New(ierr.ErrDTOConvert, "file part missing file field")
		}
		fileData := ""
		if p.File.FileData != nil {
			fileData = *p.File.FileData
		}
		fileID := ""
		if p.File.FileID != nil {
			fileID = *p.File.FileID
		}
		filename := ""
		if p.File.Filename != nil {
			filename = *p.File.Filename
		}
		return &vo.UnifiedContentPart{
			Type:     enum.ContentPartTypeFile,
			FileData: fileData,
			FileID:   fileID,
			Filename: filename,
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
//	@update 2026-04-22 14:10:00
func FromAnthropicMessage(msg *AnthropicMessageParam) (*vo.UnifiedMessage, error) {
	um := &vo.UnifiedMessage{
		Role: msg.Role,
	}

	if msg.Content == nil {
		return um, nil
	}

	// Content 是 *AnthropicMessageContent，可能是纯字符串或 ContentBlock 数组
	if msg.Content.Text != "" && len(msg.Content.Blocks) == 0 {
		// 纯字符串内容
		um.Content = &vo.UnifiedContent{Text: msg.Content.Text}
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
//	@update 2026-04-22 14:10:00
func FromAnthropicResponse(msg *AnthropicMessage) (*vo.UnifiedMessage, error) {
	um := &vo.UnifiedMessage{
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
//	@update 2026-04-22 14:10:00
func extractAnthropicBlocks(um *vo.UnifiedMessage, blocks []*AnthropicContentBlock) error {
	var (
		textParts         []string
		thinkingParts     []string
		toolCalls         []*vo.UnifiedToolCall
		toolResultID      string
		toolResultContent *vo.UnifiedContent
	)

	for i, block := range blocks {
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			if block.Text != nil {
				textParts = append(textParts, *block.Text)
			}

		case enum.AnthropicContentBlockTypeThinking:
			if block.Thinking != nil {
				thinkingParts = append(thinkingParts, *block.Thinking)
			}

		case enum.AnthropicContentBlockTypeRedactedThinking:
			// redacted_thinking 块不包含用户可见的内容，跳过（data 字段是加密数据）
			continue

		case enum.AnthropicContentBlockTypeToolUse, enum.AnthropicContentBlockTypeServerToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				return ierr.Wrapf(ierr.ErrDTOMarshal, err, "marshal tool_use input for block[%d]", i)
			}
			id := ""
			if block.ID != nil {
				id = *block.ID
			}
			name := ""
			if block.Name != nil {
				name = *block.Name
			}
			toolCalls = append(toolCalls, &vo.UnifiedToolCall{
				ID:        id,
				Name:      name,
				Arguments: args,
			})

		case enum.AnthropicContentBlockTypeToolResult:
			if block.ToolUseID != nil {
				toolResultID = *block.ToolUseID
			}
			if block.Content != nil {
				// tool_result 的 content 可以是字符串或 ContentBlock 数组
				if block.Content.Text != "" && len(block.Content.Blocks) == 0 {
					toolResultContent = &vo.UnifiedContent{Text: block.Content.Text}
				} else if len(block.Content.Blocks) > 0 {
					// 嵌套的 content blocks，提取文本
					var nestedTexts []string
					for _, nested := range block.Content.Blocks {
						if nested.Type == enum.AnthropicContentBlockTypeText && nested.Text != nil {
							nestedTexts = append(nestedTexts, *nested.Text)
						}
						// 其他类型（image 等）也可以在这里扩展
					}
					if len(nestedTexts) > 0 {
						toolResultContent = &vo.UnifiedContent{Text: strings.Join(nestedTexts, "\n")}
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
			um.Content = &vo.UnifiedContent{Text: strings.Join(textParts, "\n")}
		}
	}

	return nil
}
