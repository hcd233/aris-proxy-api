package repository

import "sort"

// ChunkSortedUniqueIDs 对输入的 ID 切片去重、升序排序，再按 chunkSize 切分为多个
// 不重叠的子切片。
//
// 设计动机：当输入 ID 数量很大（万级以上）时，单纯把全部 ID 塞进
// "SELECT ... WHERE id IN (?)" 会产生非常宽的 IN 列表与巨大的绑定参数；按固定大小
// 切块后，每块单独发一次 IN 查询，既规避 PG 单语句 65535 bind param 的硬上限，
// 也把顺序的 keyset 分页（FindInBatches 反复 WHERE id IN (all) AND id > last_id）
// 压缩为少量且可预期的查询。
//
// 排序与去重同时让每块的 ID 范围在物理上连续，PG planner 可以走主键索引区间扫描，
// 避免对大 IN 列表反复做 hash 探测。
//
// 行为契约：
//
//   - ids == nil 或 len == 0：返回 nil（与 callers 期望的"无 chunk"一致）
//
//   - chunkSize <= 0：退化为 chunkSize=1（保证每块非空且不会死循环）
//
//   - 切分时按"前半段 len == chunkSize、最后一段可能更短"分配
//
//     @param ids []uint 输入 ID 列表
//     @param chunkSize int 每块最大 ID 数量
//     @return [][]uint 切分后的子切片，每个子切片已升序去重
//     @author centonhuang
//     @update 2026-06-07 01:30:00
func ChunkSortedUniqueIDs(ids []uint, chunkSize int) [][]uint {
	if len(ids) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 1
	}
	seen := make(map[uint]struct{}, len(ids))
	uniq := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i] < uniq[j] })

	chunks := make([][]uint, 0, (len(uniq)+chunkSize-1)/chunkSize)
	for start := 0; start < len(uniq); start += chunkSize {
		end := start + chunkSize
		if end > len(uniq) {
			end = len(uniq)
		}
		chunks = append(chunks, uniq[start:end])
	}
	return chunks
}
