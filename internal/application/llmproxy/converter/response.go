package converter

import (
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ResponseProtocolConverter 以 OpenAI ChatCompletion 作为 OpenAI Response API 的兼容基座。
type ResponseProtocolConverter struct{}

func (*ResponseProtocolConverter) FromResponseRequest(req *dto.OpenAICreateResponseReq) (*dto.OpenAIChatCompletionReq, error) {
	chatReq := &dto.OpenAIChatCompletionReq{
		Model:       lo.FromPtr(req.Model),
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Metadata:    req.Metadata,
	}
	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = lo.ToPtr(int(*req.MaxOutputTokens))
	}
	if req.Text != nil && req.Text.Format != nil {
		chatReq.ResponseFormat = responseTextFormatToChat(req.Text.Format)
	}
	if len(req.Tools) > 0 {
		chatReq.Tools = responseToolsToChat(req.Tools)
	}
	if req.ToolChoice != nil {
		chatReq.ToolChoice = responseToolChoiceToChat(req.ToolChoice)
	}

	messages, err := responseInputToChatMessages(req)
	if err != nil {
		return nil, err
	}
	chatReq.Messages = messages
	return chatReq, nil
}

func (*ResponseProtocolConverter) ToResponseResponse(completion *dto.OpenAIChatCompletion) (*dto.OpenAICreateResponseRsp, error) {
	if completion == nil {
		return nil, ierr.New(ierr.ErrDTOConvert, "openai chat completion is nil")
	}
	rsp := &dto.OpenAICreateResponseRsp{
		ID:        completion.ID,
		Object:    constant.OpenAIResponseObject,
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
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return rsp, nil
	}
	item, err := chatMessageToResponseOutput(completion.Choices[0].Message)
	if err != nil {
		return nil, err
	}
	if item != nil {
		rsp.Output = []*dto.ResponseInputItem{item}
	}
	return rsp, nil
}

func responseInputToChatMessages(req *dto.OpenAICreateResponseReq) ([]*dto.OpenAIChatCompletionMessageParam, error) {
	var messages []*dto.OpenAIChatCompletionMessageParam
	if req.Instructions != nil && *req.Instructions != "" {
		messages = append(messages, &dto.OpenAIChatCompletionMessageParam{
			Role:    enum.RoleSystem,
			Content: &dto.OpenAIMessageContent{Text: *req.Instructions},
		})
	}
	if req.Input == nil {
		return messages, nil
	}
	if len(req.Input.Items) == 0 {
		if req.Input.Text != "" {
			messages = append(messages, &dto.OpenAIChatCompletionMessageParam{
				Role:    enum.RoleUser,
				Content: &dto.OpenAIMessageContent{Text: req.Input.Text},
			})
		}
		return messages, nil
	}
	for i, item := range req.Input.Items {
		chatMsgs, err := responseInputItemToChatMessages(item)
		if err != nil {
			return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert response input item[%d] to chat", i)
		}
		messages = append(messages, chatMsgs...)
	}
	return messages, nil
}

func responseInputItemToChatMessages(item *dto.ResponseInputItem) ([]*dto.OpenAIChatCompletionMessageParam, error) {
	if item == nil {
		return nil, nil
	}
	switch lo.FromPtr(item.Type) {
	case "", enum.ResponseInputItemTypeMessage:
		msg := &dto.OpenAIChatCompletionMessageParam{Role: responseRoleToChat(lo.FromPtr(item.Role))}
		content, err := responseMessageContentToChat(item.Content)
		if err != nil {
			return nil, err
		}
		msg.Content = content
		return []*dto.OpenAIChatCompletionMessageParam{msg}, nil
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallToChat(item)}, nil
	case enum.ResponseInputItemTypeFunctionCallOutput, enum.ResponseInputItemTypeCustomToolCallOutput:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallOutputToChat(item)}, nil
	case enum.ResponseInputItemTypeReasoning:
		if text := responseReasoningText(item); text != "" {
			return []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleAssistant, ReasoningContent: lo.ToPtr(text)}}, nil
		}
	}
	return nil, nil
}

func responseRoleToChat(role string) enum.Role {
	switch role {
	case string(enum.RoleAssistant):
		return enum.RoleAssistant
	case string(enum.RoleSystem), string(enum.RoleDeveloper):
		return enum.RoleSystem
	case string(enum.RoleUser), "":
		return enum.RoleUser
	default:
		return enum.RoleUser
	}
}

func responseMessageContentToChat(content *dto.ResponseInputMessageContent) (*dto.OpenAIMessageContent, error) {
	if content == nil {
		return nil, nil
	}
	if len(content.Parts) == 0 {
		return &dto.OpenAIMessageContent{Text: content.Text}, nil
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
					Detail: enum.ImageDetail(lo.FromPtr(part.Detail)),
				},
			})
		case enum.ResponseContentTypeRefusal:
			texts = append(texts, lo.FromPtr(part.Refusal))
		default:
			continue
		}
	}
	if multimodal {
		return &dto.OpenAIMessageContent{Parts: parts}, nil
	}
	return &dto.OpenAIMessageContent{Text: strings.Join(texts, "\n")}, nil
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
	var texts []string
	for _, part := range output.FunctionOutput.Parts {
		if part != nil && (part.Type == enum.ResponseContentTypeInputText || part.Type == enum.ResponseContentTypeOutputText) {
			texts = append(texts, lo.FromPtr(part.Text))
		}
	}
	return strings.Join(texts, "\n")
}

func responseReasoningText(item *dto.ResponseInputItem) string {
	var texts []string
	for _, summary := range item.Summary {
		if summary != nil && summary.Text != "" {
			texts = append(texts, summary.Text)
		}
	}
	for _, content := range item.ReasoningContent {
		if content != nil && content.Text != "" {
			texts = append(texts, content.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func responseTextFormatToChat(format *dto.ResponseTextFormat) *dto.OpenAIResponseFormat {
	if format == nil {
		return nil
	}
	rspFormat := &dto.OpenAIResponseFormat{Type: enum.ResponseFormatType(format.Type)}
	if format.Schema != nil {
		var schema map[string]any
		schemaBytes, err := sonic.Marshal(format.Schema)
		if err == nil {
			_ = sonic.Unmarshal(schemaBytes, &schema)
		}
		rspFormat.JSONSchema = &dto.OpenAIJSONSchemaFormat{
			Name:        lo.FromPtr(format.Name),
			Description: format.Description,
			Schema:      schema,
			Strict:      format.Strict,
		}
	}
	return rspFormat
}

func responseToolsToChat(tools []*dto.ResponseTool) []dto.OpenAIChatCompletionTool {
	chatTools := make([]dto.OpenAIChatCompletionTool, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if tool.Function != nil {
			chatTools = append(chatTools, dto.OpenAIChatCompletionTool{
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIFunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
					Strict:      &tool.Function.Strict,
				},
			})
		}
	}
	return chatTools
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
	if tc.Object != nil && tc.Object.Type == string(enum.ResponseToolChoiceTypeFunction) {
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

func chatMessageToResponseOutput(msg *dto.OpenAIChatCompletionMessageParam) (*dto.ResponseInputItem, error) {
	if msg == nil {
		return nil, nil
	}
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		if tc.Function != nil {
			return &dto.ResponseInputItem{
				Type:      lo.ToPtr(string(enum.ResponseInputItemTypeFunctionCall)),
				CallID:    tc.ID,
				Name:      lo.ToPtr(tc.Function.Name),
				Arguments: lo.ToPtr(tc.Function.Arguments),
			}, nil
		}
	}
	content := chatContentToResponseContent(msg.Content, enum.ResponseContentTypeOutputText)
	if msg.Refusal != nil && *msg.Refusal != "" {
		content = append(content, &dto.ResponseInputContent{Type: enum.ResponseContentTypeRefusal, Refusal: msg.Refusal})
	}
	return &dto.ResponseInputItem{
		Type:    lo.ToPtr(string(enum.ResponseInputItemTypeMessage)),
		Role:    lo.ToPtr(string(enum.RoleAssistant)),
		Content: &dto.ResponseInputMessageContent{Parts: content},
	}, nil
}

func chatContentToResponseContent(content *dto.OpenAIMessageContent, textType string) []*dto.ResponseInputContent {
	if content == nil {
		return nil
	}
	if len(content.Parts) == 0 {
		return []*dto.ResponseInputContent{{Type: textType, Text: lo.ToPtr(content.Text)}}
	}
	parts := make([]*dto.ResponseInputContent, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		switch part.Type {
		case enum.ContentPartTypeText:
			parts = append(parts, &dto.ResponseInputContent{Type: textType, Text: part.Text})
		case enum.ContentPartTypeImageURL:
			if part.ImageURL != nil {
				detail := string(part.ImageURL.Detail)
				parts = append(parts, &dto.ResponseInputContent{Type: enum.ResponseContentTypeInputImage, ImageURL: lo.ToPtr(part.ImageURL.URL), Detail: &detail})
			}
		}
	}
	return parts
}
