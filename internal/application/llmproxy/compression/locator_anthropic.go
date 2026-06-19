package compression

import (
	"strings"

	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// AnthropicMessagesLocator 扫描 Anthropic Messages body 中的 content[type=tool_result]。
type AnthropicMessagesLocator struct{}

func (l *AnthropicMessagesLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) { //nolint:gocognit // tool_result content has string and array variants, inherently complex
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] Anthropic: failed to parse body", zap.Error(err))
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
		contentRaw, ok := msg["content"]
		if !ok {
			continue
		}
		contentArr, ok := contentRaw.([]any)
		if !ok {
			continue
		}

		for _, blockRaw := range contentArr {
			block, ok := blockRaw.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType != constant.CompressionJSONKeyToolResult {
				continue
			}

			// tool_result 的 content 可以是 string 或 content block 数组
			switch contentVal := block["content"].(type) {
			case string:
				if len(contentVal) < minToolOutputBytes {
					stats.addItem(ItemCompressionResult{
						Output:      contentVal,
						Strategy:    constant.CompressionStrategySkippedTooSmall,
						Applied:     false,
						BytesBefore: len(contentVal),
						BytesAfter:  len(contentVal),
					})
					continue
				}
				result := dispatcher.Compress(contentVal)
				stats.addItem(result)
				if result.Applied {
					block["content"] = result.Output
					modified = true
				}

			case []any:
				// 数组形式：提取所有 text block 的 text，合并后压缩
				combined := extractTextFromBlocks(contentVal)
				if len(combined) < minToolOutputBytes {
					stats.addItem(ItemCompressionResult{
						Output:      combined,
						Strategy:    constant.CompressionStrategySkippedTooSmall,
						Applied:     false,
						BytesBefore: len(combined),
						BytesAfter:  len(combined),
					})
					continue
				}
				result := dispatcher.Compress(combined)
				stats.addItem(result)
				if result.Applied {
					// 替换为单个 text block
					block["content"] = []any{
						map[string]any{constant.CompressionJSONKeyType: constant.CompressionJSONKeyText, constant.CompressionJSONKeyText: result.Output},
					}
					modified = true
				}
			}
		}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] Anthropic: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}

func extractTextFromBlocks(blocks []any) string {
	var texts []string
	for _, blockRaw := range blocks {
		block, ok := blockRaw.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := block["type"].(string); t == constant.CompressionJSONKeyText {
			if text, ok := block["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}
	if len(texts) == 0 {
		return ""
	}
	if len(texts) == 1 {
		return texts[0]
	}
	var sb strings.Builder
	for i, t := range texts {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(t)
	}
	return sb.String()
}
