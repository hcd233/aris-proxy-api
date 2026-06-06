// Package session_list_batch_perf 验证 session 列表接口在「空 summary fallback」分支
// 下不再用 FindInBatches(500) 顺序 keyset 拉消息，而是把 ID 排序去重后按固定大小切块、
// 一次性用 IN 列表加载，从而把 N 次顺序往返压缩为 ceil(N/chunkSize) 次单查询。
//
// 回归背景（refactor/session-list-batch-perf-2026-06-07）：
//   - 线上 trace efe54869-d52d-4375-9ea4-366b26923283 显示
//     GET /api/v1/session/list 总耗时 6.1s，其中 5.4s 来自
//     loadMessagesForEmptySummaries → BatchGetByField → FindInBatches(500)，
//     对 200 个空 summary session 的 ~12000 条 message IDs 发出 24 次
//     SELECT ... IN (501 ids) AND id > last_id LIMIT 500。
//   - 修复方案：抽 ChunkSortedUniqueIDs 纯函数做排序去重切块；
//     session_read_repository 暴露 FindMessagesByIDsChunked，
//     每次 chunk 用一条 SELECT ... WHERE id IN (?) 拉，绕过 GORM keyset。
//
// 本测试覆盖：
//  1. ChunkSortedUniqueIDs 正确去重、排序、切块
//  2. 空输入/单元素/整除/不整除/极小块等边界
//  3. chunkSize <= 0 时退化为单元素块
package session_list_batch_perf

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func TestChunkSortedUniqueIDs_DedupesAndSortsAndChunks(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{5, 3, 5, 1, 3, 9, 7, 9}, 3)
	want := [][]uint{{1, 3, 5}, {7, 9}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_Empty(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs(nil, 100)
	if len(got) != 0 {
		t.Fatalf("nil input should produce 0 chunks, got %d: %v", len(got), got)
	}
	got = repository.ChunkSortedUniqueIDs([]uint{}, 100)
	if len(got) != 0 {
		t.Fatalf("empty slice should produce 0 chunks, got %d: %v", len(got), got)
	}
}

func TestChunkSortedUniqueIDs_SingleElement(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{42}, 100)
	want := [][]uint{{42}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_ExactMultiple(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{6, 1, 4, 2, 5, 3}, 2)
	want := [][]uint{{1, 2}, {3, 4}, {5, 6}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_RemainderChunk(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{1, 2, 3, 4, 5}, 2)
	want := [][]uint{{1, 2}, {3, 4}, {5}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_ChunkSizeOne(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{3, 1, 2, 1}, 1)
	want := [][]uint{{1}, {2}, {3}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_ChunkSizeLargerThanInput(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{2, 1}, 100)
	want := [][]uint{{1, 2}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_ChunkSizeZeroOrNegative(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{2, 1, 3}, 0)
	want := [][]uint{{1}, {2}, {3}}
	assertChunksEqual(t, want, got)
	got = repository.ChunkSortedUniqueIDs([]uint{2, 1, 3}, -5)
	want = [][]uint{{1}, {2}, {3}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_AllDuplicates(t *testing.T) {
	t.Parallel()
	got := repository.ChunkSortedUniqueIDs([]uint{7, 7, 7, 7}, 2)
	want := [][]uint{{7}}
	assertChunksEqual(t, want, got)
}

func TestChunkSortedUniqueIDs_LargeInputProducesFewerChunksThanFindInBatches(t *testing.T) {
	t.Parallel()
	const totalIDs = 12000
	ids := make([]uint, totalIDs)
	for i := 0; i < totalIDs; i++ {
		ids[i] = uint(i)
	}
	const chunkSize = 5000
	got := repository.ChunkSortedUniqueIDs(ids, chunkSize)
	if len(got) != 3 {
		t.Fatalf("12000 ids / 5000 chunk = 3 chunks, got %d", len(got))
	}
	if got[0] == nil || got[1] == nil || got[2] == nil {
		t.Fatalf("expected 3 non-nil chunks, got %v", got)
	}
	if len(got[0]) != chunkSize || len(got[1]) != chunkSize || len(got[2]) != 2000 {
		t.Fatalf("chunk sizes = [%d, %d, %d], want [%d, %d, %d]",
			len(got[0]), len(got[1]), len(got[2]), chunkSize, chunkSize, 2000)
	}
	if got[0][0] != 0 || got[0][chunkSize-1] != uint(chunkSize-1) {
		t.Errorf("first chunk must be [0..4999], got [%d..%d]", got[0][0], got[0][chunkSize-1])
	}
	if got[1][0] != uint(chunkSize) || got[1][chunkSize-1] != uint(2*chunkSize-1) {
		t.Errorf("second chunk must be [5000..9999], got [%d..%d]", got[1][0], got[1][chunkSize-1])
	}
	if got[2][0] != uint(2*chunkSize) || got[2][1999] != uint(totalIDs-1) {
		t.Errorf("third chunk must be [10000..11999], got [%d..%d]", got[2][0], got[2][1999])
	}
}

func assertChunksEqual(t *testing.T, want, got [][]uint) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("len = %d, want %d; want=%v got=%v", len(got), len(want), want, got)
	}
	for i := range want {
		if len(want[i]) != len(got[i]) {
			t.Fatalf("chunk[%d] len = %d, want %d; want=%v got=%v", i, len(got[i]), len(want[i]), want[i], got[i])
		}
		for j := range want[i] {
			if want[i][j] != got[i][j] {
				t.Fatalf("chunk[%d][%d] = %d, want %d; want=%v got=%v", i, j, got[i][j], want[i][j], want[i], got[i])
			}
		}
	}
}
