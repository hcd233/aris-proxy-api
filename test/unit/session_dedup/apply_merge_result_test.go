package session_dedup

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	repository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newApplyTestDB 创建 sqlite 内存库并迁移 sessions 表
func newApplyTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("failed to migrate sqlite db: %v", err)
	}
	return db
}

// seedApplySessions 写入两条 session：id=1 为保留者，id=2 为冗余者
func seedApplySessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	keeper := &dbmodel.Session{MessageIDs: []uint{1, 2, 3}, ToolIDs: []uint{10}}
	keeper.ID = 1
	redundant := &dbmodel.Session{MessageIDs: []uint{1, 2}, ToolIDs: []uint{20}}
	redundant.ID = 2
	for _, s := range []*dbmodel.Session{keeper, redundant} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("failed to seed session %d: %v", s.ID, err)
		}
	}
}

// TestApplyMergeResultCommits 验证 ToolIDs 合并与冗余软删在同一事务内成功提交
//
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func TestApplyMergeResultCommits(t *testing.T) {
	t.Parallel()
	db := newApplyTestDB(t, "apply_merge_commit")
	seedApplySessions(t, db)

	result := repository.MergeResult{
		RedundantIDs: []uint{2},
		MergeMapping: map[uint]map[uint]struct{}{
			1: {10: {}, 20: {}},
		},
	}

	merged, err := repository.ApplyMergeResult(db, dao.GetSessionDAO(), result)
	if err != nil {
		t.Fatalf("ApplyMergeResult() error = %v, want nil", err)
	}
	if merged != 1 {
		t.Errorf("ApplyMergeResult() merged = %d, want 1", merged)
	}

	var keeper dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 1).First(&keeper).Error; err != nil {
		t.Fatalf("failed to reload keeper: %v", err)
	}
	if !slices.Equal(keeper.ToolIDs, []uint{10, 20}) {
		t.Errorf("keeper tool_ids = %v, want [10 20]", keeper.ToolIDs)
	}
	if keeper.DeletedAt != 0 {
		t.Errorf("keeper deleted_at = %d, want 0", keeper.DeletedAt)
	}

	var deleted dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 2).First(&deleted).Error; err != nil {
		t.Fatalf("failed to reload redundant: %v", err)
	}
	if deleted.DeletedAt == 0 {
		t.Error("redundant session deleted_at = 0, want non-zero (soft deleted)")
	}
}

// TestApplyMergeResultRollsBackOnFailure 验证 ToolIDs 更新失败时不会删除冗余 Session。
//
// 这是旧实现的数据丢失路径：更新失败被 continue 吞掉后仍执行 BatchDeleteByField，
// 冗余 session 被删而 ToolIDs 未合并，tool 引用永久丢失。
//
// 失败注入用 GORM callback（driver 无关），不用 DropColumn——sqlite 的 DropColumn
// 会重建表，行为不可靠。
//
// 关键：callback 只拦 tool_ids 的 UPDATE，放过软删的 UPDATE（BatchDeleteByField
// 也走 UPDATE）。若无差别地拦下所有 UPDATE，删除本身也会失败，那么旧的「吞掉错误
// 继续删」实现同样能让本测试通过，测试就失去区分度。
//
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func TestApplyMergeResultRollsBackOnFailure(t *testing.T) {
	t.Parallel()
	db := newApplyTestDB(t, "apply_merge_rollback")
	seedApplySessions(t, db)

	if err := db.Callback().Update().Before("gorm:update").
		Register("test:force_tool_ids_update_error", func(tx *gorm.DB) {
			if tx.Statement == nil {
				return
			}
			dest, ok := tx.Statement.Dest.(map[string]any)
			if !ok {
				return
			}
			if _, updatingToolIDs := dest["tool_ids"]; updatingToolIDs {
				tx.AddError(ierr.New(ierr.ErrInternal, "injected tool_ids update failure"))
			}
		}); err != nil {
		t.Fatalf("failed to register callback: %v", err)
	}

	result := repository.MergeResult{
		RedundantIDs: []uint{2},
		MergeMapping: map[uint]map[uint]struct{}{
			1: {10: {}, 20: {}},
		},
	}

	if _, err := repository.ApplyMergeResult(db, dao.GetSessionDAO(), result); err == nil {
		t.Fatal("ApplyMergeResult() error = nil, want non-nil")
	}

	var redundant dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 2).First(&redundant).Error; err != nil {
		t.Fatalf("failed to reload redundant session: %v", err)
	}
	if redundant.DeletedAt != 0 {
		t.Error("redundant session deleted_at != 0, want 0: it must not be deleted when the tool id update failed")
	}
}
