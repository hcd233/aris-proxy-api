// Package query session 查询实现
package query

import (
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// BuildMessageCountBuckets 按固定边界 + 动态上限生成消息数区间桶，仅保留有会话的桶。
//
// maxCount 为当前时间范围最大会话消息数：末桶上限被 maxCount 截断（如 max=82 时
// 桶为 0-10 / 11-50 / 51-82），超出 maxCount 的桶不生成；maxCount <= 0（无会话）返回 nil。
//
// @author centonhuang
// @update 2026-08-12 16:00:00
func BuildMessageCountBuckets(maxCount int, bucketCounts map[int]int64) []string {
	if maxCount <= 0 {
		return nil
	}

	// 定位最后一个有意义的桶：maxCount 落在哪个固定边界内（501+ 时指向末尾动态桶）
	edges := constant.SessionMessageCountBucketEdges
	last := len(edges)
	for i, edge := range edges {
		if maxCount <= edge {
			last = i
			break
		}
	}

	items := make([]string, 0, last+1)
	lower := 0
	for i := 0; i <= last; i++ {
		upper := maxCount
		if i < len(edges) && i < last {
			upper = edges[i]
		}
		if bucketCounts[i] > 0 {
			items = append(items, fmt.Sprintf(constant.SessionMessageCountBucketFormat, lower, upper))
		}
		lower = upper + 1
	}
	return items
}
