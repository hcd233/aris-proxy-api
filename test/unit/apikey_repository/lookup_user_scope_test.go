// Package apikey_repository API Key 仓储用户范围查询测试
//
// 回归（2026-09-25 CR P1）：LookupOwnerNamesByUserID / LookupIDsByUserID 在
// userID=0（认证缺失/零值哨兵）时必须返回空列表。旧实现用
// `BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, ...)` 承载过滤，GORM 会把
// struct 零值字段整体丢弃，userID=0 时退化为全表返回（跨租户越权）。
//
//	@author centonhuang
//	@update 2026-09-26 00:00:00
package apikey_repository

import (
	"slices"
	"testing"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLookupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ProxyAPIKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedLookupKeys 种子：user1 两把 key、user2 一把、外加一条 user_id=0 的无主历史 key
// （user_id=0 是合法存量值，等值查询同样不得把它当作「缺失身份」的可见数据）。
func seedLookupKeys(t *testing.T, db *gorm.DB) {
	t.Helper()
	keys := []*dbmodel.ProxyAPIKey{
		{UserID: 1, Name: "u1-key", Key: "k1"},
		{UserID: 1, Name: "u1-key-2", Key: "k2"},
		{UserID: 2, Name: "u2-key", Key: "k3"},
		{UserID: 0, Name: "orphan-key", Key: "k4"},
	}
	if err := db.Create(keys).Error; err != nil {
		t.Fatalf("seed keys: %v", err)
	}
}

// TestLookupOwnerNamesByUserID_ZeroUserReturnsEmpty userID=0 必须返回空名称列表，
// 不得命中任何 key（含 user_id=0 的无主 key）；非零用户只返回自己的 key 名。
func TestLookupOwnerNamesByUserID_ZeroUserReturnsEmpty(t *testing.T) {
	t.Parallel()
	db := newLookupDB(t)
	seedLookupKeys(t, db)
	repo := repository.NewAPIKeyRepository(db)

	names, err := repo.LookupOwnerNamesByUserID(t.Context(), 0)
	if err != nil {
		t.Fatalf("lookup names by user 0: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty for userID=0", names)
	}

	names, err = repo.LookupOwnerNamesByUserID(t.Context(), 1)
	if err != nil {
		t.Fatalf("lookup names by user 1: %v", err)
	}
	want := []string{"u1-key", "u1-key-2"}
	if !slices.Equal(names, want) {
		t.Fatalf("names = %v, want %v (cross-tenant leak or dropped rows)", names, want)
	}
}

// TestLookupIDsByUserID_ZeroUserReturnsEmpty userID=0 必须返回空 ID 列表；
// 非零用户只返回自己的 key ID。
func TestLookupIDsByUserID_ZeroUserReturnsEmpty(t *testing.T) {
	t.Parallel()
	db := newLookupDB(t)
	seedLookupKeys(t, db)
	repo := repository.NewAPIKeyRepository(db)

	ids, err := repo.LookupIDsByUserID(t.Context(), 0)
	if err != nil {
		t.Fatalf("lookup ids by user 0: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("ids = %v, want empty for userID=0", ids)
	}

	ids, err = repo.LookupIDsByUserID(t.Context(), 2)
	if err != nil {
		t.Fatalf("lookup ids by user 2: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("ids = %v, want exactly user 2's single key", ids)
	}
	var row dbmodel.ProxyAPIKey
	if err := db.First(&row, ids[0]).Error; err != nil {
		t.Fatalf("load key %d: %v", ids[0], err)
	}
	if row.UserID != 2 {
		t.Fatalf("key %d belongs to user %d, want 2 (cross-tenant leak)", ids[0], row.UserID)
	}
}
