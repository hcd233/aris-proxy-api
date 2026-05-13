package util

import (
	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// MarshalOpenAIChatCompletionBodyForModel 使用上游模型名序列化 ChatCompletion 请求体，且不修改原请求。
func MarshalOpenAIChatCompletionBodyForModel(req *dto.OpenAIChatCompletionReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(sonic.Marshal(&body))
}

// MarshalOpenAIResponseBodyForModel 使用上游模型名序列化 Response API 请求体，且不修改原请求。
func MarshalOpenAIResponseBodyForModel(req *dto.OpenAICreateResponseReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(sonic.Marshal(&body))
}

// MarshalAnthropicMessageBodyForModel 使用上游模型名序列化 Anthropic Message 请求体，且不修改原请求。
func MarshalAnthropicMessageBodyForModel(req *dto.AnthropicCreateMessageReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(sonic.Marshal(&body))
}

// MarshalAnthropicCountTokensBodyForModel 使用上游模型名序列化 Anthropic CountTokens 请求体，且不修改原请求。
func MarshalAnthropicCountTokensBodyForModel(req *dto.AnthropicCountTokensReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(sonic.Marshal(&body))
}

// ReplaceModelInBody 替换 raw JSON body 中的 model 字段。
// 仅用于未持有 typed DTO 的上游响应体；请求体请优先使用上面的 typed marshal helper。
func ReplaceModelInBody(body []byte, modelName string) []byte {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Util] ReplaceModelInBody unmarshal error", zap.Error(err))
		return body
	}
	bodyMap["model"] = modelName
	return lo.Must1(sonic.Marshal(bodyMap))
}

// ReplaceModelInSSEData 替换 Anthropic SSE data 中的 model 字段（包括嵌套的 message.model）
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
	return lo.Must1(sonic.Marshal(dataMap))
}
