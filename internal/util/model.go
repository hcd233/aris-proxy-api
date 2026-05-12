package util

import (
	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// ReplaceModelInBody 替换 JSON body 中的 model 字段
// 原定义位于 infrastructure/transport，移至 util 以消除 application → infrastructure 依赖。
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
