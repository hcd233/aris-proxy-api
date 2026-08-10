package tool_dedup

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// mustRecordTool 用当前算法构造 Tool 聚合。
func mustRecordTool(t *testing.T, content *vo.UnifiedTool) *aggregate.Tool {
	t.Helper()
	tool, err := aggregate.RecordTool(content, vo.ComputeToolChecksum(content))
	if err != nil {
		t.Fatalf("failed to record tool %q: %v", content.Name, err)
	}
	return tool
}

// TestBatchSaveDedup_DuplicateWithinBatch 验证同批次内重复工具只落一行，
// 且返回的 ID 列表与输入顺序对齐、不含零值。
//
// 零值 ID曾是真实缺陷：插入失败被静默吞掉后，映射缺失会让调用方把0 写进
// sessions.tool_ids，形成悬空引用。
func TestBatchSaveDedup_DuplicateWithinBatch(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewToolRepository(db)

	bash := newTool("Bash", "Execute a bash command")
	read := newTool("Read", "Read a file")
	tools := []*aggregate.Tool{
		mustRecordTool(t, bash),
		mustRecordTool(t, read),
		mustRecordTool(t, bash),
	}

	ids, err := repo.BatchSaveDedup(context.Background(), tools)
	if err != nil {
		t.Fatalf("BatchSaveDedup failed: %v", err)
	}

	if len(ids) != len(tools) {
		t.Fatalf("ids length = %d, want %d", len(ids), len(tools))
	}
	for i, id := range ids {
		if id == 0 {
			t.Errorf("ids[%d] is zero, dedup must never yield a zero ID", i)
		}
	}
	if ids[0] != ids[2] {
		t.Errorf("duplicate tool got different IDs: %d vs %d", ids[0], ids[2])
	}
	if got := countTools(t, db); got != 2 {
		t.Errorf("tools count = %d, want 2 (batch-internal duplicate must collapse)", got)
	}
}

// TestBatchSaveDedup_ReusesExistingRows 验证已存在的工具复用原 ID，不产生新行。
func TestBatchSaveDedup_ReusesExistingRows(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewToolRepository(db)
	ctx := context.Background()

	bash := newTool("Bash", "Execute a bash command")

	first, err := repo.BatchSaveDedup(ctx, []*aggregate.Tool{mustRecordTool(t, bash)})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	second, err := repo.BatchSaveDedup(ctx, []*aggregate.Tool{mustRecordTool(t, bash)})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	if first[0] != second[0] {
		t.Errorf("existing tool got a new ID: %d -> %d", first[0], second[0])
	}
	if got := countTools(t, db); got != 1 {
		t.Errorf("tools count = %d, want 1", got)
	}
}

// TestBatchSaveDedup_DescriptionDistinguishesTools 端到端验证 description 语义变更：
// 同名同参数但描述不同的工具不再被合并成一条记录。
func TestBatchSaveDedup_DescriptionDistinguishesTools(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewToolRepository(db)

	tools := []*aggregate.Tool{
		mustRecordTool(t, newTool("Bash", "Execute a bash command")),
		mustRecordTool(t, newTool("Bash", "Run a shell command")),
	}

	ids, err := repo.BatchSaveDedup(context.Background(), tools)
	if err != nil {
		t.Fatalf("BatchSaveDedup failed: %v", err)
	}

	if ids[0] == ids[1] {
		t.Errorf("tools with different descriptions collapsed into ID %d", ids[0])
	}
	if got := countTools(t, db); got != 2 {
		t.Errorf("tools count = %d, want 2", got)
	}
}
