package compression

import (
	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// OpenAIChatLocator 扫描 OpenAI Chat Completions body 中的 messages[role=tool]。
type OpenAIChatLocator struct{}

func (l *OpenAIChatLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] OpenAI Chat: failed to parse body", zap.Error(err))
		return nil, CompressionStats{}
	}

	messagesRaw, ok := bodyMap["messages"]
	if !ok {
		return nil, CompressionStats{}
	}
	messages, ok := messagesRaw.([]any)
	if !ok {
		return nil, CompressionStats{}
	}

	stats := CompressionStats{}
	modified := false

	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != constant.CompressionJSONKeyTool {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			continue
		}
		if len(content) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				Output:      content,
				Strategy:    constant.CompressionStrategySkippedTooSmall,
				Applied:     false,
				BytesBefore: len(content),
				BytesAfter:  len(content),
			})
			continue
		}

		result := dispatcher.Compress(content)
		stats.addItem(result)
		if result.Applied {
			msg["content"] = result.Output
			modified = true
		}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] OpenAI Chat: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}
