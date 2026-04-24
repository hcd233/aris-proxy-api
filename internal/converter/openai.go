// Package converter 协议转换器，实现 OpenAI 和 Anthropic 协议的双向转换
package converter

import (
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

// OpenAIProtocolConverter 将 Anthropic 协议转换为 OpenAI 协议
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type OpenAIProtocolConverter struct{}

// FromAnthropicRequest 将 Anthropic CreateMessage 请求转换为 OpenAI ChatCompletion 请求
//
//	@receiver OpenAIProtocolConverter
//	@param req *dto.AnthropicCreateMessageReq
//	@return *dto.OpenAIChatCompletionReq
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*OpenAIProtocolConverter) FromAnthropicRequest(req *dto.AnthropicCreateMessageReq) (*dto.OpenAIChatCompletionReq, error) {
	openAIReq := &dto.OpenAIChatCompletionReq{
		Model:               req.Model,
		Stream:              req.Stream,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: lo.ToPtr(req.MaxTokens),
	}

	// 转换 system prompt
	messages, err := convertAnthropicSystemToOpenAI(req.System)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "convert anthropic system to openai")
	}

	// 转换消息列表
	for i, msg := range req.Messages {
		openAIMsg, err := convertAnthropicMessageToOpenAI(msg)
		if err != nil {
			return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert anthropic message[%d]", i)
		}
		messages = append(messages, openAIMsg...)
	}
	openAIReq.Messages = messages

	// 转换 stop_sequences
	if len(req.StopSequences) > 0 {
		openAIReq.Stop = &dto.OpenAIStopSequence{Texts: req.StopSequences}
	}

	// 转换工具
	if len(req.Tools) > 0 {
		openAIReq.Tools = convertAnthropicToolsToOpenAI(req.Tools)
	}

	// 转换 tool_choice
	if req.ToolChoice != nil {
		openAIReq.ToolChoice = convertAnthropicToolChoiceToOpenAI(req.ToolChoice)
	}

	return openAIReq, nil
}

// ToAnthropicResponse 将 OpenAI ChatCompletion 响应转换为 Anthropic Message 响应
//
//	@receiver OpenAIProtocolConverter
//	@param completion *dto.OpenAIChatCompletion
//	@return *dto.AnthropicMessage
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*OpenAIProtocolConverter) ToAnthropicResponse(completion *dto.OpenAIChatCompletion) (*dto.AnthropicMessage, error) {
	msg := &dto.AnthropicMessage{
		ID:    completion.ID,
		Type:  "message",
		Role:  enum.RoleAssistant,
		Model: completion.Model,
	}

	// 转换 usage
	if completion.Usage != nil {
		msg.Usage = &dto.AnthropicUsage{
			InputTokens:  completion.Usage.PromptTokens,
			OutputTokens: completion.Usage.CompletionTokens,
		}
	}

	if len(completion.Choices) == 0 {
		msg.Content = []*dto.AnthropicContentBlock{}
		return msg, nil
	}

	choice := completion.Choices[0]

	// 转换 finish_reason -> stop_reason
	msg.StopReason = convertOpenAIFinishReasonToAnthropic(choice.FinishReason)

	// 转换消息内容
	content, err := convertOpenAIMessageToAnthropicContent(choice.Message)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "convert openai message to anthropic content")
	}
	msg.Content = content

	return msg, nil
}

// SSEContentBlockTracker 追踪已发送过 content_block_start 的内容块索引
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type SSEContentBlockTracker struct {
	// startedTextBlocks 已发送 content_block_start（text/thinking）的 choice.Index 集合
	startedTextBlocks map[int]struct{}
	// startedToolBlocks 已发送 content_block_start（tool_use）的 tc.Index 集合
	startedToolBlocks map[int]struct{}
}

// NewSSEContentBlockTracker 创建内容块追踪器
//
//	@return *SSEContentBlockTracker
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func NewSSEContentBlockTracker() *SSEContentBlockTracker {
	return &SSEContentBlockTracker{
		startedTextBlocks: make(map[int]struct{}),
		startedToolBlocks: make(map[int]struct{}),
	}
}

// ToAnthropicSSEResponse 将 OpenAI ChatCompletionChunk 流式块转换为 Anthropic SSE 事件序列
//
//	使用 tracker 追踪 content_block_start 发送状态，确保每个内容块只发送一次 start 事件。
//	调用方需为同一流的所有 chunk 共享同一个 tracker 实例。
//
//	@receiver OpenAIProtocolConverter
//	@param chunk *dto.OpenAIChatCompletionChunk
//	@param isFirst bool
//	@param model string
//	@param tracker *SSEContentBlockTracker
//	@return []dto.AnthropicSSEEvent
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*OpenAIProtocolConverter) ToAnthropicSSEResponse(chunk *dto.OpenAIChatCompletionChunk, isFirst bool, model string, tracker *SSEContentBlockTracker) ([]dto.AnthropicSSEEvent, error) {
	var events []dto.AnthropicSSEEvent

	if isFirst {
		startMsg := &dto.AnthropicSSEMessageStart{
			Message: &dto.AnthropicMessage{
				ID:      chunk.ID,
				Type:    "message",
				Role:    enum.RoleAssistant,
				Model:   model,
				Content: []*dto.AnthropicContentBlock{},
				Usage:   &dto.AnthropicUsage{},
			},
		}
		events = append(events, dto.AnthropicSSEEvent{
			Event: enum.AnthropicSSEEventTypeMessageStart,
			Data:  lo.Must1(sonic.Marshal(startMsg)),
		})
	}

	for _, choice := range chunk.Choices {
		if choice.Delta == nil {
			continue
		}

		// 文本内容增量
		if choice.Delta.Content != "" {
			if _, started := tracker.startedTextBlocks[choice.Index]; !started {
				events = append(events, newContentBlockStartEvent(choice.Index, &dto.AnthropicContentBlock{
					Type: enum.AnthropicContentBlockTypeText,
					Text: "",
				}))
				tracker.startedTextBlocks[choice.Index] = struct{}{}
			}
			events = append(events, newTextDeltaEvent(choice.Index, choice.Delta.Content))
		}

		// 推理内容增量（thinking 与 text 共用同一 index，用负数偏移区分）
		if choice.Delta.ReasoningContent != "" {
			thinkingKey := -(choice.Index + 1)
			if _, started := tracker.startedTextBlocks[thinkingKey]; !started {
				events = append(events, newContentBlockStartEvent(choice.Index, &dto.AnthropicContentBlock{
					Type:     enum.AnthropicContentBlockTypeThinking,
					Thinking: lo.ToPtr(""),
				}))
				tracker.startedTextBlocks[thinkingKey] = struct{}{}
			}
			events = append(events, newThinkingDeltaEvent(choice.Index, choice.Delta.ReasoningContent))
		}

		// 工具调用增量
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Function != nil && tc.ID != "" {
				if _, started := tracker.startedToolBlocks[tc.Index]; !started {
					events = append(events, newContentBlockStartEvent(tc.Index, &dto.AnthropicContentBlock{
						Type:  enum.AnthropicContentBlockTypeToolUse,
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: map[string]any{},
					}))
					tracker.startedToolBlocks[tc.Index] = struct{}{}
				}
			}
			if tc.Function != nil && tc.Function.Arguments != "" {
				events = append(events, newInputJSONDeltaEvent(tc.Index, tc.Function.Arguments))
			}
		}

		// finish_reason
		if choice.FinishReason != "" {
			events = append(events, dto.AnthropicSSEEvent{
				Event: enum.AnthropicSSEEventTypeMessageDelta,
				Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEMessageDelta{
					Delta: dto.AnthropicSSEMessageDeltaPayload{
						StopReason: convertOpenAIFinishReasonToAnthropic(choice.FinishReason),
					},
					Usage: convertChunkUsageToAnthropic(chunk.Usage),
				})),
			})
		}
	}

	return events, nil
}

// ==================== Internal Helpers ====================

func convertAnthropicSystemToOpenAI(system *dto.AnthropicMessageContent) ([]*dto.OpenAIChatCompletionMessageParam, error) {
	if system == nil {
		return nil, nil
	}

	if system.Text != "" {
		return []*dto.OpenAIChatCompletionMessageParam{{
			Role:    enum.RoleSystem,
			Content: &dto.OpenAIMessageContent{Text: system.Text},
		}}, nil
	}

	if len(system.Blocks) > 0 {
		var texts []string
		for _, block := range system.Blocks {
			if block.Type == enum.AnthropicContentBlockTypeText {
				texts = append(texts, block.Text)
			}
		}
		if len(texts) > 0 {
			return []*dto.OpenAIChatCompletionMessageParam{{
				Role:    enum.RoleSystem,
				Content: &dto.OpenAIMessageContent{Text: strings.Join(texts, "\n")},
			}}, nil
		}
	}

	return nil, nil
}

func convertAnthropicMessageToOpenAI(msg *dto.AnthropicMessageParam) ([]*dto.OpenAIChatCompletionMessageParam, error) {
	if msg.Content == nil {
		return []*dto.OpenAIChatCompletionMessageParam{{
			Role: msg.Role,
		}}, nil
	}

	// 纯字符串内容
	if msg.Content.Text != "" && len(msg.Content.Blocks) == 0 {
		return []*dto.OpenAIChatCompletionMessageParam{{
			Role:    msg.Role,
			Content: &dto.OpenAIMessageContent{Text: msg.Content.Text},
		}}, nil
	}

	// 需要拆分的 block 内容
	return convertAnthropicBlocksToOpenAIMessages(msg.Role, msg.Content.Blocks)
}

func convertAnthropicBlocksToOpenAIMessages(role string, blocks []*dto.AnthropicContentBlock) ([]*dto.OpenAIChatCompletionMessageParam, error) {
	var toolResultMessages []*dto.OpenAIChatCompletionMessageParam
	var thinkingParts []string
	var toolCalls []*dto.OpenAIChatCompletionMessageToolCall
	var contentParts []*dto.OpenAIChatCompletionContentPart
	hasMultiModal := false

	for i, block := range blocks {
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			contentParts = append(contentParts, &dto.OpenAIChatCompletionContentPart{
				Type: enum.ContentPartTypeText,
				Text: block.Text,
			})

		case enum.AnthropicContentBlockTypeThinking:
			thinkingParts = append(thinkingParts, lo.FromPtr(block.Thinking))

		case enum.AnthropicContentBlockTypeToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				return nil, ierr.Wrapf(ierr.ErrDTOMarshal, err, "marshal tool_use input for block[%d]", i)
			}
			toolCalls = append(toolCalls, &dto.OpenAIChatCompletionMessageToolCall{
				ID:   block.ID,
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
					Name:      block.Name,
					Arguments: args,
				},
			})

		case enum.AnthropicContentBlockTypeToolResult:
			toolMsg := &dto.OpenAIChatCompletionMessageParam{
				Role:       enum.RoleTool,
				ToolCallID: block.ToolUseID,
			}
			if block.Content != nil {
				toolMsg.Content = &dto.OpenAIMessageContent{Text: extractAnthropicToolResultText(block.Content)}
			}
			toolResultMessages = append(toolResultMessages, toolMsg)

		case enum.AnthropicContentBlockTypeImage:
			if block.Source != nil {
				part := convertAnthropicImageToOpenAIPart(block)
				if part != nil {
					hasMultiModal = true
					contentParts = append(contentParts, part)
				}
			}

		case enum.AnthropicContentBlockTypeRedactedThinking:
			continue

		default:
			continue
		}
	}

	var messages []*dto.OpenAIChatCompletionMessageParam

	// 构建主消息
	if len(contentParts) > 0 || len(thinkingParts) > 0 || len(toolCalls) > 0 {
		mainMsg := &dto.OpenAIChatCompletionMessageParam{
			Role: role,
		}

		if len(contentParts) > 0 {
			if hasMultiModal {
				mainMsg.Content = &dto.OpenAIMessageContent{Parts: contentParts}
			} else {
				// 纯文本，合并为单个字符串
				var texts []string
				for _, p := range contentParts {
					texts = append(texts, p.Text)
				}
				mainMsg.Content = &dto.OpenAIMessageContent{Text: strings.Join(texts, "\n")}
			}
		}

		if len(thinkingParts) > 0 {
			mainMsg.ReasoningContent = strings.Join(thinkingParts, "\n")
		}
		if len(toolCalls) > 0 {
			mainMsg.ToolCalls = toolCalls
		}
		messages = append(messages, mainMsg)
	}

	// tool_result 消息附加在主消息之后
	messages = append(messages, toolResultMessages...)

	if len(messages) == 0 {
		messages = append(messages, &dto.OpenAIChatCompletionMessageParam{
			Role: role,
		})
	}

	return messages, nil
}

func extractAnthropicToolResultText(content *dto.AnthropicToolResultContent) string {
	if content.Text != "" {
		return content.Text
	}
	if len(content.Blocks) > 0 {
		var texts []string
		for _, block := range content.Blocks {
			if block.Type == enum.AnthropicContentBlockTypeText {
				texts = append(texts, block.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func convertAnthropicImageToOpenAIPart(block *dto.AnthropicContentBlock) *dto.OpenAIChatCompletionContentPart {
	if block.Source == nil {
		return nil
	}
	switch block.Source.Type {
	case enum.ImageSourceTypeBase64:
		return &dto.OpenAIChatCompletionContentPart{
			Type: enum.ContentPartTypeImageURL,
			ImageURL: &dto.OpenAIChatCompletionImageURL{
				URL: fmt.Sprintf(constant.DataURLTemplate, block.Source.MediaType, block.Source.Data),
			},
		}
	case enum.ImageSourceTypeURL:
		return &dto.OpenAIChatCompletionContentPart{
			Type: enum.ContentPartTypeImageURL,
			ImageURL: &dto.OpenAIChatCompletionImageURL{
				URL: block.Source.URL,
			},
		}
	}
	return nil
}

func convertAnthropicToolsToOpenAI(tools []*dto.AnthropicTool) []dto.OpenAIChatCompletionTool {
	openAITools := make([]dto.OpenAIChatCompletionTool, 0, len(tools))
	for _, tool := range tools {
		// 仅转换自定义工具（有 input_schema 的），跳过内置工具
		if tool.InputSchema == nil && tool.Name == "" {
			continue
		}

		// 对于无参数工具，OpenAI 要求省略 parameters 字段
		// Anthropic 的 input_schema 可能是 {"type": "object"} 或带有 additionalProperties: false
		// 这种情况下应该将 parameters 设为 nil
		var params *dto.JSONSchemaProperty
		if tool.InputSchema != nil && !isEmptyObjectSchema(tool.InputSchema) {
			params = normalizeOpenAISchema(tool.InputSchema)
		}

		openAITools = append(openAITools, dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
				Strict:      tool.Strict,
			},
		})
	}
	return openAITools
}

// isEmptyObjectSchema 检查 schema 是否表示空对象（无参数）
// OpenAI 对于无参数工具要求省略 parameters 字段，而不是传 {"type": "object"}
func isEmptyObjectSchema(schema *dto.JSONSchemaProperty) bool {
	if schema == nil {
		return true
	}
	// 如果 type 是 object 且没有定义任何 properties，认为是空对象
	if schema.HasType(enum.JSONSchemaObjectType) && len(schema.Properties) == 0 {
		return true
	}
	return false
}

// normalizeOpenAISchema 规范化 JSON Schema，确保符合 OpenAI 要求
// - 清除 $schema（不应出现在 parameters 内部）
// 注意：返回的是浅拷贝，不会修改入参
func normalizeOpenAISchema(schema *dto.JSONSchemaProperty) *dto.JSONSchemaProperty {
	if schema == nil {
		return nil
	}
	// 创建浅拷贝以避免修改入参（防止污染原始 InputSchema）
	copied := *schema
	copied.SchemaURI = ""
	return &copied
}

func convertAnthropicToolChoiceToOpenAI(tc *dto.AnthropicToolChoice) *dto.OpenAIChatCompletionToolChoiceParam {
	switch tc.Type {
	case enum.AnthropicToolChoiceTypeAuto:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceAuto}
	case enum.AnthropicToolChoiceTypeAny:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceRequired}
	case enum.AnthropicToolChoiceTypeNone:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceNone}
	case enum.AnthropicToolChoiceTypeTool:
		return &dto.OpenAIChatCompletionToolChoiceParam{
			Named: &dto.OpenAIChatCompletionToolChoice{
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIToolChoiceFunction{
					Name: tc.Name,
				},
			},
		}
	}
	return nil
}

func convertOpenAIFinishReasonToAnthropic(reason enum.FinishReason) *string {
	switch reason {
	case enum.FinishReasonStop:
		return lo.ToPtr("end_turn")
	case enum.FinishReasonLength:
		return lo.ToPtr("max_tokens")
	case enum.FinishReasonToolCalls:
		return lo.ToPtr("tool_use")
	case enum.FinishReasonContentFilter:
		return lo.ToPtr("end_turn")
	default:
		return lo.ToPtr("end_turn")
	}
}

func convertOpenAIMessageToAnthropicContent(msg *dto.OpenAIChatCompletionMessageParam) ([]*dto.AnthropicContentBlock, error) {
	if msg == nil {
		return []*dto.AnthropicContentBlock{}, nil
	}

	var blocks []*dto.AnthropicContentBlock

	// 推理内容 -> thinking block
	if msg.ReasoningContent != "" {
		blocks = append(blocks, &dto.AnthropicContentBlock{
			Type:     enum.AnthropicContentBlockTypeThinking,
			Thinking: lo.ToPtr(msg.ReasoningContent),
		})
	}

	// 文本内容 -> text block
	if msg.Content != nil {
		if msg.Content.Text != "" {
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type: enum.AnthropicContentBlockTypeText,
				Text: msg.Content.Text,
			})
		} else if len(msg.Content.Parts) > 0 {
			for _, part := range msg.Content.Parts {
				if part.Type == enum.ContentPartTypeText && part.Text != "" {
					blocks = append(blocks, &dto.AnthropicContentBlock{
						Type: enum.AnthropicContentBlockTypeText,
						Text: part.Text,
					})
				}
			}
		}
	}

	// 工具调用 -> tool_use blocks
	for _, tc := range msg.ToolCalls {
		if tc.Function != nil {
			var input map[string]any
			if tc.Function.Arguments != "" {
				if err := sonic.UnmarshalString(tc.Function.Arguments, &input); err != nil {
					return nil, ierr.Wrapf(ierr.ErrDTOUnmarshal, err, "unmarshal tool call arguments for %q", tc.Function.Name)
				}
			}
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type:  enum.AnthropicContentBlockTypeToolUse,
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, &dto.AnthropicContentBlock{
			Type: enum.AnthropicContentBlockTypeText,
			Text: "",
		})
	}

	return blocks, nil
}

func convertChunkUsageToAnthropic(usage *dto.OpenAICompletionUsage) *dto.AnthropicUsage {
	if usage == nil {
		return &dto.AnthropicUsage{}
	}
	return &dto.AnthropicUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
}

func newContentBlockStartEvent(index int, block *dto.AnthropicContentBlock) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockStart,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockStart{
			Index:        index,
			ContentBlock: block,
		})),
	}
}

func newTextDeltaEvent(index int, text string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type: enum.AnthropicDeltaTypeTextDelta,
				Text: text,
			},
		})),
	}
}

func newThinkingDeltaEvent(index int, thinking string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type:     enum.AnthropicDeltaTypeThinkingDelta,
				Thinking: thinking,
			},
		})),
	}
}

func newInputJSONDeltaEvent(index int, partialJSON string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type:        enum.AnthropicDeltaTypeInputJSONDelta,
				PartialJSON: partialJSON,
			},
		})),
	}
}

// GenerateAnthropicMessageID 生成 Anthropic 风格的消息 ID
func GenerateAnthropicMessageID() string {
	return fmt.Sprintf(constant.AnthropicMessageIDTemplate, uuid.New().String())
}
