// Package audit_repo 验证审计筛选选项查询的 API Key 范围语义：
// 非 nil 空 key 列表必须短路返回空结果，不得退化为无过滤全量查询
// （与 audit_metrics_scope_test.go 同一缺陷模式的选项接口回归）。
package audit_repo

import (
	"context"
	"testing"
	"time"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestListDistinctOptions_EmptyAPIKeyIDsShortCircuitsToEmpty 非 nil 空 key
// 列表必须返回空结果且不产生错误。
func TestListDistinctOptions_EmptyAPIKeyIDsShortCircuitsToEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	seed := []*dbmodel.ModelCallAudit{
		{APIKeyID: 10, ModelID: "gpt-4", UpstreamStatusCode: 200},
		{APIKeyID: 20, ModelID: "claude", UpstreamStatusCode: 500},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	repo := repository.NewAuditRepository(db)
	ctx := context.Background()
	now := time.Now()
	start, end := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	empty := []uint{}

	if items, err := repo.ListDistinctUserNames(ctx, empty, "", start, end); err != nil || len(items) != 0 {
		t.Errorf("ListDistinctUserNames(empty) = (%d items, %v); want (0, nil)", len(items), err)
	}
	if items, err := repo.ListDistinctModels(ctx, empty, "", start, end); err != nil || len(items) != 0 {
		t.Errorf("ListDistinctModels(empty) = (%d items, %v); want (0, nil)", len(items), err)
	}
	if items, err := repo.ListDistinctStatusCodes(ctx, empty, start, end); err != nil || len(items) != 0 {
		t.Errorf("ListDistinctStatusCodes(empty) = (%d items, %v); want (0, nil)", len(items), err)
	}
	if items, err := repo.ListDistinctUserAgents(ctx, empty, "", start, end); err != nil || len(items) != 0 {
		t.Errorf("ListDistinctUserAgents(empty) = (%d items, %v); want (0, nil)", len(items), err)
	}
}

// TestListDistinctOptions_NilAPIKeyIDsQueriesAll nil key 列表（admin/demo
// 全量路径）必须保留全量行为：造 2 行不同 key 数据，全部返回。
func TestListDistinctOptions_NilAPIKeyIDsQueriesAll(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	seed := []*dbmodel.ModelCallAudit{
		{APIKeyID: 10, ModelID: "gpt-4"},
		{APIKeyID: 20, ModelID: "claude"},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	repo := repository.NewAuditRepository(db)
	ctx := context.Background()

	if items, err := repo.ListDistinctModels(ctx, nil, "", time.Time{}, time.Time{}); err != nil || len(items) != 2 {
		t.Errorf("ListDistinctModels(nil) = (%d items, %v); want (2, nil)", len(items), err)
	}
}
