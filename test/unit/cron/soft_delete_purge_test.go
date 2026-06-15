package cron_test

import (
	"testing"

	"github.com/samber/lo"
)

// TestSoftDeletePurgeCron_OrphanCalculation 测试软删除清理的孤儿记录计算逻辑
func TestSoftDeletePurgeCron_OrphanCalculation(t *testing.T) {
	t.Parallel()

	t.Run("no soft deleted sessions returns empty orphans", func(t *testing.T) {
		t.Parallel()
		candidateIDs := []uint{}
		activeIDs := []uint{1, 2, 3}
		orphanIDs, _ := lo.Difference(candidateIDs, activeIDs)
		if len(orphanIDs) != 0 {
			t.Fatalf("expected 0 orphans, got %d", len(orphanIDs))
		}
	})

	t.Run("no active sessions means all candidates are orphans", func(t *testing.T) {
		t.Parallel()
		candidateIDs := []uint{10, 20, 30}
		activeIDs := []uint{}
		orphanIDs, _ := lo.Difference(candidateIDs, activeIDs)
		if len(orphanIDs) != 3 {
			t.Fatalf("expected 3 orphans, got %d", len(orphanIDs))
		}
	})

	t.Run("shared messages are excluded from orphans", func(t *testing.T) {
		t.Parallel()
		candidateIDs := []uint{10, 20, 30}
		activeIDs := []uint{20, 30, 40}
		orphanIDs, _ := lo.Difference(candidateIDs, activeIDs)
		if len(orphanIDs) != 1 || orphanIDs[0] != 10 {
			t.Fatalf("expected [10], got %v", orphanIDs)
		}
	})

	t.Run("orphan messages and tools are correctly identified", func(t *testing.T) {
		t.Parallel()
		candidateMsgIDs := []uint{1, 2, 3, 4}
		candidateToolIDs := []uint{10, 20, 30}
		activeMsgIDs := []uint{2, 3}
		activeToolIDs := []uint{20}

		orphanMsgIDs, _ := lo.Difference(candidateMsgIDs, activeMsgIDs)
		orphanToolIDs, _ := lo.Difference(candidateToolIDs, activeToolIDs)

		if len(orphanMsgIDs) != 2 {
			t.Fatalf("expected 2 orphan messages, got %d: %v", len(orphanMsgIDs), orphanMsgIDs)
		}
		if len(orphanToolIDs) != 2 {
			t.Fatalf("expected 2 orphan tools, got %d: %v", len(orphanToolIDs), orphanToolIDs)
		}
	})

	t.Run("deduplication of candidate IDs from multiple sessions", func(t *testing.T) {
		t.Parallel()
		// Simulate: session A has messages [1,2,3], session B has messages [2,3,4]
		allFromSessions := lo.Flatten([][]uint{{1, 2, 3}, {2, 3, 4}})
		candidateIDs := lo.Uniq(allFromSessions)
		if len(candidateIDs) != 4 {
			t.Fatalf("expected 4 unique candidates, got %d: %v", len(candidateIDs), candidateIDs)
		}
	})
}
