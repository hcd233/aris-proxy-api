// Package trace_repository 验证 PaginateByOwners 的 owner 范围语义：
// 非 nil 空 owner 列表必须返回空结果，不得退化为无过滤全量查询。
//
// 背景（2026-08-25 越权修复）：旧实现用 `if len(owners) > 0` 决定是否加
// api_key_name 过滤，用户名下无 API Key 时（LookupOwnerNamesByUserID 返回空 slice）
// 过滤被整体跳过，普通用户可越权查看全平台 trace 列表。
package trace_repository

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestPaginateByOwners_OwnerScopeSemantics 三态语义：
// nil（admin 全量）/ 空 slice（名下无 Key，必须空）/ 具体值（按 owner 过滤）。
func TestPaginateByOwners_OwnerScopeSemantics(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Trace{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	seed := []*dbmodel.Trace{
		{SessionID: "s-1", APIKeyName: "owner-a", Agent: "codex"},
		{SessionID: "s-2", APIKeyName: "owner-b", Agent: "codex"},
		{SessionID: "s-3", APIKeyName: "owner-b", Agent: "claude"},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	repo := repository.NewTraceRepository(db)
	ctx := context.Background()
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}}

	// 空（非 nil）owner 列表：用户名下无 Key，必须返回空
	traces, pageInfo, err := repo.PaginateByOwners(ctx, []string{}, param)
	if err != nil {
		t.Fatalf("PaginateByOwners(empty) err: %v", err)
	}
	if len(traces) != 0 || pageInfo.Total != 0 {
		t.Errorf("PaginateByOwners(empty) = (%d traces, total %d); want (0, 0)", len(traces), pageInfo.Total)
	}

	// 具体 owner：只返回该 owner 名下的
	traces, pageInfo, err = repo.PaginateByOwners(ctx, []string{"owner-b"}, param)
	if err != nil {
		t.Fatalf("PaginateByOwners(owner-b) err: %v", err)
	}
	if len(traces) != 2 || pageInfo.Total != 2 {
		t.Errorf("PaginateByOwners(owner-b) = (%d traces, total %d); want (2, 2)", len(traces), pageInfo.Total)
	}

	// nil：admin 全量
	traces, pageInfo, err = repo.PaginateByOwners(ctx, nil, param)
	if err != nil {
		t.Fatalf("PaginateByOwners(nil) err: %v", err)
	}
	if len(traces) != 3 || pageInfo.Total != 3 {
		t.Errorf("PaginateByOwners(nil) = (%d traces, total %d); want (3, 3)", len(traces), pageInfo.Total)
	}
}
