package converter

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/dto/schema"
)

// ResponseProtocolConverter 以 OpenAI ChatCompletion 作为 OpenAI Response API 的兼容基座。
type ResponseProtocolConverter struct {
	toolTypeMap  map[string]string // Chat function name → original Response tool type
	namespaceMap map[string]string // flattened function name → namespace name
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
			IncludeUsage:       lo.ToPtr(true),
		}
	} else if req.Stream != nil && *req.Stream {
		chatReq.StreamOptions = &dto.OpenAIChatCompletionStreamOptions{
			IncludeUsage: lo.ToPtr(true),
		}
	}
	if len(req.Tools) > 0 {
		chatReq.Tools = responseToolsToChat(req.Tools)
		c.toolTypeMap = BuildToolTypeMap(req.Tools)
		c.namespaceMap = BuildNamespaceMap(req.Tools)
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
	items := chatMessageToResponseOutputs(completion.Choices[0].Message, c.toolTypeMap, c.namespaceMap)
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
	messages = append(messages, mergeConsecutiveAssistantMessages(chatMsgs)...)
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
		if isEmptyAssistantMessage(msg) {
			return nil
		}
		return []*dto.OpenAIChatCompletionMessageParam{msg}
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallToChat(item)}
	case enum.ResponseInputItemTypeFunctionCallOutput, enum.ResponseInputItemTypeCustomToolCallOutput:
		return []*dto.OpenAIChatCompletionMessageParam{responseFunctionCallOutputToChat(item)}
	case enum.ResponseInputItemTypeReasoning:
		return nil
	}
	return nil
}

func mergeConsecutiveAssistantMessages(msgs []*dto.OpenAIChatCompletionMessageParam) []*dto.OpenAIChatCompletionMessageParam {
	if len(msgs) == 0 {
		return msgs
	}
	merged := make([]*dto.OpenAIChatCompletionMessageParam, 0, len(msgs))
	var pending *dto.OpenAIChatCompletionMessageParam
	flushPending := func() {
		if pending != nil && !isEmptyAssistantMessage(pending) {
			merged = append(merged, pending)
		}
		pending = nil
	}
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.Role != enum.RoleAssistant {
			flushPending()
			merged = append(merged, msg)
			continue
		}
		if pending == nil {
			pending = msg
		} else {
			mergeAssistantInto(pending, msg)
		}
	}
	flushPending()
	return merged
}

func mergeAssistantInto(dst, src *dto.OpenAIChatCompletionMessageParam) {
	if dst.Content == nil || (dst.Content.Text == "" && len(dst.Content.Parts) == 0) {
		dst.Content = src.Content
	}
	dst.ToolCalls = append(dst.ToolCalls, src.ToolCalls...)
	if src.ReasoningContent != nil && *src.ReasoningContent != "" {
		dst.ReasoningContent = src.ReasoningContent
	}
	if src.Refusal != nil && *src.Refusal != "" {
		dst.Refusal = src.Refusal
	}
}

func isEmptyAssistantMessage(msg *dto.OpenAIChatCompletionMessageParam) bool {
	if msg == nil || msg.Role != enum.RoleAssistant {
		return false
	}
	if len(msg.ToolCalls) > 0 {
		return false
	}
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		return false
	}
	if msg.Refusal != nil && *msg.Refusal != "" {
		return false
	}
	return msg.Content == nil || (msg.Content.Text == "" && len(msg.Content.Parts) == 0)
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
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		switch part.Type {
		case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
			parts = append(parts, &dto.OpenAIChatCompletionContentPart{Type: enum.ContentPartTypeText, Text: part.Text})
		case enum.ResponseContentTypeInputImage:
			parts = append(parts, &dto.OpenAIChatCompletionContentPart{
				Type: enum.ContentPartTypeImageURL,
				ImageURL: &dto.OpenAIChatCompletionImageURL{
					URL:    lo.FromPtr(part.ImageURL),
					Detail: lo.FromPtr(part.Detail),
				},
			})
		case enum.ResponseContentTypeRefusal:
			parts = append(parts, &dto.OpenAIChatCompletionContentPart{Type: enum.ContentPartTypeText, Text: part.Refusal})
		default:
			continue
		}
	}
	if len(parts) == 0 {
		return &dto.OpenAIMessageContent{Text: content.Text}
	}
	return &dto.OpenAIMessageContent{Parts: parts}
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
	name := lo.FromPtr(item.Name)
	if ns := lo.FromPtr(item.Namespace); ns != "" && name != "" {
		name = ns + constant.NamespaceToolSeparator + name
	}
	return &dto.OpenAIChatCompletionMessageParam{
		Role: enum.RoleAssistant,
		ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
			ID:   lo.ToPtr(callID),
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
				Name:      name,
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
	var result []dto.OpenAIChatCompletionTool
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if tool.Namespace != nil {
			result = append(result, convertNamespaceToolsToChat(tool.Namespace)...)
			continue
		}
		if t, ok := convertResponseToolToChat(tool); ok {
			result = append(result, t)
		}
	}
	return result
}

// convertNamespaceToolsToChat 将 namespace 工具内的子工具铺平为独立的 ChatCompletion function/custom 工具。
// 子工具名称使用 `{namespace}__{subToolName}` 格式，保证在 ChatCompletion 协议中唯一可调用。
func convertNamespaceToolsToChat(ns *dto.ResponseToolNamespace) []dto.OpenAIChatCompletionTool {
	if ns == nil || ns.Name == "" {
		return nil
	}
	return lo.FilterMap(ns.Tools, func(sub *dto.ResponseNamespaceTool, _ int) (dto.OpenAIChatCompletionTool, bool) {
		if sub == nil || sub.Name == "" {
			return dto.OpenAIChatCompletionTool{}, false
		}
		flatName := ns.Name + constant.NamespaceToolSeparator + sub.Name
		switch sub.Type {
		case enum.ResponseToolTypeCustom:
			return dto.OpenAIChatCompletionTool{
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIFunctionDefinition{
					Name:        flatName,
					Description: sub.Description,
					Parameters:  sub.Parameters,
					Strict:      sub.Strict,
				},
			}, true
		default:
			return dto.OpenAIChatCompletionTool{
				Type: enum.ToolTypeFunction,
				Function: &dto.OpenAIFunctionDefinition{
					Name:        flatName,
					Description: sub.Description,
					Parameters:  sub.Parameters,
					Strict:      sub.Strict,
				},
			}, true
		}
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
		desc := lo.FromPtr(tool.Custom.Description)
		if tool.Custom.Format != nil && tool.Custom.Format.Definition != nil {
			formatLabel := lo.FromPtr(tool.Custom.Format.Syntax)
			if formatLabel == "" {
				formatLabel = constant.CustomToolFormatDefault
			}
			desc += fmt.Sprintf(constant.CustomToolFormatLabelFmt, formatLabel) + *tool.Custom.Format.Definition
		}
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.Custom.Name,
				Description: lo.ToPtr(desc),
				Parameters: &schema.JSONSchemaProperty{
					JSONSchemaProperty: vo.JSONSchemaProperty{
						Type: lo.ToPtr(vo.JSONSchemaTypeValue{Single: enum.JSONSchemaTypeObject}),
						Properties: &map[string]*vo.JSONSchemaProperty{
							constant.CustomToolParamContent: {
								Type: lo.ToPtr(vo.JSONSchemaTypeValue{Single: enum.JSONSchemaTypeString}),
							},
						},
						Required: []string{constant.CustomToolParamContent},
					},
				},
			},
		}, true
	case tool.FileSearch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.FileSearch.Type,
				Description: lo.ToPtr(constant.ChatCompletionConvertToolDescFileSearch),
			},
		}, true
	case tool.WebSearch != nil:
		return dto.OpenAIChatCompletionTool{}, false
	case tool.WebSearchPreview != nil:
		return dto.OpenAIChatCompletionTool{}, false
	case tool.Computer != nil:
		return dto.OpenAIChatCompletionTool{}, false
	case tool.ComputerUsePreview != nil:
		return dto.OpenAIChatCompletionTool{}, false
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
		return dto.OpenAIChatCompletionTool{}, false
	case tool.LocalShell != nil:
		return dto.OpenAIChatCompletionTool{}, false
	case tool.Shell != nil:
		return dto.OpenAIChatCompletionTool{}, false
	case tool.ToolSearch != nil:
		return dto.OpenAIChatCompletionTool{
			Type: enum.ToolTypeFunction,
			Function: &dto.OpenAIFunctionDefinition{
				Name:        tool.ToolSearch.Type,
				Description: tool.ToolSearch.Description,
			},
		}, true
	case tool.ApplyPatch != nil:
		return dto.OpenAIChatCompletionTool{}, false
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

func chatMessageToResponseOutputs(msg *dto.OpenAIChatCompletionMessageParam, toolTypeMap, namespaceMap map[string]string) []*dto.ResponseInputItem {
	if msg == nil {
		return nil
	}
	var items []*dto.ResponseInputItem

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		items = append(items, &dto.ResponseInputItem{
			Type: lo.ToPtr(enum.ResponseInputItemTypeReasoning),
			Summary: &[]*dto.ResponseReasoningSummary{{
				Text: *msg.ReasoningContent,
				Type: enum.ResponseContentTypeSummaryText,
			}},
		})
	}

	toolCallItems := lo.FilterMap(msg.ToolCalls, func(tc *dto.OpenAIChatCompletionMessageToolCall, _ int) (*dto.ResponseInputItem, bool) {
		if tc == nil {
			return nil, false
		}
		switch {
		case tc.Function != nil:
			itemType := resolveToolCallOutputType(tc.Function.Name, toolTypeMap)
			name, ns := splitNamespacedName(tc.Function.Name, namespaceMap)
			if itemType == enum.ResponseInputItemTypeCustomToolCall {
				return buildCustomToolCallItem(lo.FromPtr(tc.ID), name, ns, tc.Function.Arguments), true
			}
			return &dto.ResponseInputItem{
				Type:      lo.ToPtr(itemType),
				CallID:    tc.ID,
				Name:      lo.ToPtr(name),
				Namespace: lo.ToPtr(ns),
				Arguments: lo.ToPtr(tc.Function.Arguments),
			}, true
		case tc.Custom != nil:
			itemType := resolveToolCallOutputType(tc.Custom.Name, toolTypeMap)
			name, ns := splitNamespacedName(tc.Custom.Name, namespaceMap)
			return &dto.ResponseInputItem{
				Type:      lo.ToPtr(itemType),
				CallID:    tc.ID,
				Name:      lo.ToPtr(name),
				Namespace: lo.ToPtr(ns),
				Input:     lo.ToPtr(tc.Custom.Input),
			}, true
		}
		return nil, false
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

// buildCustomToolCallItem 构建 custom_tool_call 类型的 ResponseInputItem。
// 将 Chat Completions function call 的 arguments（JSON 包装的 {"content": "..."}）
// 拆包为 custom_tool_call 要求的 input 原始字符串。
func buildCustomToolCallItem(callID, name, namespace, arguments string) *dto.ResponseInputItem {
	input := unwrapCustomToolArguments(arguments)
	return &dto.ResponseInputItem{
		Type:      lo.ToPtr(enum.ResponseInputItemTypeCustomToolCall),
		CallID:    lo.ToPtr(callID),
		Name:      lo.ToPtr(name),
		Namespace: lo.ToPtr(namespace),
		Input:     lo.ToPtr(input),
	}
}

func unwrapCustomToolArguments(arguments string) string {
	var wrapper customToolArgumentsWrapper
	if err := sonic.UnmarshalString(arguments, &wrapper); err == nil {
		return wrapper.Content
	}
	return arguments
}

// customToolArgumentsWrapper 用于拆包 Chat Completions function call 的 arguments JSON。
type customToolArgumentsWrapper struct {
	Content string `json:"content"`
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
	m := make(map[string]string)
	for _, t := range tools {
		if t == nil {
			continue
		}
		if t.Namespace != nil {
			for _, sub := range t.Namespace.Tools {
				if sub == nil || sub.Name == "" {
					continue
				}
				flatName := t.Namespace.Name + constant.NamespaceToolSeparator + sub.Name
				m[flatName] = sub.Type
			}
			continue
		}
		name := responseToolFunctionName(t)
		if name == "" {
			continue
		}
		m[name] = responseToolOrigType(t)
	}
	return m
}

// BuildNamespaceMap 构建铺平后的函数名 → 命名空间名称的映射，用于响应方向拆分 namespaced tool call。
func BuildNamespaceMap(tools []*dto.ResponseTool) map[string]string {
	m := make(map[string]string)
	for _, t := range tools {
		if t == nil || t.Namespace == nil || t.Namespace.Name == "" {
			continue
		}
		for _, sub := range t.Namespace.Tools {
			if sub == nil || sub.Name == "" {
				continue
			}
			flatName := t.Namespace.Name + constant.NamespaceToolSeparator + sub.Name
			m[flatName] = t.Namespace.Name
		}
	}
	return m
}

// splitNamespacedName 根据命名空间映射将铺平的函数名拆分回 (subToolName, namespaceName)。
// 未在映射中找到时返回原始名称和空命名空间。
func splitNamespacedName(flatName string, namespaceMap map[string]string) (name, namespace string) {
	if ns, ok := namespaceMap[flatName]; ok {
		return strings.TrimPrefix(flatName, ns+constant.NamespaceToolSeparator), ns
	}
	return flatName, ""
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

func (c *ResponseProtocolConverter) SetNamespaceMap(m map[string]string) {
	c.namespaceMap = m
}

func (c *ResponseProtocolConverter) NamespaceMap() map[string]string {
	return c.namespaceMap
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
