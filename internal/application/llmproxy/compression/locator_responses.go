package compression

import (
	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// OpenAIResponsesLocator 扫描 OpenAI Responses body 中的 input[type=function_call_output]。
type OpenAIResponsesLocator struct{}

func (l *OpenAIResponsesLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] Responses: failed to parse body", zap.Error(err))
		return nil, CompressionStats{}
	}

	inputRaw, ok := bodyMap["input"]
	if !ok {
		return nil, CompressionStats{}
	}

	stats := CompressionStats{}
	modified := false

	switch input := inputRaw.(type) {
	case []any:
		for _, itemRaw := range input {
			item, ok := itemRaw.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType != constant.CompressionJSONKeyFuncCallOutput {
				continue
			}
			output, ok := item["output"].(string)
			if !ok {
				continue
			}
			if len(output) < minToolOutputBytes {
				stats.addItem(ItemCompressionResult{
					Output:      output,
					Strategy:    constant.CompressionStrategySkippedTooSmall,
					Applied:     false,
					BytesBefore: len(output),
					BytesAfter:  len(output),
				})
				continue
			}
			result := dispatcher.Compress(output)
			stats.addItem(result)
			if result.Applied {
				item["output"] = result.Output
				modified = true
			}
		}

	case string:
		// input 是字符串时不处理
		return nil, CompressionStats{}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] Responses: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}
