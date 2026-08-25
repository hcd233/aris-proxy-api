// Package audit_repo 验证审计图表查询的 API Key 范围语义：
// 非 nil 空 key 列表必须短路返回空结果，不得退化为无过滤全量查询。
//
// 背景（2026-08-25 越权修复）：旧实现用 `if len(apiKeyIDs) > 0` 决定是否加
// api_key_id 过滤，用户名下无 API Key 时（LookupIDsByUserID 返回空 slice）
// 过滤被整体跳过，普通用户可越权查看全平台指标聚合图。
//
// sqlite 无 date_trunc/FILTER 函数，若空列表未短路会直接报 SQL 错误，
// 恰好可作为"未触达 SQL"的判据；nil（admin 全量）路径依赖 PG 语法，不在本测试范围。
package audit_repo

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestQueryMetrics_EmptyAPIKeyIDsShortCircuitsToEmpty 非 nil 空 key 列表必须
// 返回空结果且不产生错误（修复前会落入 date_trunc SQL 报错 / PG 上返回全量）。
func TestQueryMetrics_EmptyAPIKeyIDsShortCircuitsToEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// 造 2 行不同 key 的审计记录：若空列表未短路，这些数据会被无过滤查回
	seed := []*dbmodel.ModelCallAudit{
		{APIKeyID: 10, ModelID: "gpt-4"},
		{APIKeyID: 20, ModelID: "claude"},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	repo := repository.NewAuditRepository(db)
	ctx := context.Background()
	now := time.Now()
	start, end := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	empty := []uint{}

	if pts, err := repo.QueryModelTrend(ctx, empty, start, end, enum.GranularityHour); err != nil || len(pts) != 0 {
		t.Errorf("QueryModelTrend(empty) = (%d pts, %v); want (0, nil)", len(pts), err)
	}
	if pts, err := repo.QueryRequestRate(ctx, empty, start, end, enum.GranularityHour); err != nil || len(pts) != 0 {
		t.Errorf("QueryRequestRate(empty) = (%d pts, %v); want (0, nil)", len(pts), err)
	}
	if pts, err := repo.QueryTokenThroughput(ctx, empty, start, end, enum.GranularityHour); err != nil || len(pts) != 0 {
		t.Errorf("QueryTokenThroughput(empty) = (%d pts, %v); want (0, nil)", len(pts), err)
	}
	if pts, err := repo.QueryFirstTokenLatency(ctx, empty, start, end, enum.GranularityHour); err != nil || len(pts) != 0 {
		t.Errorf("QueryFirstTokenLatency(empty) = (%d pts, %v); want (0, nil)", len(pts), err)
	}
}
