package proxyutil

import (
	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// MarshalOpenAIChatCompletionBodyForModel 使用上游模型名序列化 ChatCompletion 请求体，且不修改原请求。
//
// 使用 MarshalUpstreamBody 保证字节稳定（map key 字典序），守护上游 prompt cache 命中。
func MarshalOpenAIChatCompletionBodyForModel(req *dto.OpenAIChatCompletionReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	body.Messages = normalizeMessagesForUpstream(body.Messages)

	// 流式请求强制要求上游返回 usage 信息，确保审计 token 计数准确。
	// 部分上游（如 tokenhub.tencentmaas.com）在 standard streaming 中不返回
	// usage，必须通过 stream_options.include_usage 显式请求。
	if body.Stream != nil && *body.Stream {
		if body.StreamOptions == nil {
			body.StreamOptions = &dto.OpenAIChatCompletionStreamOptions{
				IncludeUsage: lo.ToPtr(true),
			}
		} else if body.StreamOptions.IncludeUsage == nil || !*body.StreamOptions.IncludeUsage {
			body.StreamOptions.IncludeUsage = lo.ToPtr(true)
		}
	}

	return lo.Must1(MarshalUpstreamBody(&body))
}

// normalizeMessagesForUpstream 将请求消息中的角色标准化为上游兼容格式。
//
// OpenAI 的 "developer" 角色（o1 系列及以上）并非所有上游都支持，
// 统一转为等价的 "system" 角色以保证兼容性。
func normalizeMessagesForUpstream(msgs []*dto.OpenAIChatCompletionMessageParam) []*dto.OpenAIChatCompletionMessageParam {
	isDev := func(msg *dto.OpenAIChatCompletionMessageParam) bool {
		return msg != nil && msg.Role == enum.RoleDeveloper
	}
	if !lo.SomeBy(msgs, isDev) {
		return msgs
	}
	return lo.Map(msgs, func(msg *dto.OpenAIChatCompletionMessageParam, _ int) *dto.OpenAIChatCompletionMessageParam {
		if !isDev(msg) {
			return msg
		}
		cp := *msg
		cp.Role = enum.RoleSystem
		return &cp
	})
}

// MarshalOpenAIResponseBodyForModel 使用上游模型名序列化 Response API 请求体，且不修改原请求。
func MarshalOpenAIResponseBodyForModel(req *dto.OpenAICreateResponseReq, modelName string) []byte {
	body := *req
	body.Model = &modelName
	if req.Input != nil && req.Input.Items != nil {
		input := *req.Input
		input.Items = make([]*dto.ResponseInputItem, 0, len(req.Input.Items))
		for _, item := range req.Input.Items {
			input.Items = append(input.Items, normalizeResponseInputItemForUpstream(item))
		}
		body.Input = &input
	}
	return lo.Must1(MarshalUpstreamBody(&body))
}

func normalizeResponseInputItemForUpstream(item *dto.ResponseInputItem) *dto.ResponseInputItem {
	if item == nil {
		return nil
	}
	copied := *item
	if lo.FromPtr(copied.Type) != enum.ResponseInputItemTypeReasoning {
		copied.Summary = nil
		return &copied
	}
	if copied.Summary == nil {
		empty := make([]*dto.ResponseReasoningSummary, 0)
		copied.Summary = &empty
	}
	return &copied
}

// MarshalAnthropicMessageBodyForModel 使用上游模型名序列化 Anthropic Message 请求体，且不修改原请求。
func MarshalAnthropicMessageBodyForModel(req *dto.AnthropicCreateMessageReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(MarshalUpstreamBody(&body))
}

// MarshalAnthropicCountTokensBodyForModel 使用上游模型名序列化 Anthropic CountTokens 请求体，且不修改原请求。
func MarshalAnthropicCountTokensBodyForModel(req *dto.AnthropicCountTokensReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(MarshalUpstreamBody(&body))
}

// ReplaceModelInBody 替换 raw JSON body 中的 model 字段。
// 仅用于未持有 typed DTO 的上游响应体；请求体请优先使用上面的 typed marshal helper。
//
// 使用 MarshalUpstreamBody 保证 map[string]any 输出 key 顺序稳定，避免下游再次序列化时漂移。
func ReplaceModelInBody(body []byte, modelName string) []byte {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Util] ReplaceModelInBody unmarshal error", zap.Error(err))
		return body
	}
	bodyMap["model"] = modelName
	return lo.Must1(MarshalUpstreamBody(bodyMap))
}

// ReplaceModelInSSEData 替换 Anthropic SSE data 中的 model 字段（包括嵌套的 message.model）
//
// 使用 MarshalUpstreamBody 保证 map key 顺序稳定。
func ReplaceModelInSSEData(data []byte, modelName string) []byte {
	var dataMap map[string]any
	if err := sonic.Unmarshal(data, &dataMap); err != nil {
		logger.Logger().Warn("[Util] ReplaceModelInSSEData unmarshal error", zap.Error(err))
		return data
	}
	if msgRaw, ok := dataMap["message"]; ok {
		if msgMap, ok := msgRaw.(map[string]any); ok {
			if _, hasModel := msgMap["model"]; hasModel {
				msgMap["model"] = modelName
			}
		}
	}
	if _, hasModel := dataMap["model"]; hasModel {
		dataMap["model"] = modelName
	}
	return lo.Must1(MarshalUpstreamBody(dataMap))
}

// MarshalRawOpenAIChatCompletionBodyForModel 使用上游模型名序列化原始 ChatCompletion 请求体，保留未知字段。
//
// 与 MarshalOpenAIChatCompletionBodyForModel 不同，此函数接受原始 JSON body，
// 解析后仅替换 model 字段，保留所有未知字段（包括 messages 中的扩展字段）。
func MarshalRawOpenAIChatCompletionBodyForModel(raw []byte, modelName string) []byte {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(raw, &bodyMap); err != nil {
		logger.Logger().Warn("[Util] MarshalRawOpenAIChatCompletionBodyForModel unmarshal error", zap.Error(err))
		return raw
	}
	bodyMap["model"] = modelName
	return lo.Must1(MarshalUpstreamBody(bodyMap))
}
