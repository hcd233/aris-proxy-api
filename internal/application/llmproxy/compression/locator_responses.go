package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressOpenAIResponses 扫描 OpenAI Responses input 中的 function_call_output 项，
// 压缩其 Output.Text，in-place 修改 DTO。
func CompressOpenAIResponses(items []*dto.ResponseInputItem, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
	stats := CompressionStats{}
	for _, item := range items {
		if lo.FromPtr(item.Type) != constant.CompressionJSONKeyFuncCallOutput || item.Output == nil {
			continue
		}
		output := item.Output.Text
		if len(output) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				ToolCallID:  lo.FromPtr(item.CallID),
				Input:       output,
				Output:      output,
				Strategy:    constant.CompressionStrategySkippedTooSmall,
				Applied:     false,
				BytesBefore: len(output),
				BytesAfter:  len(output),
			})
			continue
		}
		result := dispatcher.Compress(output)
		result.ToolCallID = lo.FromPtr(item.CallID)
		result.Input = output
		stats.addItem(result)
		if result.Applied {
			item.Output.Text = result.Output
			item.Output.FunctionOutput = nil
		}
	}
	return stats
}
