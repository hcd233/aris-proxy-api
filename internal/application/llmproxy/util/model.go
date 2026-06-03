package proxyutil

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
	return lo.Must1(MarshalUpstreamBody(&body))
}

// MarshalRawOpenAIChatCompletionBodyForModel 基于原始 JSON 请求体替换顶层 model 字段。
// 除 model 外的字段由 raw body 决定，避免 DTO round-trip 丢弃未知字段。
//
// 内部用 map[string]sonic.NoCopyRawMessage 重组顶层；尽管每个 value 是 RawMessage 透传，
// 顶层 map 的 key 顺序仍依赖 encoder 是否启用 SortMapKeys，这里使用 MarshalUpstreamBody。
func MarshalRawOpenAIChatCompletionBodyForModel(raw []byte, req *dto.OpenAIChatCompletionReq, modelName string) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return MarshalOpenAIChatCompletionBodyForModel(req, modelName)
	}

	var body map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(trimmed, &body); err != nil || body == nil {
		return MarshalOpenAIChatCompletionBodyForModel(req, modelName)
	}

	body[constant.FieldNameModel] = sonic.NoCopyRawMessage(lo.Must1(sonic.Marshal(modelName)))
	return lo.Must1(MarshalUpstreamBody(body))
}

// MarshalOpenAIResponseBodyForModel 使用上游模型名序列化 Response API 请求体，且不修改原请求。
func MarshalOpenAIResponseBodyForModel(req *dto.OpenAICreateResponseReq, modelName string) []byte {
	body := *req
	body.Model = &modelName
	return lo.Must1(MarshalUpstreamBody(&body))
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
