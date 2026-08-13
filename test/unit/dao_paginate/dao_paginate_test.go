// Package dao_paginate 验证 dao.Paginate 多字段模糊搜索的 OR 条件分组正确性。
//
// 背景（Critical，2026-08-12 review 发现）：
//   - 旧实现 `sql.Where(expressions[0]); for ... sql.Or(expr)` 生成
//     `WHERE permission = ? AND deleted_at = 0 AND name LIKE ? OR email LIKE ?`，
//     实际语义为 `(permission AND deleted_at AND name) OR email`，
//     email 分支不携带权限与软删约束，导致被软删用户/非目标权限用户混入搜索结果。
//   - 修复后 `sql.Where(clause.Or(expressions...))` 生成
//     `... AND (name LIKE ? OR email LIKE ?)`，权限与软删约束对 OR 全分支生效。
package dao_paginate

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
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

// TestPaginateSearchDoesNotLeakSoftDeletedOrOtherPermission
// 验证：permission=pending + query 命中 email 时，
// 被软删用户与其他权限用户不得混入结果（OR 分支必须受权限与软删约束）。
func TestPaginateSearchDoesNotLeakSoftDeletedOrOtherPermission(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.User{}); err != nil {
		t.Fatalf("failed to migrate users: %v", err)
	}

	// 三条记录：email 均含 "alice" 关键字，但权限/软删状态不同
	// （github/google bind id 必须互不相同，避免 sqlite 唯一索引冲突）
	records := []*dbmodel.User{
		{Name: "alice", Email: "alice@example.com", Permission: enum.PermissionPending, GithubBindID: "a1", GoogleBindID: "g1"},
		{Name: "alice-bad", Email: "alice-bad@example.com", Permission: enum.PermissionPending, DeletedAt: 1, GithubBindID: "a2", GoogleBindID: "g2"}, // 软删
		{Name: "alice-admin", Email: "alice-admin@example.com", Permission: enum.PermissionAdmin, GithubBindID: "a3", GoogleBindID: "g3"},             // 非目标权限
	}
	for _, r := range records {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("failed to create user %s: %v", r.Name, err)
		}
	}

	data, pageInfo, err := dao.GetUserDAO().Paginate(
		db,
		&dbmodel.User{Permission: enum.PermissionPending},
		constant.UserRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: 1, PageSize: 20},
			QueryParam: dao.QueryParam{Query: "alice", QueryFields: []string{constant.FieldName, constant.FieldEmail}},
			SortParam:  dao.SortParam{Sort: enum.SortAsc, SortField: constant.FieldID},
		},
	)
	if err != nil {
		t.Fatalf("Paginate failed: %v", err)
	}
	if pageInfo.Total != 1 {
		t.Fatalf("expected total=1 (only pending+alive alice), got %d", pageInfo.Total)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(data), data)
	}
	if data[0].Name != "alice" {
		t.Fatalf("expected alice, got %s", data[0].Name)
	}
}

// TestPaginateNoQueryKeepsFilters 无 query 时权限+软删过滤保持（回归护栏）。
func TestPaginateNoQueryKeepsFilters(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.User{}); err != nil {
		t.Fatalf("failed to migrate users: %v", err)
	}

	records := []*dbmodel.User{
		{Name: "u1", Email: "u1@example.com", Permission: enum.PermissionPending, GithubBindID: "b1", GoogleBindID: "g4"},
		{Name: "u2", Email: "u2@example.com", Permission: enum.PermissionAdmin, GithubBindID: "b2", GoogleBindID: "g5"},
	}
	for _, r := range records {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("failed to create user %s: %v", r.Name, err)
		}
	}

	data, pageInfo, err := dao.GetUserDAO().Paginate(
		db,
		&dbmodel.User{Permission: enum.PermissionPending},
		constant.UserRepoFieldsFull,
		&dao.CommonParam{
			PageParam: dao.PageParam{Page: 1, PageSize: 20},
			SortParam: dao.SortParam{Sort: enum.SortAsc, SortField: constant.FieldID},
		},
	)
	if err != nil {
		t.Fatalf("Paginate failed: %v", err)
	}
	if pageInfo.Total != 1 || len(data) != 1 || data[0].Name != "u1" {
		t.Fatalf("expected only u1 (pending), got total=%d data=%+v", pageInfo.Total, data)
	}
}
