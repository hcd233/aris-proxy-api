// Package converter 提供 OpenAI/Anthropic 协议间的 DTO 转换。
//
// 注意：本包在操作 LLM tool/function 调用参数 schema 时使用了 map[string]any，
// 因为 schema 结构是动态的（由用户/模型定义），无法用静态类型表示。这是一个有意的
// 例外，已记录在案。
package converter

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// AnthropicProtocolConverter 将 OpenAI 协议转换为 Anthropic 协议
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type AnthropicProtocolConverter struct{}

// FromOpenAIRequest 将 OpenAI ChatCompletion 请求转换为 Anthropic CreateMessage 请求
//
//	@receiver AnthropicProtocolConverter
//	@param req *dto.OpenAIChatCompletionReq
//	@return *dto.AnthropicCreateMessageReq
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*AnthropicProtocolConverter) FromOpenAIRequest(req *dto.OpenAIChatCompletionReq) (*dto.AnthropicCreateMessageReq, error) {
	anthropicReq := &dto.AnthropicCreateMessageReq{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	// 转换 max_tokens
	anthropicReq.MaxTokens = resolveMaxTokens(req)

	// 转换 stop sequences
	if req.Stop != nil {
		anthropicReq.StopSequences = resolveStopSequences(req.Stop)
	}

	// 提取 system 消息并转换其余消息
	system, messages, err := extractOpenAISystemAndMessages(req.Messages)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "extract openai system and messages")
	}
	anthropicReq.System = system
	anthropicReq.Messages = messages

	// 转换工具
	if len(req.Tools) > 0 {
		anthropicReq.Tools = convertOpenAIToolsToAnthropic(req.Tools)
	}

	// 转换 tool_choice
	if req.ToolChoice != nil {
		anthropicReq.ToolChoice = convertOpenAIToolChoiceToAnthropic(req.ToolChoice)
	}

	return anthropicReq, nil
}

// ToOpenAIResponse 将 Anthropic Message 响应转换为 OpenAI ChatCompletion 响应
//
//	@receiver AnthropicProtocolConverter
//	@param msg *dto.AnthropicMessage
//	@return *dto.OpenAIChatCompletion
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*AnthropicProtocolConverter) ToOpenAIResponse(msg *dto.AnthropicMessage) (*dto.OpenAIChatCompletion, error) {
	completion := &dto.OpenAIChatCompletion{
		ID:      msg.ID,
		Object:  enum.CompletionObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   msg.Model,
	}

	// 转换 usage
	if msg.Usage != nil {
		completion.Usage = &dto.OpenAICompletionUsage{
			PromptTokens:     msg.Usage.InputTokens,
			CompletionTokens: msg.Usage.OutputTokens,
			TotalTokens:      msg.Usage.InputTokens + msg.Usage.OutputTokens,
		}
	}

	// 转换消息内容和 finish_reason
	message, err := convertAnthropicContentToOpenAIMessage(msg.Content)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "convert anthropic content to openai message")
	}

	choice := &dto.OpenAIChatCompletionChoice{
		Index:        0,
		Message:      message,
		FinishReason: convertAnthropicStopReasonToOpenAI(msg.StopReason),
	}
	completion.Choices = []*dto.OpenAIChatCompletionChoice{choice}

	return completion, nil
}

// ToOpenAISSEResponse 将 Anthropic SSE 事件转换为 OpenAI ChatCompletionChunk 流式块
//
//	@receiver AnthropicProtocolConverter
//	@param event dto.AnthropicSSEEvent
//	@param model string
//	@param chunkID string
//	@return []*dto.OpenAIChatCompletionChunk
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (*AnthropicProtocolConverter) ToOpenAISSEResponse(event dto.AnthropicSSEEvent, model, chunkID string) ([]*dto.OpenAIChatCompletionChunk, error) {
	switch event.Event {
	case enum.AnthropicSSEEventTypeContentBlockDelta:
		return convertContentBlockDeltaToChunks(event.Data, model, chunkID)

	case enum.AnthropicSSEEventTypeMessageDelta:
		return convertMessageDeltaToChunks(event.Data, model, chunkID)

	case enum.AnthropicSSEEventTypeContentBlockStart:
		return convertContentBlockStartToChunks(event.Data, model, chunkID)

	case enum.AnthropicSSEEventTypeMessageStart,
		enum.AnthropicSSEEventTypeContentBlockStop,
		enum.AnthropicSSEEventTypeMessageStop,
		enum.AnthropicSSEEventTypePing:
		return nil, nil

	default:
		return nil, nil
	}
}

// ==================== Internal Helpers ====================

func resolveMaxTokens(req *dto.OpenAIChatCompletionReq) int {
	if req.MaxCompletionTokens != nil {
		return *req.MaxCompletionTokens
	}
	if req.MaxTokens != nil {
		return *req.MaxTokens
	}
	return 0
}

func resolveStopSequences(stop *dto.OpenAIStopSequence) []string {
	if len(stop.Texts) > 0 {
		return stop.Texts
	}
	if stop.Text != "" {
		return []string{stop.Text}
	}
	return nil
}

func extractOpenAISystemAndMessages(messages []*dto.OpenAIChatCompletionMessageParam) (*dto.AnthropicMessageContent, []*dto.AnthropicMessageParam, error) {
	var systemTexts []string
	var anthropicMessages []*dto.AnthropicMessageParam

	for i, msg := range messages {
		switch msg.Role {
		case enum.RoleSystem, enum.RoleDeveloper:
			if msg.Content != nil {
				systemTexts = append(systemTexts, resolveOpenAIContentText(msg.Content))
			}

		case enum.RoleUser:
			am, err := convertOpenAIUserMessageToAnthropic(msg)
			if err != nil {
				return nil, nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert openai user message[%d]", i)
			}
			anthropicMessages = append(anthropicMessages, am)

		case enum.RoleAssistant:
			am, err := convertOpenAIAssistantMessageToAnthropic(msg)
			if err != nil {
				return nil, nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert openai assistant message[%d]", i)
			}
			anthropicMessages = append(anthropicMessages, am)

		case enum.RoleTool:
			am := convertOpenAIToolMessageToAnthropic(msg)
			anthropicMessages = mergeToolResultIntoLastUser(anthropicMessages, am)

		default:
			continue
		}
	}

	var system *dto.AnthropicMessageContent
	if len(systemTexts) > 0 {
		system = &dto.AnthropicMessageContent{Text: strings.Join(systemTexts, "\n")}
	}

	return system, anthropicMessages, nil
}

func resolveOpenAIContentText(content *dto.OpenAIMessageContent) string {
	if content.Text != "" {
		return content.Text
	}
	if len(content.Parts) > 0 {
		var texts []string
		for _, part := range content.Parts {
			if part.Type == enum.ContentPartTypeText {
				texts = append(texts, lo.FromPtr(part.Text))
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func convertOpenAIUserMessageToAnthropic(msg *dto.OpenAIChatCompletionMessageParam) (*dto.AnthropicMessageParam, error) {
	am := &dto.AnthropicMessageParam{
		Role: enum.RoleUser,
	}

	if msg.Content == nil {
		am.Content = &dto.AnthropicMessageContent{Text: ""}
		return am, nil
	}

	// 纯文本
	if msg.Content.Text != "" && len(msg.Content.Parts) == 0 {
		am.Content = &dto.AnthropicMessageContent{Text: msg.Content.Text}
		return am, nil
	}

	// 多部分内容
	if len(msg.Content.Parts) > 0 {
		blocks, err := convertOpenAIPartsToAnthropicBlocks(msg.Content.Parts)
		if err != nil {
			return nil, err
		}
		am.Content = &dto.AnthropicMessageContent{Blocks: blocks}
		return am, nil
	}

	am.Content = &dto.AnthropicMessageContent{Text: ""}
	return am, nil
}

func convertOpenAIPartsToAnthropicBlocks(parts []*dto.OpenAIChatCompletionContentPart) ([]*dto.AnthropicContentBlock, error) {
	var blocks []*dto.AnthropicContentBlock
	for _, part := range parts {
		switch part.Type {
		case enum.ContentPartTypeText:
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type: enum.AnthropicContentBlockTypeText,
				Text: part.Text,
			})
		case enum.ContentPartTypeImageURL:
			if part.ImageURL != nil {
				block := convertOpenAIImageURLToAnthropicBlock(part.ImageURL)
				blocks = append(blocks, block)
			}
		default:
			continue
		}
	}
	return blocks, nil
}

func convertOpenAIImageURLToAnthropicBlock(img *dto.OpenAIChatCompletionImageURL) *dto.AnthropicContentBlock {
	block := &dto.AnthropicContentBlock{
		Type: enum.AnthropicContentBlockTypeImage,
	}

	// 检查是否是 data URI
	if strings.HasPrefix(img.URL, constant.DataURLPrefix) {
		parts := strings.SplitN(img.URL, constant.DataURLBase64Separator, 2)
		if len(parts) == 2 {
			mediaType := strings.TrimPrefix(parts[0], constant.DataURLPrefix)
			mt := mediaType
			d := parts[1]
			block.Source = &dto.AnthropicContentSource{
				Type:      enum.SourceTypeBase64,
				MediaType: &mt,
				Data:      &d,
			}
			return block
		}
	}

	// URL 形式
	u := img.URL
	block.Source = &dto.AnthropicContentSource{
		Type: enum.SourceTypeURL,
		URL:  &u,
	}
	return block
}

func convertOpenAIAssistantMessageToAnthropic(msg *dto.OpenAIChatCompletionMessageParam) (*dto.AnthropicMessageParam, error) {
	am := &dto.AnthropicMessageParam{
		Role: enum.RoleAssistant,
	}

	var blocks []*dto.AnthropicContentBlock

	// 推理内容 -> thinking block
	if lo.FromPtr(msg.ReasoningContent) != "" {
		blocks = append(blocks, &dto.AnthropicContentBlock{
			Type:     enum.AnthropicContentBlockTypeThinking,
			Thinking: msg.ReasoningContent,
		})
	}

	// 文本内容 -> text block
	if msg.Content != nil {
		text := resolveOpenAIContentText(msg.Content)
		if text != "" {
			t := text
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type: enum.AnthropicContentBlockTypeText,
				Text: &t,
			})
		}
	}

	// 工具调用 -> tool_use blocks
	for i, tc := range msg.ToolCalls {
		if tc.Function != nil {
			var input map[string]any
			if tc.Function.Arguments != "" {
				if err := sonic.UnmarshalString(tc.Function.Arguments, &input); err != nil {
					return nil, ierr.Wrapf(ierr.ErrDTOUnmarshal, err, "unmarshal tool call arguments[%d]", i)
				}
			}
			name := tc.Function.Name
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type:  enum.AnthropicContentBlockTypeToolUse,
				ID:    tc.ID,
				Name:  &name,
				Input: input,
			})
		}
	}

	if len(blocks) > 0 {
		am.Content = &dto.AnthropicMessageContent{Blocks: blocks}
	} else {
		am.Content = &dto.AnthropicMessageContent{Text: ""}
	}

	return am, nil
}

func convertOpenAIToolMessageToAnthropic(msg *dto.OpenAIChatCompletionMessageParam) *dto.AnthropicContentBlock {
	block := &dto.AnthropicContentBlock{
		Type:      enum.AnthropicContentBlockTypeToolResult,
		ToolUseID: msg.ToolCallID,
	}
	if msg.Content != nil {
		text := resolveOpenAIContentText(msg.Content)
		block.Content = &dto.AnthropicToolResultContent{Text: text}
	}
	return block
}

// mergeToolResultIntoLastUser 将 tool_result 合并到最后一个 user 消息中
// Anthropic 要求 tool_result 必须在 user 角色的消息中
func mergeToolResultIntoLastUser(messages []*dto.AnthropicMessageParam, toolResult *dto.AnthropicContentBlock) []*dto.AnthropicMessageParam {
	// 检查最后一条消息是否是 user 消息（用于合并多个 tool results）
	if len(messages) > 0 && messages[len(messages)-1].Role == enum.RoleUser {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Content != nil && len(lastMsg.Content.Blocks) > 0 {
			lastMsg.Content.Blocks = append(lastMsg.Content.Blocks, toolResult)
			return messages
		}
	}

	// 创建新的 user 消息包含 tool_result
	return append(messages, &dto.AnthropicMessageParam{
		Role: enum.RoleUser,
		Content: &dto.AnthropicMessageContent{
			Blocks: []*dto.AnthropicContentBlock{toolResult},
		},
	})
}

func convertOpenAIToolsToAnthropic(tools []dto.OpenAIChatCompletionTool) []*dto.AnthropicTool {
	anthropicTools := make([]*dto.AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Function != nil {
			name := tool.Function.Name
			anthropicTools = append(anthropicTools, &dto.AnthropicTool{
				Name:        &name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
				Strict:      tool.Function.Strict,
			})
		}
	}
	return anthropicTools
}

func convertOpenAIToolChoiceToAnthropic(tc *dto.OpenAIChatCompletionToolChoiceParam) *dto.AnthropicToolChoice {
	if tc.Named != nil && tc.Named.Function != nil {
		name := tc.Named.Function.Name
		return &dto.AnthropicToolChoice{
			Type: enum.AnthropicToolChoiceTypeTool,
			Name: &name,
		}
	}
	switch tc.Mode {
	case enum.ToolChoiceAuto:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeAuto}
	case enum.ToolChoiceRequired:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeAny}
	case enum.ToolChoiceNone:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeNone}
	}
	return nil
}

func convertAnthropicStopReasonToOpenAI(stopReason *string) enum.FinishReason {
	if stopReason == nil {
		return enum.FinishReasonStop
	}
	switch *stopReason {
	case enum.AnthropicStopReasonEndTurn, enum.AnthropicStopReasonStop:
		return enum.FinishReasonStop
	case enum.AnthropicStopReasonMaxTokens:
		return enum.FinishReasonLength
	case enum.AnthropicStopReasonToolUse:
		return enum.FinishReasonToolCalls
	default:
		return enum.FinishReasonStop
	}
}

func convertAnthropicContentToOpenAIMessage(blocks []*dto.AnthropicContentBlock) (*dto.OpenAIChatCompletionMessageParam, error) {
	msg := &dto.OpenAIChatCompletionMessageParam{
		Role: enum.RoleAssistant,
	}

	var textParts []string
	var thinkingParts []string
	var toolCalls []*dto.OpenAIChatCompletionMessageToolCall

	for i, block := range blocks {
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			textParts = append(textParts, lo.FromPtr(block.Text))

		case enum.AnthropicContentBlockTypeThinking:
			thinkingParts = append(thinkingParts, lo.FromPtr(block.Thinking))

		case enum.AnthropicContentBlockTypeToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				return nil, ierr.Wrapf(ierr.ErrDTOMarshal, err, "marshal tool_use input for block[%d]", i)
			}
			name := lo.FromPtr(block.Name)
			toolCalls = append(toolCalls, &dto.OpenAIChatCompletionMessageToolCall{
				ID:   block.ID,
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
					Name:      name,
					Arguments: args,
				},
			})

		case enum.AnthropicContentBlockTypeRedactedThinking:
			continue

		default:
			continue
		}
	}

	if joined := strings.Join(textParts, "\n"); joined != "" {
		msg.Content = &dto.OpenAIMessageContent{Text: joined}
	}
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "\n")
		msg.ReasoningContent = &thinking
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return msg, nil
}

// GenerateOpenAIChunkID 生成 OpenAI 风格的 chunk ID
func GenerateOpenAIChunkID() string {
	return fmt.Sprintf(constant.OpenAIChunkIDTemplate, uuid.New().String())
}
