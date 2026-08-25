// Package session_export_repository 验证导出链路（ListSessionsForExport / PreviewExport）
// 的 owner 范围语义：非 nil 空 OwnerNames 必须返回空结果，不得退化为无过滤全量查询。
//
// 背景（2026-08-25 越权修复，最高危）：旧实现 applyExportFilter 用
// `if len(f.OwnerNames) > 0` 决定是否加 api_key_name 过滤，用户名下无 API Key 时
// 过滤被整体跳过——普通用户可经数据集导出/统计预览/格式预览三个接口
// 越权导出全平台所有用户的完整会话内容。
//
// PreviewExport 与 ListSessionsForExport 共用 applyExportFilter，本测试仅真实执行
// 后者（前者的 SELECT 含 jsonb_array_elements_text 等 PG 专属语法，sqlite 无法解析）。
package session_export_repository

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	seed := []*dbmodel.Session{
		{APIKeyName: "owner-a", ModelIDs: []string{"gpt-4"}},
		{APIKeyName: "owner-b", ModelIDs: []string{"gpt-4"}},
		{APIKeyName: "owner-b", ModelIDs: []string{"claude"}},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return db
}

// TestListSessionsForExport_OwnerScopeSemantics 三态语义：
// nil（admin 全量）/ 空 slice（名下无 Key，必须空）/ 具体值（按 owner 过滤）。
func TestListSessionsForExport_OwnerScopeSemantics(t *testing.T) {
	t.Parallel()

	db := newExportTestDB(t)
	repo := repository.NewSessionReadRepository(db)
	ctx := context.Background()

	// 空（非 nil）OwnerNames：用户名下无 Key，必须返回空（不得导出全平台会话）
	rows, err := repo.ListSessionsForExport(ctx, session.ExportFilter{OwnerNames: []string{}})
	if err != nil {
		t.Fatalf("ListSessionsForExport(empty) err: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("ListSessionsForExport(empty) = %d rows; want 0", len(rows))
	}

	// 具体 owner：只返回该 owner 名下的
	rows, err = repo.ListSessionsForExport(ctx, session.ExportFilter{OwnerNames: []string{"owner-b"}})
	if err != nil {
		t.Fatalf("ListSessionsForExport(owner-b) err: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("ListSessionsForExport(owner-b) = %d rows; want 2", len(rows))
	}

	// nil：admin 全量
	rows, err = repo.ListSessionsForExport(ctx, session.ExportFilter{})
	if err != nil {
		t.Fatalf("ListSessionsForExport(nil) err: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("ListSessionsForExport(nil) = %d rows; want 3", len(rows))
	}
}
