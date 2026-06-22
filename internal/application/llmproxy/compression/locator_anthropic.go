package compression

import (
	"strings"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressAnthropicMessages 扫描 Anthropic Messages 中的 tool_result content block，
// 压缩其 Content，in-place 修改 DTO。
func CompressAnthropicMessages(messages []*dto.AnthropicMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats { //nolint:gocognit // tool_result content has string and array variants, inherently complex
	stats := CompressionStats{}
	for _, msg := range messages {
		if msg.Content == nil {
			continue
		}
		for _, block := range msg.Content.Blocks {
			if block.Type != constant.CompressionJSONKeyToolResult || block.Content == nil {
				continue
			}
			content := extractAnthropicToolResultText(block.Content)
			if len(content) < minToolOutputBytes {
				stats.addItem(ItemCompressionResult{
					ToolCallID:  lo.FromPtr(block.ToolUseID),
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
			result.ToolCallID = lo.FromPtr(block.ToolUseID)
			result.Input = content
			stats.addItem(result)
			if result.Applied {
				block.Content.Text = result.Output
				block.Content.Blocks = nil
			}
		}
	}
	return stats
}

// extractAnthropicToolResultText 从 AnthropicToolResultContent 中提取文本。
// 若 Text 非空则直接返回；若 Blocks 非空则提取所有 text block 的 text 合并返回。
func extractAnthropicToolResultText(content *dto.AnthropicToolResultContent) string {
	if content.Text != "" {
		return content.Text
	}
	if len(content.Blocks) == 0 {
		return ""
	}
	texts := make([]string, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Text != nil {
			texts = append(texts, *block.Text)
		}
	}
	return strings.Join(texts, "\n")
}
