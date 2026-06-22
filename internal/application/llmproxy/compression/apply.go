package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// ApplyResultsToMessages 按 tool_call_id 将压缩结果回填到 UnifiedMessage。
// 仅处理 Applied=true 的结果：设置 RawContent(before) 和 CompressionStrategy。
// 未匹配到的消息不受影响。
func ApplyResultsToMessages(messages []*vo.UnifiedMessage, results []ItemCompressionResult) {
	if len(results) == 0 {
		return
	}
	resultsByID := lo.SliceToMap(results, func(r ItemCompressionResult) (string, ItemCompressionResult) {
		return r.ToolCallID, r
	})
	for _, msg := range messages {
		if msg.ToolCallID == "" {
			continue
		}
		if result, ok := resultsByID[msg.ToolCallID]; ok && result.Applied {
			msg.RawContent = &result.Input
			msg.CompressionStrategy = result.Strategy
		}
	}
}
