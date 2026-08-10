package tool_dedup

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
)

// TestBackfill_OneToOneUpdate 验证回填是一对一 UPDATE：
// 每条存量记录的 checksum 被改写为当前算法结果，行数与主键均不变。
//
// 这是"无需 remap sessions.tool_ids"的前提：新算法输入是旧算法输入的超集，
// 原本互不相同的 checksum 重算后仍互不相同，不会产生需要合并的记录。
func TestBackfill_OneToOneUpdate(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	tools := []*vo.UnifiedTool{
		newTool("Bash", "Execute a bash command"),
		newTool("Read", "Read a file"),
		newTool("Edit", "Edit a file"),
	}
	records := make([]uint, 0, len(tools))
	for _, tool := range tools {
		records = append(records, insertTool(t, db, tool, legacyChecksum(tool)).ID)
	}

	if err := database.BackfillToolChecksumsWithDB(db); err != nil {
		t.Fatalf("backfill failed: %v", err)
	}

	if got := countTools(t, db); got != int64(len(tools)) {
		t.Errorf("tools count = %d, want %d (backfill must not insert or delete rows)", got, len(tools))
	}

	seen := map[string]bool{}
	for i, id := range records {
		want := vo.ComputeToolChecksum(tools[i])
		got := findTool(t, db, id).CheckSum
		if got != want {
			t.Errorf("tool %q checksum = %s, want %s", tools[i].Name, got, want)
		}
		if seen[got] {
			t.Errorf("tool %q checksum %s collided with an earlier row", tools[i].Name, got)
		}
		seen[got] = true
	}
}

// TestBackfill_Idempotent 验证回填幂等：第二次执行不再写库。
//
// 用 updated_at 是否变化来判定，避免为测试暴露内部统计。
func TestBackfill_Idempotent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	tool := newTool("Bash", "Execute a bash command")
	id := insertTool(t, db, tool, legacyChecksum(tool)).ID

	if err := database.BackfillToolChecksumsWithDB(db); err != nil {
		t.Fatalf("first backfill failed: %v", err)
	}
	afterFirst := findTool(t, db, id)

	if err := database.BackfillToolChecksumsWithDB(db); err != nil {
		t.Fatalf("second backfill failed: %v", err)
	}
	afterSecond := findTool(t, db, id)

	if afterSecond.CheckSum != afterFirst.CheckSum {
		t.Errorf("checksum changed on second run: %s -> %s", afterFirst.CheckSum, afterSecond.CheckSum)
	}
	if !afterSecond.UpdatedAt.Equal(afterFirst.UpdatedAt) {
		t.Errorf("second run rewrote the row: updated_at %s -> %s", afterFirst.UpdatedAt, afterSecond.UpdatedAt)
	}
}

// TestBackfill_ConflictSkipped 验证回填遇唯一冲突时保留旧行并继续。
//
// 场景：新版本先按新算法写入了记录（B），随后才执行回填，存量行（A）重算后与 B 撞车。
// 期望回填不报错、A 保持旧 checksum 成为无害孤儿、B 不受影响。
func TestBackfill_ConflictSkipped(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	tool := newTool("Bash", "Execute a bash command")
	current := vo.ComputeToolChecksum(tool)
	legacy := legacyChecksum(tool)

	stale := insertTool(t, db, tool, legacy)
	fresh := insertTool(t, db, tool, current)

	if err := database.BackfillToolChecksumsWithDB(db); err != nil {
		t.Fatalf("backfill should tolerate unique conflict, got: %v", err)
	}

	if got := findTool(t, db, stale.ID).CheckSum; got != legacy {
		t.Errorf("conflicting row checksum = %s, want unchanged %s", got, legacy)
	}
	if got := findTool(t, db, fresh.ID).CheckSum; got != current {
		t.Errorf("existing row checksum = %s, want unchanged %s", got, current)
	}
	if got := countTools(t, db); got != 2 {
		t.Errorf("tools count = %d, want 2", got)
	}
}
