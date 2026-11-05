package dto

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
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
		um.ToolCalls = lo.Map(msg.ToolCalls, func(tc *OpenAIChatCompletionMessageToolCall, _ int) *vo.UnifiedToolCall {
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
			return utc
		})
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
		images            []*vo.UnifiedContentPart
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
			continue

		case enum.AnthropicContentBlockTypeImage:
			if part := anthropicImagePart(block); part != nil {
				images = append(images, part)
			}

		case enum.AnthropicContentBlockTypeToolUse, enum.AnthropicContentBlockTypeServerToolUse:
			tc, err := buildToolCallFromAnthropicBlock(block, i)
			if err != nil {
				return err
			}
			toolCalls = append(toolCalls, tc)

		case enum.AnthropicContentBlockTypeToolResult:
			id, content := extractToolResultContent(block)
			if id != "" {
				toolResultID = id
			}
			toolResultContent = content

		case enum.AnthropicContentBlockTypeWebSearchToolResult, enum.AnthropicContentBlockTypeCodeExecutionToolResult, enum.AnthropicContentBlockTypeWebFetchToolResult:
			continue

		default:
			return ierr.Newf(ierr.ErrDTOConvert, "unknown anthropic content block type: %q at block[%d]", block.Type, i)
		}
	}

	if len(thinkingParts) > 0 {
		um.ReasoningContent = strings.Join(thinkingParts, "\n")
	}

	if len(toolCalls) > 0 {
		um.ToolCalls = toolCalls
	}

	if toolResultID != "" {
		um.ToolCallID = toolResultID
		um.Content = appendImages(toolResultContent, images)
	} else {
		um.Content = buildPartsContent(textParts, images)
	}

	return nil
}

func buildToolCallFromAnthropicBlock(block *AnthropicContentBlock, idx int) (*vo.UnifiedToolCall, error) {
	args, err := sonic.MarshalString(block.Input)
	if err != nil {
		return nil, ierr.Wrapf(ierr.ErrDTOMarshal, err, "marshal tool_use input for block[%d]", idx)
	}
	id := ""
	if block.ID != nil {
		id = *block.ID
	}
	name := ""
	if block.Name != nil {
		name = *block.Name
	}
	return &vo.UnifiedToolCall{
		ID:        id,
		Name:      name,
		Arguments: args,
	}, nil
}

func extractToolResultContent(block *AnthropicContentBlock) (toolUseID string, content *vo.UnifiedContent) {
	if block.ToolUseID != nil {
		toolUseID = *block.ToolUseID
	}
	if block.Content == nil {
		return
	}
	if block.Content.Text != "" && len(block.Content.Blocks) == 0 {
		content = &vo.UnifiedContent{Text: block.Content.Text}
		return
	}
	if len(block.Content.Blocks) > 0 {
		nestedTexts := lo.FilterMap(block.Content.Blocks, func(nested *AnthropicContentBlock, _ int) (string, bool) {
			if nested.Type != enum.AnthropicContentBlockTypeText || nested.Text == nil {
				return "", false
			}
			return *nested.Text, true
		})
		nestedImages := lo.FilterMap(block.Content.Blocks, func(nested *AnthropicContentBlock, _ int) (*vo.UnifiedContentPart, bool) {
			if nested.Type != enum.AnthropicContentBlockTypeImage {
				return nil, false
			}
			part := anthropicImagePart(nested)
			return part, part != nil
		})
		content = buildPartsContent(nestedTexts, nestedImages)
	}
	return
}

// anthropicImagePart 将 Anthropic image 块转为统一图片部分；source 缺失或类型未知时返回 nil
func anthropicImagePart(block *AnthropicContentBlock) *vo.UnifiedContentPart {
	if block.Source == nil {
		return nil
	}
	var url string
	switch block.Source.Type {
	case enum.ImageSourceTypeBase64:
		url = fmt.Sprintf(constant.DataURLTemplate, lo.FromPtr(block.Source.MediaType), lo.FromPtr(block.Source.Data))
	case enum.ImageSourceTypeURL:
		url = lo.FromPtr(block.Source.URL)
	}
	if url == "" {
		return nil
	}
	return &vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: url}
}

// buildPartsContent 合并文本与图片：无图时保持 Text 形态（保证纯文本消息 checksum 不变），
// 有图时文本合并为首个 text 部分、图片按序在后；两者皆空返回 nil
func buildPartsContent(texts []string, images []*vo.UnifiedContentPart) *vo.UnifiedContent {
	text := strings.Join(texts, "\n")
	if len(images) == 0 {
		if text == "" {
			return nil
		}
		return &vo.UnifiedContent{Text: text}
	}
	parts := make([]*vo.UnifiedContentPart, 0, len(images)+1)
	if text != "" {
		parts = append(parts, &vo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: text})
	}
	return &vo.UnifiedContent{Parts: append(parts, images...)}
}

// appendImages 把顶层图片追加到 tool_result 内容末尾
func appendImages(content *vo.UnifiedContent, images []*vo.UnifiedContentPart) *vo.UnifiedContent {
	if len(images) == 0 {
		return content
	}
	if content == nil {
		return &vo.UnifiedContent{Parts: images}
	}
	if len(content.Parts) == 0 {
		return buildPartsContent([]string{content.Text}, images)
	}
	return &vo.UnifiedContent{Parts: append(slices.Clone(content.Parts), images...)}
}
