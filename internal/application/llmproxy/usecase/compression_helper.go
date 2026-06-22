package usecase

import "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"

// compItems 从 CompressionStats 提取 per-item 压缩结果列表。
// 若 stats 为 nil 则返回 nil。
func compItems(stats *compression.CompressionStats) []compression.ItemCompressionResult {
	if stats == nil {
		return nil
	}
	return stats.Items
}
