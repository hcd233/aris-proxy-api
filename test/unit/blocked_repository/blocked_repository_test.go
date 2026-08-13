// Package blocked_repository 验证 blockedRepository.UpdateAction 的存在性检查、
// 软删过滤与 updated_at 维护。
//
// 背景（Major，2026-08-12 review 发现）：
//   - 旧实现 `UpdateColumn` 不检查 RowsAffected、不带 deleted_at=0 过滤，
//     PATCH 不存在的 id 返回 200 静默成功；已软删词条可被悄悄改 action。
//   - 修复：加存在性检查（RowsAffected==0 → ErrDataNotExists）+ 软删过滤 +
//     updated_at 显式更新。
package blocked_repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db
}

func mustCreate(t *testing.T, db *gorm.DB, word string) uint {
	t.Helper()
	m := &dbmodel.Blocked{Word: word, Action: "deny"}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create blocked word failed: %v", err)
	}
	return m.ID
}

// TestUpdateAction_Success 更新存在的未软删词条成功，且 updated_at 被刷新。
func TestUpdateAction_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Blocked{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	id := mustCreate(t, db, "foo")
	repo := repository.NewBlockedRepository(db)

	if err := repo.UpdateAction(context.Background(), id, "omit"); err != nil {
		t.Fatalf("UpdateAction failed: %v", err)
	}
	var got dbmodel.Blocked
	if err := db.First(&got, id).Error; err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if got.Action != "omit" {
		t.Fatalf("expected action omit, got %s", got.Action)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be refreshed")
	}
}

// TestUpdateAction_NotExists 更新不存在的 id 必须返回 ErrDataNotExists（非静默成功）。
func TestUpdateAction_NotExists(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Blocked{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	repo := repository.NewBlockedRepository(db)

	err := repo.UpdateAction(context.Background(), 9999, "omit")
	if err == nil {
		t.Fatal("expected error for non-existent id, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}

// TestUpdateAction_SoftDeletedRejected 已软删词条不能被更新（deleted_at=0 过滤）。
func TestUpdateAction_SoftDeletedRejected(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Blocked{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	id := mustCreate(t, db, "foo")
	if err := db.Model(&dbmodel.Blocked{}).Where("id = ?", id).Update("deleted_at", 1).Error; err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}
	repo := repository.NewBlockedRepository(db)

	err := repo.UpdateAction(context.Background(), id, "omit")
	if err == nil {
		t.Fatal("expected error for soft-deleted id, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
	// 软删行 action 必须保持原值
	var got dbmodel.Blocked
	if err := db.Unscoped().First(&got, id).Error; err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if got.Action != "deny" {
		t.Fatalf("soft-deleted row action must not change, got %s", got.Action)
	}
}
