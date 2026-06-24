package converter

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// FromResponseAPIRequest 将 OpenAI Response API 请求转换为 Anthropic CreateMessage 请求
//
//	@receiver *AnthropicProtocolConverter
//	@param req *dto.OpenAICreateResponseReq
//	@return *dto.AnthropicCreateMessageReq
//	@return error
//	@author centonhuang
//	@update 2026-04-18 18:00:00
func (*AnthropicProtocolConverter) FromResponseAPIRequest(req *dto.OpenAICreateResponseReq) (*dto.AnthropicCreateMessageReq, error) {
	model := lo.FromPtr(req.Model)
	anthropicReq := &dto.AnthropicCreateMessageReq{
		Model: model,
	}

	// 转换 max_tokens
	anthropicReq.MaxTokens = int(lo.FromPtr(req.MaxOutputTokens))

	// 转换 temperature/top_p
	anthropicReq.Temperature = req.Temperature
	anthropicReq.TopP = req.TopP

	// 转换 reasoning → Anthropic thinking
	if req.Reasoning != nil {
		anthropicReq.Thinking = &dto.AnthropicThinkingConfig{}
		switch strings.ToLower(lo.FromPtr(req.Reasoning.Effort)) {
		case enum.ResponseEffortLow:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeLow
		case enum.ResponseEffortMedium:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeMedium
		case enum.ResponseEffortHigh, enum.ResponseEffortXHigh:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeHigh
		case enum.ResponseEffortMinimal:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeMinimal
		case enum.ResponseEffortNone:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeDisabled
		default:
			anthropicReq.Thinking.Type = enum.AnthropicThinkingTypeMedium
		}
	}

	// 转换 text format 配置
	anthropicReq.OutputConfig = convertResponseOutputFormat(req.Text)

	// 构建消息列表
	var messages []*dto.AnthropicMessageParam

	// instructions → system 消息
	if req.Instructions != nil && *req.Instructions != "" {
		messages = append(messages, &dto.AnthropicMessageParam{
			Role:    enum.RoleSystem,
			Content: &dto.AnthropicMessageContent{Text: *req.Instructions},
		})
	}

	// input 处理
	rawMsgs, err := convertResponseInputMessages(req.Input)
	if err != nil {
		return nil, err
	}
	messages = append(messages, rawMsgs...)

	anthropicReq.Messages = messages

	// 转换工具
	if len(req.Tools) > 0 {
		anthropicReq.Tools = convertResponseToolsToAnthropic(req.Tools)
	}

	// 转换 tool_choice
	if req.ToolChoice != nil {
		anthropicReq.ToolChoice = convertResponseToolChoiceToAnthropic(req.ToolChoice)
	}

	return anthropicReq, nil
}

// convertResponseOutputFormat 将 Response API 文本格式配置转换为 Anthropic 输出格式
func convertResponseOutputFormat(text *dto.ResponseTextConfig) *dto.AnthropicOutputConfig {
	if text == nil || text.Format == nil {
		return nil
	}
	switch text.Format.Type {
	case enum.ResponseTextFormatTypeJSONObject, enum.ResponseTextFormatTypeJSONSchema:
		cfg := &dto.AnthropicOutputConfig{}
		if text.Format.Type == enum.ResponseTextFormatTypeJSONSchema && text.Format.Schema != nil {
			cfg.Format = &dto.AnthropicJSONOutputFormat{
				Type:   enum.ResponseFormatTypeJSONSchema,
				Schema: text.Format.Schema,
			}
		}
		return cfg
	}
	return nil
}

// convertResponseInputMessages 将 Response API input 转换为 Anthropic 消息列表
func convertResponseInputMessages(input *dto.ResponseInput) ([]*dto.AnthropicMessageParam, error) {
	if input == nil {
		return nil, nil
	}
	if len(input.Items) > 0 {
		var messages []*dto.AnthropicMessageParam
		for _, item := range input.Items {
			amsg, err := convertResponseInputItemToAnthropic(item)
			if err != nil {
				return nil, ierr.Wrap(ierr.ErrDTOConvert, err, "convert response input item")
			}
			if amsg != nil {
				messages = append(messages, amsg)
			}
		}
		return messages, nil
	}
	if input.Text != "" {
		return []*dto.AnthropicMessageParam{{
			Role:    enum.RoleUser,
			Content: &dto.AnthropicMessageContent{Text: input.Text},
		}}, nil
	}
	return nil, nil
}

// convertResponseInputItemToAnthropic 将 Response API input item 转换为 Anthropic 消息
func convertResponseInputItemToAnthropic(item *dto.ResponseInputItem) (*dto.AnthropicMessageParam, error) {
	if item == nil {
		return nil, nil
	}

	itemType := lo.FromPtr(item.Type)
	switch itemType {
	case "", enum.ResponseInputItemTypeMessage:
		return convertResponseMessageToAnthropic(item)
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall:
		return convertResponseFunctionCallToAnthropic(item), nil
	case enum.ResponseInputItemTypeFunctionCallOutput, enum.ResponseInputItemTypeCustomToolCallOutput:
		return convertResponseFunctionCallOutputToAnthropic(item), nil
	case enum.ResponseInputItemTypeReasoning:
		return convertResponseReasoningToAnthropic(item), nil
	default:
		// 不支持的 item 类型返回 nil
		return nil, nil
	}
}

// convertResponseMessageToAnthropic 将 message 类型 item 转换为 Anthropic 消息
func convertResponseMessageToAnthropic(item *dto.ResponseInputItem) (*dto.AnthropicMessageParam, error) {
	role := resolveResponseAPIRole(lo.FromPtr(item.Role))
	msg := &dto.AnthropicMessageParam{
		Role: role,
	}

	if item.Content == nil {
		msg.Content = &dto.AnthropicMessageContent{Text: ""}
		return msg, nil
	}

	// content 是字符串形态
	if len(item.Content.Parts) == 0 {
		msg.Content = &dto.AnthropicMessageContent{Text: item.Content.Text}
		return msg, nil
	}

	// 数组形态 → content blocks
	msg.Content = &dto.AnthropicMessageContent{Blocks: convertResponseContentPartsToAnthropicBlocks(item.Content.Parts)}
	return msg, nil
}

// resolveResponseAPIRole 将 Response API 角色字符串解析为 Anthropic 角色
func resolveResponseAPIRole(role string) string {
	switch role {
	case enum.RoleAssistant, "":
		return enum.RoleAssistant
	case enum.RoleUser:
		return enum.RoleUser
	case enum.RoleDeveloper:
		// Anthropic 不支持 developer 角色，将其映射为 system
		return enum.RoleSystem
	default:
		return enum.RoleUser
	}
}

// convertResponseContentPartsToAnthropicBlocks 将 Response API content parts 转换为 Anthropic content blocks
func convertResponseContentPartsToAnthropicBlocks(parts []*dto.ResponseInputContent) []*dto.AnthropicContentBlock {
	return lo.FilterMap(parts, func(p *dto.ResponseInputContent, _ int) (*dto.AnthropicContentBlock, bool) {
		if p == nil {
			return nil, false
		}
		switch p.Type {
		case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
			if p.Text == nil {
				return nil, false
			}
			return &dto.AnthropicContentBlock{
				Type: enum.AnthropicContentBlockTypeText,
				Text: p.Text,
			}, true
		case enum.ResponseContentTypeInputImage:
			return &dto.AnthropicContentBlock{
				Type:   enum.AnthropicContentBlockTypeImage,
				Source: buildImageSource(p.ImageURL),
			}, true
		default:
			return nil, false
		}
	})
}

// buildImageSource 根据图片 URL 构建 Anthropic content source
func buildImageSource(imageURL *string) *dto.AnthropicContentSource {
	if imageURL == nil {
		return nil
	}
	if strings.HasPrefix(*imageURL, constant.DataURLPrefix) {
		dataURLParts := strings.SplitN(*imageURL, constant.DataURLBase64Separator, 2)
		if len(dataURLParts) == 2 {
			mt := strings.TrimPrefix(dataURLParts[0], constant.DataURLPrefix)
			d := dataURLParts[1]
			return &dto.AnthropicContentSource{
				Type:      enum.SourceTypeBase64,
				MediaType: &mt,
				Data:      &d,
			}
		}
		return nil
	}
	u := *imageURL
	return &dto.AnthropicContentSource{
		Type: enum.SourceTypeURL,
		URL:  &u,
	}
}

// convertResponseFunctionCallToAnthropic 将 function_call / custom_tool_call 转换为 Anthropic assistant 消息
func convertResponseFunctionCallToAnthropic(item *dto.ResponseInputItem) *dto.AnthropicMessageParam {
	args := lo.FromPtr(item.Arguments)
	if args == "" {
		args = lo.FromPtr(item.Input)
	}
	return &dto.AnthropicMessageParam{
		Role: enum.RoleAssistant,
		Content: &dto.AnthropicMessageContent{
			Blocks: []*dto.AnthropicContentBlock{{
				Type:  enum.AnthropicContentBlockTypeToolUse,
				ID:    item.CallID,
				Name:  item.Name,
				Input: parseJSONToRaw(args),
			}},
		},
	}
}

// convertResponseFunctionCallOutputToAnthropic 将 function_call_output / custom_tool_call_output 转换为 Anthropic 消息
func convertResponseFunctionCallOutputToAnthropic(item *dto.ResponseInputItem) *dto.AnthropicMessageParam {
	msg := &dto.AnthropicMessageParam{
		Role: enum.RoleUser,
	}

	if item.Output == nil || (item.Output.Text == "" && item.Output.FunctionOutput == nil) {
		msg.Content = &dto.AnthropicMessageContent{Text: ""}
		return msg
	}

	text := extractResponseFunctionCallOutputText(item.Output)
	msg.Content = &dto.AnthropicMessageContent{
		Blocks: []*dto.AnthropicContentBlock{{
			Type:      enum.AnthropicContentBlockTypeToolResult,
			ToolUseID: item.CallID,
			Content:   &dto.AnthropicToolResultContent{Text: text},
		}},
	}
	return msg
}

// extractResponseFunctionCallOutputText 从 function call output 中提取文本内容
func extractResponseFunctionCallOutputText(output *dto.ResponseInputItemOutput) string {
	if output.Text != "" {
		return output.Text
	}
	if output.FunctionOutput == nil {
		return ""
	}
	if output.FunctionOutput.Text != "" {
		return output.FunctionOutput.Text
	}
	if len(output.FunctionOutput.Parts) == 0 {
		return ""
	}
	parts := lo.FilterMap(output.FunctionOutput.Parts, func(p *dto.ResponseInputContent, _ int) (string, bool) {
		if p != nil && (p.Type == enum.ResponseContentTypeInputText || p.Type == enum.ResponseContentTypeOutputText) && p.Text != nil {
			return lo.FromPtr(p.Text), true
		}
		return "", false
	})
	return strings.Join(parts, "\n")
}

// convertResponseReasoningToAnthropic 将 reasoning item 转换为 Anthropic thinking block
func convertResponseReasoningToAnthropic(item *dto.ResponseInputItem) *dto.AnthropicMessageParam {
	var thinkingParts []string
	thinkingParts = append(thinkingParts, lo.FilterMap(lo.FromPtr(item.Summary), func(s *dto.ResponseReasoningSummary, _ int) (string, bool) {
		if s != nil && s.Text != "" {
			return s.Text, true
		}
		return "", false
	})...)
	thinkingParts = append(thinkingParts, lo.FilterMap(item.ReasoningContent, func(c *dto.ResponseReasoningTextContent, _ int) (string, bool) {
		if c != nil && c.Text != "" {
			return c.Text, true
		}
		return "", false
	})...)
	thinking := strings.Join(thinkingParts, "\n")
	if thinking == "" {
		return nil
	}
	return &dto.AnthropicMessageParam{
		Role: enum.RoleAssistant,
		Content: &dto.AnthropicMessageContent{
			Blocks: []*dto.AnthropicContentBlock{{
				Type:     enum.AnthropicContentBlockTypeThinking,
				Thinking: lo.ToPtr(thinking),
			}},
		},
	}
}

// convertResponseToolsToAnthropic 将 Response API tools 转换为 Anthropic tools
func convertResponseToolsToAnthropic(tools []*dto.ResponseTool) []*dto.AnthropicTool {
	return lo.FilterMap(tools, func(tool *dto.ResponseTool, _ int) (*dto.AnthropicTool, bool) {
		if tool == nil {
			return nil, false
		}
		switch {
		case tool.Function != nil:
			name := tool.Function.Name
			return &dto.AnthropicTool{
				Name:        &name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
				Strict:      &tool.Function.Strict,
			}, true
		case tool.Custom != nil:
			name := tool.Custom.Name
			return &dto.AnthropicTool{
				Name:        &name,
				Description: tool.Custom.Description,
			}, true
		}
		return nil, false
	})
}

// convertResponseToolChoiceToAnthropic 将 Response API tool_choice 转换为 Anthropic tool_choice
func convertResponseToolChoiceToAnthropic(tc *dto.ResponseToolChoiceParam) *dto.AnthropicToolChoice {
	if tc == nil {
		return nil
	}
	switch tc.Mode {
	case enum.ResponseToolChoiceOptionNone:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeNone}
	case enum.ResponseToolChoiceOptionAuto:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeAuto}
	case enum.ResponseToolChoiceOptionRequired:
		return &dto.AnthropicToolChoice{Type: enum.AnthropicToolChoiceTypeAny}
	}
	if tc.Object != nil && tc.Object.Type == enum.ResponseToolChoiceTypeFunction {
		return &dto.AnthropicToolChoice{
			Type: enum.AnthropicToolChoiceTypeTool,
			Name: tc.Object.Name,
		}
	}
	return nil
}

// parseJSONToRaw 解析 JSON 字符串为原始字节，解析失败时保留原始内容
func parseJSONToRaw(jsonStr string) sonic.NoCopyRawMessage {
	if jsonStr == "" {
		return nil
	}
	var result sonic.NoCopyRawMessage
	if err := sonic.UnmarshalString(jsonStr, &result); err != nil {
		// 解析失败时保留原始 JSON 字符串，交由上游尝试解释
		return sonic.NoCopyRawMessage(jsonStr)
	}
	return result
}
