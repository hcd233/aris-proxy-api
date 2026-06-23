package converter

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ResponseProtocolConverter 以 OpenAI ChatCompletion 作为 OpenAI Response API 的兼容基座。
type ResponseProtocolConverter struct {
	toolTypeMap map[string]string // Chat function name → original Response tool type
}

func (c *ResponseProtocolConverter) FromResponseRequest(req *dto.OpenAICreateResponseReq) (*dto.OpenAIChatCompletionReq, error) {
	chatReq := &dto.OpenAIChatCompletionReq{
		Model:             lo.FromPtr(req.Model),
		Stream:            req.Stream,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		TopLogprobs:       req.TopLogprobs,
		Metadata:          req.Metadata,
		ParallelToolCalls: req.ParallelToolCalls,
		PromptCacheKey:    req.PromptCacheKey,
		SafetyIdentifier:  req.SafetyIdentifier,
		Store:             req.Store,
		User:              req.User,
	}
	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = lo.ToPtr(int(*req.MaxOutputTokens))
	}
	chatReq.PromptCacheRetention = lo.FromPtr(req.PromptCacheRetention)
	chatReq.ServiceTier = lo.FromPtr(req.ServiceTier)
	if req.Reasoning != nil && req.Reasoning.Effort != nil {
		chatReq.ReasoningEffort = *req.Reasoning.Effort
	}
	if req.Text != nil {
		if req.Text.Verbosity != nil {
			chatReq.Verbosity = *req.Text.Verbosity
		}
		if req.Text.Format != nil {
			chatReq.ResponseFormat = responseTextFormatToChat(req.Text.Format)
		}
	}
	if req.StreamOptions != nil {
		chatReq.StreamOptions = &dto.OpenAIChatCompletionStreamOptions{
			IncludeObfuscation: req.StreamOptions.IncludeObfuscation,
		}
	}
	if len(req.Tools) > 0 {
		chatReq.Tools = responseToolsToChat(req.Tools)
		c.toolTypeMap = BuildToolTypeMap(req.Tools)
	}
	if req.ToolChoice != nil {
		chatReq.ToolChoice = responseToolChoiceToChat(req.ToolChoice)
	}

	messages := responseInputToChatMessages(req)
	chatReq.Messages = messages
	return chatReq, nil
}

func (c *ResponseProtocolConverter) ToResponseResponse(completion *dto.OpenAIChatCompletion) (*dto.OpenAICreateResponseRsp, error) {
	if completion == nil {
		return nil, ierr.New(ierr.ErrDTOConvert, "openai chat completion is nil")
	}
	rsp := &dto.OpenAICreateResponseRsp{
		ID:        completion.ID,
		Object:    enum.CompletionObjectResponse,
		CreatedAt: completion.Created,
		Status:    enum.ResponseStatusCompleted,
		Model:     completion.Model,
	}
	if rsp.ID == "" {
		rsp.ID = "resp_" + uuid.New().String()
	}
	if rsp.CreatedAt == 0 {
		rsp.CreatedAt = time.Now().Unix()
	}
	if completion.Usage != nil {
		rsp.Usage = &dto.ResponseUsage{
			InputTokens:  completion.Usage.PromptTokens,
			OutputTokens: completion.Usage.CompletionTokens,
			TotalTokens:  completion.Usage.TotalTokens,
		}
		if rsp.Usage.TotalTokens == 0 {
			rsp.Usage.TotalTokens = rsp.Usage.InputTokens + rsp.Usage.OutputTokens
		}
		if completion.Usage.PromptTokensDetails != nil && completion.Usage.PromptTokensDetails.CachedTokens != nil {
			rsp.Usage.InputTokensDetails = &dto.ResponseInputTokensDetail{
				CachedTokens: *completion.Usage.PromptTokensDetails.CachedTokens,
			}
		}
		if completion.Usage.CompletionTokensDetails != nil && completion.Usage.CompletionTokensDetails.ReasoningTokens != nil {
			rsp.Usage.OutputTokensDetails = &dto.ResponseOutputTokensDetail{
				ReasoningTokens: *completion.Usage.CompletionTokensDetails.ReasoningTokens,
			}
		}
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return rsp, nil
	}
	items := chatMessageToResponseOutputs(completion.Choices[0].Message, c.toolTypeMap)
	rsp.Output = items
	return rsp, nil
}

func responseInputToChatMessages(req *dto.OpenAICreateResponseReq) []*dto.OpenAIChatCompletionMessageParam {
	var messages []*dto.OpenAIChatCompletionMessageParam
	if req.Instructions != nil && *req.Instructions != "" {
		messages = append(messages, &dto.OpenAIChatCompletionMessageParam{
			Role:    enum.RoleSystem,
			Content: &dto.OpenAIMessageContent{Text: *req.Instructions},
		})
	}
	if req.Input == nil {
		return messages
	}
	if len(req.Input.Items) == 0 {
		if req.Input.Text != "" {
			messages = append(messages, &dto.OpenAIChatCompletionMessageParam{
				Role:    enum.RoleUser,
				Content: &dto.OpenAIMessageContent{Text: req.Input.Text},
			})
		}
		return messages
	}
	chatMsgs := lo.Flatten(lo.Map(req.Input.Items, func(item *dto.ResponseInputItem, _ int) []*dto.OpenAIChatCompletionMessageParam {
		return responseInputItemToChatMessages(item)
	}))
	messages = append(messages, chatMsgs...)
	return messages
}

func responseInputItemToChatMessages(item *dto.ResponseInputItem) []*dto.OpenAIChatCompletionMessageParam {
	if item == nil {
		return nil
	}
	switch lo.FromPtr(item.Type) {
	case "", enum.ResponseInputItemTypeMessage:
		msg := &dto.OpenAIChatCompletionMessageParam{Role: responseRoleToChat(lo.FromPtr(item.Role))}
		content := responseMessageContentToChat(item.Content)
		msg.Content = content
		return []*dto.OpenAIChatCompletionMessageParam{msg}
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallToChat(item)}
	case enum.ResponseInputItemTypeFunctionCallOutput, enum.ResponseInputItemTypeCustomToolCallOutput:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallOutputToChat(item)}
	case enum.ResponseInputItemTypeReasoning:
		if text := responseReasoningText(item); text != "" {
			return []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleAssistant, ReasoningContent: lo.ToPtr(text)}}
		}
	}
	return nil
}

func responseRoleToChat(role string) enum.Role {
	switch role {
	case enum.RoleAssistant:
		return enum.RoleAssistant
	case enum.RoleSystem, enum.RoleDeveloper:
		return enum.RoleSystem
	case enum.RoleUser, "":
		return enum.RoleUser
	default:
		return enum.RoleUser
	}
}

func responseMessageContentToChat(content *dto.ResponseInputMessageContent) *dto.OpenAIMessageContent {
	if content == nil {
		return nil
	}
	if len(content.Parts) == 0 {
		return &dto.OpenAIMessageContent{Text: content.Text}
	}
	parts := make([]*dto.OpenAIChatCompletionContentPart, 0, len(content.Parts))
	var texts []string
	multimodal := false
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		switch part.Type {
		case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
			texts = append(texts, lo.FromPtr(part.Text))
			parts = append(parts, &dto.OpenAIChatCompletionContentPart{Type: enum.ContentPartTypeText, Text: part.Text})
		case enum.ResponseContentTypeInputImage:
			multimodal = true
			parts = append(parts, &dto.OpenAIChatCompletionContentPart{
				Type: enum.ContentPartTypeImageURL,
				ImageURL: &dto.OpenAIChatCompletionImageURL{
					URL:    lo.FromPtr(part.ImageURL),
					Detail: lo.FromPtr(part.Detail),
				},
			})
		case enum.ResponseContentTypeRefusal:
			texts = append(texts, lo.FromPtr(part.Refusal))
		default:
			continue
		}
	}
	if multimodal {
		return &dto.OpenAIMessageContent{Parts: parts}
	}
	return &dto.OpenAIMessageContent{Text: strings.Join(texts, "\n")}
}

func responseFunctionCallToChat(item *dto.ResponseInputItem) *dto.OpenAIChatCompletionMessageParam {
	args := lo.FromPtr(item.Arguments)
	if args == "" {
		args = lo.FromPtr(item.Input)
	}
	callID := lo.FromPtr(item.CallID)
	if callID == "" {
		callID = "call_" + uuid.New().String()
	}
	return &dto.OpenAIChatCompletionMessageParam{
		Role: enum.RoleAssistant,
		ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
			ID:   lo.ToPtr(callID),
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
				Name:      lo.FromPtr(item.Name),
				Arguments: args,
			},
		}},
	}
}

func responseFunctionCallOutputToChat(item *dto.ResponseInputItem) *dto.OpenAIChatCompletionMessageParam {
	return &dto.OpenAIChatCompletionMessageParam{
		Role:       enum.RoleTool,
		ToolCallID: item.CallID,
		Content:    &dto.OpenAIMessageContent{Text: responseOutputText(item.Output)},
	}
}

func responseOutputText(output *dto.ResponseInputItemOutput) string {
	if output == nil {
		return ""
	}
	if output.Text != "" {
		return output.Text
	}
	if output.FunctionOutput == nil {
		return ""
	}
	if output.FunctionOutput.Text != "" {
		return output.FunctionOutput.Text
	}
	texts := lo.FilterMap(output.FunctionOutput.Parts, func(part *dto.ResponseInputContent, _ int) (string, bool) {
		if part == nil {
			return "", false
		}
		isTextType := part.Type == enum.ResponseContentTypeInputText || part.Type == enum.ResponseContentTypeOutputText
		return lo.FromPtr(part.Text), isTextType
	})
	return strings.Join(texts, "\n")
}

func responseReasoningText(item *dto.ResponseInputItem) string {
	summaryTexts := lo.FilterMap(item.Summary, func(s *dto.ResponseReasoningSummary, _ int) (string, bool) {
		if s == nil || s.Text == "" {
			return "", false
		}
		return s.Text, true
	})
	reasoningTexts := lo.FilterMap(item.ReasoningContent, func(c *dto.ResponseReasoningTextContent, _ int) (string, bool) {
		if c == nil || c.Text == "" {
			return "", false
		}
		return c.Text, true
	})
	return strings.Join(append(summaryTexts, reasoningTexts...), "\n")
}

func responseTextFormatToChat(format *dto.ResponseTextFormat) *dto.OpenAIResponseFormat {
	if format == nil {
		return nil
	}
	rspFormat := &dto.OpenAIResponseFormat{Type: format.Type}
	if format.Schema != nil {
		rspFormat.JSONSchema = &dto.OpenAIJSONSchemaFormat{
			Name:        lo.FromPtr(format.Name),
			Description: format.Description,
			Schema:      format.Schema,
			Strict:      format.Strict,
		}
	}
	return rspFormat
}

func responseToolsToChat(tools []*dto.ResponseTool) []dto.OpenAIChatCompletionTool {
	return lo.FilterMap(tools, func(tool *dto.ResponseTool, _ int) (dto.OpenAIChatCompletionTool, bool) {
		if tool == nil {
			return dto.OpenAIChatCompletionTool{}, false
		}
		return convertResponseToolToChat(tool)
	})
}

func convertResponseToolToChat(tool *dto.ResponseTool) (dto.OpenAIChatCompletionTool, bool) {
	switch {
	case tool.Function != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
				Strict:      &tool.Function.Strict,
			},
		}, true
	case tool.Custom != nil:
		chatTool := dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeCustom,
			Custom: &dto.OpenAICustomToolDefinition{
				Name:        tool.Custom.Name,
				Description: tool.Custom.Description,
			},
		}
		if tool.Custom.Format != nil {
			chatTool.Custom.Format = &dto.OpenAICustomToolFormat{
				Type: tool.Custom.Format.Type,
			}
		}
		return chatTool, true
	case tool.FileSearch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.FileSearch.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescFileSearch),
			},
		}, true
	case tool.WebSearch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.WebSearch.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescWebSearch),
			},
		}, true
	case tool.WebSearchPreview != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.WebSearchPreview.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescWebSearchPreview),
			},
		}, true
	case tool.Computer != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Computer.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescComputer),
			},
		}, true
	case tool.ComputerUsePreview != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.ComputerUsePreview.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescComputerPreview),
			},
		}, true
	case tool.Mcp != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Mcp.Type,
				Description: lo.ToPtr(fmt.Sprintf(constant.ChatCompletionConvertToolDescMCPTemplate, tool.Mcp.ServerLabel)),
			},
		}, true
	case tool.CodeInterpreter != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.CodeInterpreter.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescCodeInterpreter),
			},
		}, true
	case tool.ImageGeneration != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.ImageGeneration.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescImageGeneration),
			},
		}, true
	case tool.LocalShell != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.LocalShell.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescLocalShell),
			},
		}, true
	case tool.Shell != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Shell.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescShell),
			},
		}, true
	case tool.Namespace != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Namespace.Name,
				Description: lo.ToPtr(tool.Namespace.Description),
			},
		}, true
	case tool.ToolSearch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.ToolSearch.Type,
				Description: tool.ToolSearch.Description,
			},
		}, true
	case tool.ApplyPatch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.ApplyPatch.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescApplyPatch),
			},
		}, true
	}
	return dto.OpenAIChatCompletionTool{}, false
}

func responseToolChoiceToChat(tc *dto.ResponseToolChoiceParam) *dto.OpenAIChatCompletionToolChoiceParam {
	if tc == nil {
		return nil
	}
	switch tc.Mode {
	case enum.ResponseToolChoiceOptionNone:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceNone}
	case enum.ResponseToolChoiceOptionAuto:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceAuto}
	case enum.ResponseToolChoiceOptionRequired:
		return &dto.OpenAIChatCompletionToolChoiceParam{Mode: enum.ToolChoiceRequired}
	}
	if tc.Object != nil && tc.Object.Type == enum.ResponseToolChoiceTypeFunction {
		return &dto.OpenAIChatCompletionToolChoiceParam{
			Named: &dto.OpenAIChatCompletionToolChoice{
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIToolChoiceFunction{
					Name: lo.FromPtr(tc.Object.Name),
				},
			},
		}
	}
	return nil
}

func chatMessageToResponseOutputs(msg *dto.OpenAIChatCompletionMessageParam, toolTypeMap map[string]string) []*dto.ResponseInputItem {
	if msg == nil {
		return nil
	}
	var items []*dto.ResponseInputItem

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		items = append(items, &dto.ResponseInputItem{
			Type: lo.ToPtr(enum.ResponseInputItemTypeReasoning),
			Summary: []*dto.ResponseReasoningSummary{{
				Text: *msg.ReasoningContent,
				Type: enum.ResponseContentTypeSummaryText,
			}},
		})
	}

	toolCallItems := lo.FilterMap(msg.ToolCalls, func(tc *dto.OpenAIChatCompletionMessageToolCall, _ int) (*dto.ResponseInputItem, bool) {
		if tc == nil || tc.Function == nil {
			return nil, false
		}
		itemType := resolveToolCallOutputType(tc.Function.Name, toolTypeMap)
		return &dto.ResponseInputItem{
			Type:      lo.ToPtr(itemType),
			CallID:    tc.ID,
			Name:      lo.ToPtr(tc.Function.Name),
			Arguments: lo.ToPtr(tc.Function.Arguments),
		}, true
	})
	items = append(items, toolCallItems...)

	if textItem := buildTextOutputItem(msg); textItem != nil {
		items = append(items, textItem)
	}
	return items
}

func resolveToolCallOutputType(functionName string, toolTypeMap map[string]string) string {
	if origType, ok := toolTypeMap[functionName]; ok {
		switch origType {
		case enum.ResponseToolTypeLocalShell:
			return enum.ResponseInputItemTypeLocalShellCall
		case enum.ResponseToolTypeCustom, enum.ResponseToolTypeApplyPatch, enum.ResponseToolTypeShell:
			return enum.ResponseInputItemTypeCustomToolCall
		}
	}
	return enum.ResponseInputItemTypeFunctionCall
}

func buildTextOutputItem(msg *dto.OpenAIChatCompletionMessageParam) *dto.ResponseInputItem {
	content := chatContentToResponseContent(msg.Content, enum.ResponseContentTypeOutputText)
	if msg.Refusal != nil && *msg.Refusal != "" {
		content = append(content, &dto.ResponseInputContent{Type: enum.ResponseContentTypeRefusal, Refusal: msg.Refusal})
	}
	if len(content) == 0 {
		return nil
	}
	return &dto.ResponseInputItem{
		Type:    lo.ToPtr(enum.ResponseInputItemTypeMessage),
		Role:    lo.ToPtr(enum.RoleAssistant),
		Content: &dto.ResponseInputMessageContent{Parts: content},
	}
}

func BuildToolTypeMap(tools []*dto.ResponseTool) map[string]string {
	validTools := lo.Filter(tools, func(t *dto.ResponseTool, _ int) bool {
		return t != nil && responseToolFunctionName(t) != ""
	})
	return lo.SliceToMap(validTools, func(t *dto.ResponseTool) (string, string) {
		return responseToolFunctionName(t), responseToolOrigType(t)
	})
}

func responseToolFunctionName(t *dto.ResponseTool) string {
	switch {
	case t.Function != nil:
		return t.Function.Name
	case t.Custom != nil:
		return t.Custom.Name
	case t.WebSearch != nil:
		return t.WebSearch.Type
	case t.WebSearchPreview != nil:
		return t.WebSearchPreview.Type
	case t.FileSearch != nil:
		return t.FileSearch.Type
	case t.Computer != nil:
		return t.Computer.Type
	case t.ComputerUsePreview != nil:
		return t.ComputerUsePreview.Type
	case t.CodeInterpreter != nil:
		return t.CodeInterpreter.Type
	case t.ImageGeneration != nil:
		return t.ImageGeneration.Type
	case t.LocalShell != nil:
		return t.LocalShell.Type
	case t.Shell != nil:
		return t.Shell.Type
	case t.Mcp != nil:
		return t.Mcp.Type
	case t.ApplyPatch != nil:
		return t.ApplyPatch.Type
	case t.Namespace != nil:
		return t.Namespace.Name
	case t.ToolSearch != nil:
		return t.ToolSearch.Type
	}
	return ""
}

func (c *ResponseProtocolConverter) SetToolTypeMap(m map[string]string) {
	c.toolTypeMap = m
}

func (c *ResponseProtocolConverter) ToolTypeMap() map[string]string {
	return c.toolTypeMap
}

func responseToolOrigType(t *dto.ResponseTool) string {
	switch {
	case t.Function != nil:
		return enum.ResponseToolTypeFunction
	case t.Custom != nil:
		return enum.ResponseToolTypeCustom
	case t.WebSearch != nil:
		return enum.ResponseToolTypeWebSearch
	case t.WebSearchPreview != nil:
		return enum.ResponseToolTypeWebSearchPreview
	case t.FileSearch != nil:
		return enum.ResponseToolTypeFileSearch
	case t.Computer != nil:
		return enum.ResponseToolTypeComputer
	case t.ComputerUsePreview != nil:
		return enum.ResponseToolTypeComputerUsePreview
	case t.CodeInterpreter != nil:
		return enum.ResponseToolTypeCodeInterpreter
	case t.ImageGeneration != nil:
		return enum.ResponseToolTypeImageGeneration
	case t.LocalShell != nil:
		return enum.ResponseToolTypeLocalShell
	case t.Shell != nil:
		return enum.ResponseToolTypeShell
	case t.Mcp != nil:
		return enum.ResponseToolTypeMcp
	case t.ApplyPatch != nil:
		return enum.ResponseToolTypeApplyPatch
	case t.Namespace != nil:
		return enum.ResponseToolTypeNamespace
	case t.ToolSearch != nil:
		return enum.ResponseToolTypeToolSearch
	}
	return ""
}

func chatContentToResponseContent(content *dto.OpenAIMessageContent, textType string) []*dto.ResponseInputContent {
	if content == nil {
		return nil
	}
	if len(content.Parts) == 0 {
		return []*dto.ResponseInputContent{{Type: textType, Text: lo.ToPtr(content.Text)}}
	}
	return lo.FilterMap(content.Parts, func(part *dto.OpenAIChatCompletionContentPart, _ int) (*dto.ResponseInputContent, bool) {
		if part == nil {
			return nil, false
		}
		switch part.Type {
		case enum.ContentPartTypeText:
			return &dto.ResponseInputContent{Type: textType, Text: part.Text}, true
		case enum.ContentPartTypeImageURL:
			if part.ImageURL == nil {
				return nil, false
			}
			c := &dto.ResponseInputContent{Type: enum.ResponseContentTypeInputImage, ImageURL: lo.ToPtr(part.ImageURL.URL)}
			if part.ImageURL.Detail != "" {
				detail := part.ImageURL.Detail
				c.Detail = &detail
			}
			return c, true
		default:
			return nil, false
		}
	})
}
