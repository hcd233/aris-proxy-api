package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressOpenAIChat 扫描 OpenAI Chat Completions 消息中的 role=tool 消息，
// 压缩其 Content.Text，in-place 修改 DTO。
func CompressOpenAIChat(messages []*dto.OpenAIChatCompletionMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
	stats := CompressionStats{}
	for _, msg := range messages {
		if msg.Role != enum.RoleTool || msg.Content == nil {
			continue
		}
		content := msg.Content.Text
		if len(content) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				ToolCallID:  lo.FromPtr(msg.ToolCallID),
				Input:       content,
				Output:      content,
				Strategy:    constant.CompressionStrategySkippedTooSmall,
				Applied:     false,
				BytesBefore: len(content),
				BytesAfter:  len(content),
			})
			continue
		}
		result := dispatcher.Compress(content)
		result.ToolCallID = lo.FromPtr(msg.ToolCallID)
		result.Input = content
		stats.addItem(result)
		if result.Applied {
			msg.Content.Text = result.Output
			msg.Content.Parts = nil
		}
	}
	return stats
}
