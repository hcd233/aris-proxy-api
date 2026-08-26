// Package migration_user_scope 验证多租户化存量数据回填与唯一索引重建。
//
// 背景（feature/user-level-model-endpoint-multitenancy）：
//   - endpoints/models 新增 user_id 列后，存量行需回填到主 admin（permission=admin 中 ID 最小者）；
//   - GORM AutoMigrate 不会改已有同名索引的列组合，必须 DROP+CREATE 重建复合唯一索引；
//   - 迁移必须幂等：只回填 user_id=0 的行，索引用 IF EXISTS / IF NOT EXISTS 守卫。
package migration_user_scope

import (
	"context"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
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

func migrateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
}

func mustCreate(t *testing.T, db *gorm.DB, row any) {
	t.Helper()
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create %+v failed: %v", row, err)
	}
}

// TestMigrateUserScopeData_BackfillAndIdempotent 回填主 admin、幂等、不覆盖已有归属、无 admin 时报错。
func TestMigrateUserScopeData_BackfillAndIdempotent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	migrateAll(t, db)

	admin1 := &dbmodel.User{Name: "old-admin", Permission: enum.PermissionAdmin, GithubBindID: "gh-1", GoogleBindID: "gg-1"}
	admin2 := &dbmodel.User{Name: "newer-admin", Permission: enum.PermissionAdmin, GithubBindID: "gh-2", GoogleBindID: "gg-2"}
	mustCreate(t, db, admin1)
	mustCreate(t, db, admin2)

	epA := &dbmodel.Endpoint{Name: "ep-a"}
	epB := &dbmodel.Endpoint{Name: "ep-b"}
	mdlA := &dbmodel.Model{Alias: "m-a", UpstreamModel: "up-a"}
	for _, ep := range []*dbmodel.Endpoint{epA, epB} {
		mustCreate(t, db, ep)
	}
	mustCreate(t, db, mdlA)
	preOwned := &dbmodel.Endpoint{Name: "ep-owned", UserID: 999}
	mustCreate(t, db, preOwned)

	// 幂等性验证：连跑两次
	for i := 0; i < 2; i++ {
		if err := database.MigrateUserScopeDataWith(context.Background(), db); err != nil {
			t.Fatalf("round %d failed: %v", i+1, err)
		}
	}

	var gotEp dbmodel.Endpoint
	if err := db.Where("name = ?", "ep-a").First(&gotEp).Error; err != nil {
		t.Fatal(err)
	}
	if gotEp.UserID != admin1.ID {
		t.Fatalf("endpoint user_id = %d, want %d (min-ID admin)", gotEp.UserID, admin1.ID)
	}
	var epOwned dbmodel.Endpoint
	if err := db.Where("name = ?", "ep-owned").First(&epOwned).Error; err != nil {
		t.Fatal(err)
	}
	if epOwned.UserID != 999 {
		t.Fatalf("pre-owned endpoint overwritten: user_id = %d", epOwned.UserID)
	}

	var cnt int64
	db.Model(&dbmodel.Model{}).Where("alias = ?", "m-a").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("model rows = %d, want 1", cnt)
	}
	var mdlA2 dbmodel.Model
	if err := db.Where("alias = ?", "m-a").First(&mdlA2).Error; err != nil {
		t.Fatal(err)
	}
	if mdlA2.UserID != admin1.ID {
		t.Fatalf("model user_id = %d, want %d", mdlA2.UserID, admin1.ID)
	}

	// 无 admin 用户时报错而非静默成功
	empty := newTestDBNamed(t, "empty_admin")
	migrateAll(t, empty)
	if err := database.MigrateUserScopeDataWith(context.Background(), empty); err == nil {
		t.Fatal("expected error when no admin user exists")
	}
}

// TestMigrateUserScopeData_RebuildsUniqueIndex 验证迁移后唯一索引包含 user_id 列且同名冲突跨用户合法。
func TestMigrateUserScopeData_RebuildsUniqueIndex(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	migrateAll(t, db)

	admin := &dbmodel.User{Name: "admin", Permission: enum.PermissionAdmin}
	mustCreate(t, db, admin)

	// 同名端点分属两个用户——旧全局唯一索引会拒绝，新索引应允许
	u101 := &dbmodel.User{Name: "u101", GithubBindID: "gh-101", GoogleBindID: "gg-101"}
	u202 := &dbmodel.User{Name: "u202", GithubBindID: "gh-202", GoogleBindID: "gg-202"}
	mustCreate(t, db, u101)
	mustCreate(t, db, u202)
	mustCreate(t, db, &dbmodel.Endpoint{UserID: u101.ID, Name: "ep-shared"})
	mustCreate(t, db, &dbmodel.Endpoint{UserID: u202.ID, Name: "ep-shared"})

	if err := database.MigrateUserScopeDataWith(context.Background(), db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	idxSQL := map[string]string{}
	rows, err := db.Raw("SELECT name, sql FROM sqlite_master WHERE type='index' AND name IN ('idx_endpoint_name_deleted','idx_model_alias_endpoint_deleted')").Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatal(err)
		}
		idxSQL[name] = sql
	}
	for _, idx := range []string{"idx_endpoint_name_deleted", "idx_model_alias_endpoint_deleted"} {
		sql, ok := idxSQL[idx]
		if !ok {
			t.Fatalf("index %s not found after rebuild", idx)
		}
		if !strings.Contains(sql, "user_id") {
			t.Errorf("index %s missing user_id column: %s", idx, sql)
		}
	}
}

// newTestDBNamed 以显式名字建库，避免 shared-cache 同名库复用。
func newTestDBNamed(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db
}
