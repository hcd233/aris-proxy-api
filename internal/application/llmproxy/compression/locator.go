package compression

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// ToolOutputLocator 按协议定位 body 中的 tool output，执行压缩，返回新 body。
// 任何错误都返回原始 body 和空 stats——压缩永不阻塞请求。
type ToolOutputLocator interface {
	LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats)
}

// SelectLocator 按 upstreamProtocol 选择对应的 Locator。
func SelectLocator(upstreamProtocol enum.ProtocolType) ToolOutputLocator {
	switch upstreamProtocol {
	case enum.ProtocolOpenAIChatCompletion:
		return &OpenAIChatLocator{}
	case enum.ProtocolAnthropicMessage:
		return &AnthropicMessagesLocator{}
	case enum.ProtocolOpenAIResponse:
		return &OpenAIResponsesLocator{}
	default:
		return nil
	}
}

// CompressBody 通用入口：根据 upstreamProtocol 选择 Locator 并执行压缩。
// 如果 Locator 不存在或压缩失败，返回原始 body。
func CompressBody(body []byte, upstreamProtocol enum.ProtocolType, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, *CompressionStats) {
	locator := SelectLocator(upstreamProtocol)
	if locator == nil {
		return body, nil
	}
	newBody, stats := locator.LocateAndCompress(body, dispatcher, minToolOutputBytes)
	if newBody == nil {
		return body, nil
	}
	return newBody, &stats
}
