// Package audit_repo 验证 BatchGetRelations 对已软删除 API Key / 用户的名称回填：
// 审计记录是历史事实，Key 或用户被删除后仍须解析出原名，否则 dashboard Top 排行
// 会退化为显示 api_key_id，或把归属聚进 "0" 兜底桶。
package audit_repo

import (
	"testing"

	"github.com/hcd233/aris-mass/internal/common/enum"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestBatchGetRelations_IncludesSoftDeletedKeyAndUser 已软删的 Key 与其所属用户
// 仍须回填名称，否则 Top 排行会把历史记录归入 "0" 桶并显示主键数字。
func TestBatchGetRelations_IncludesSoftDeletedKeyAndUser(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.ProxyAPIKey{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	users := []*dbmodel.User{
		{Name: "alive-user", Email: "alive@example.com", Permission: enum.PermissionUser, GithubBindID: "gh-alive", GoogleBindID: "gg-alive"},
		{Name: "deleted-user", Email: "deleted@example.com", Permission: enum.PermissionUser, GithubBindID: "gh-deleted", GoogleBindID: "gg-deleted", DeletedAt: 1792513575},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users failed: %v", err)
	}

	keys := []*dbmodel.ProxyAPIKey{
		{UserID: users[0].ID, Name: "live-key", Key: "sk-live"},
		{UserID: users[1].ID, Name: "deleted-key", Key: "sk-deleted", DeletedAt: 1792513575},
	}
	if err := db.Create(&keys).Error; err != nil {
		t.Fatalf("seed api keys failed: %v", err)
	}

	repo := repository.NewAuditRepository(db, nil)
	relations, err := repo.BatchGetRelations(t.Context(), []uint{keys[0].ID, keys[1].ID})
	if err != nil {
		t.Fatalf("BatchGetRelations failed: %v", err)
	}

	cases := []struct {
		name         string
		keyID        uint
		expectedKey  string
		expectedUser string
	}{
		{name: "alive key", keyID: keys[0].ID, expectedKey: "live-key", expectedUser: "alive-user"},
		{name: "soft deleted key", keyID: keys[1].ID, expectedKey: "deleted-key", expectedUser: "deleted-user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rel, ok := relations[tc.keyID]
			if !ok || rel == nil {
				t.Fatalf("relation for key %d missing", tc.keyID)
			}
			if rel.APIKeyName != tc.expectedKey {
				t.Errorf("APIKeyName = %q; want %q", rel.APIKeyName, tc.expectedKey)
			}
			if rel.UserName != tc.expectedUser {
				t.Errorf("UserName = %q; want %q", rel.UserName, tc.expectedUser)
			}
		})
	}
}
