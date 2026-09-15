// Package model_repository 覆盖改名 + 历史同步的乐观锁契约：
// 事务内必须校验「库里的 model_id 仍是调用方读到的旧值」，否则并发改名/行被删除时
// 历史会被替换到错误目标上且重试无法自愈。
package model_repository

import (
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// 并发改名的形态：调用方读到的旧值是 other-id，但库里已经是 old-id（另一个请求先改过）。
func TestUpdateWithHistorySync_RejectsStaleModelID(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	agg := seedModel(t, db, 1, "old-id")
	repo := repository.NewModelRepository(db)

	_, err := repo.UpdateWithHistorySync(t.Context(), agg, "other-id")
	if !errors.Is(err, ierr.ErrResourceLocked) {
		t.Fatalf("err = %v, want ErrResourceLocked", err)
	}

	var row dbmodel.Model
	if err := db.First(&row, agg.AggregateID()).Error; err != nil {
		t.Fatalf("find model row: %v", err)
	}
	if row.ModelID != "old-id" {
		t.Fatalf("model id = %q, want unchanged old-id", row.ModelID)
	}
	var rewritten int64
	db.Model(&dbmodel.ModelCallAudit{}).Where("model_id = ?", "new-id").Count(&rewritten)
	if rewritten != 0 {
		t.Fatalf("history must not be rewritten on conflict, got %d rows", rewritten)
	}
}

// 模型行被并发删除：UPDATE 影响 0 行，事务必须整体放弃而不是照常改写历史。
func TestUpdateWithHistorySync_RejectsMissingRow(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	agg := seedModel(t, db, 1, "old-id")
	if err := db.Delete(&dbmodel.Model{}, agg.AggregateID()).Error; err != nil {
		t.Fatalf("delete model row: %v", err)
	}
	repo := repository.NewModelRepository(db)

	_, err := repo.UpdateWithHistorySync(t.Context(), agg, "old-id")
	if !errors.Is(err, ierr.ErrResourceLocked) {
		t.Fatalf("err = %v, want ErrResourceLocked", err)
	}
	var rewritten int64
	db.Model(&dbmodel.ModelCallAudit{}).Where("model_id = ?", "new-id").Count(&rewritten)
	if rewritten != 0 {
		t.Fatalf("history must not be rewritten when the model row is gone, got %d rows", rewritten)
	}
}
