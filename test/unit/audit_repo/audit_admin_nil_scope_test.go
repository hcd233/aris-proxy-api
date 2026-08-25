// Package audit_repo 验证审计图表查询的范围守卫语义：
//
//	nil（admin 全量）→ 必须落库执行（不被误短路）
//	空（非 nil）切片（名下无 Key）→ 必须短路返回空结果，不得退化为全量查询
//
// 背景（2026-08-25 越权修复）：空 key 列表曾被 `if len(apiKeyIDs) > 0` 当作
// "不过滤"退化为全量查询。本测试同时守卫相反方向：admin 的 nil 路径若被守卫
// 误短路（如条件写反成 `apiKeyIDs == nil`），nil 将返回空结果而非落库——
// 确保 admin 查看全平台数据（含非自身 owner）的行为不被覆盖。
//
// audit 的 Query* 携带 date_trunc/AT TIME ZONE 等 PG 专属语法，sqlite 无法真实执行；
// "nil 落库 → sqlite 报语法错误"恰好构成"未被短路"的判据。trace/dataset 领域的
// 同构三态（nil 全量 / 空恒假 / 非空 IN）已由可执行 SQL 的行为测试完整覆盖
// （见 trace_owner_scope_test.go 与 session_export_scope_test.go）。
package audit_repo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestQueryMetrics_NilPathExecutesEmptyPathShortCircuits 双向守卫：
// admin 的 nil 必须打到 SQL（哪怕在 sqlite 上因 PG 语法报错），
// 无 Key 用户的空切片必须不打 SQL 干净返回。
func TestQueryMetrics_NilPathExecutesEmptyPathShortCircuits(t *testing.T) {
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
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	queries := map[string]func(apiKeyIDs []uint) error{
		"QueryModelTrend": func(ids []uint) error {
			_, err := repo.QueryModelTrend(ctx, ids, start, end, enum.GranularityHour)
			return err
		},
		"QueryRequestRate": func(ids []uint) error {
			_, err := repo.QueryRequestRate(ctx, ids, start, end, enum.GranularityHour)
			return err
		},
		"QueryTokenThroughput": func(ids []uint) error {
			_, err := repo.QueryTokenThroughput(ctx, ids, start, end, enum.GranularityHour)
			return err
		},
		"QueryFirstTokenLatency": func(ids []uint) error {
			_, err := repo.QueryFirstTokenLatency(ctx, ids, start, end, enum.GranularityHour)
			return err
		},
	}

	for name, query := range queries {
		// nil（admin 全量）：必须落库执行——sqlite 报 PG 专属语法错误即证明未被短路；
		// 若返回空结果且无错误，说明 nil 被守卫误短路，admin 全量行为被覆盖
		err := query(nil)
		if err == nil {
			t.Errorf("%s(nil): expected to reach SQL execution (PG syntax error on sqlite), got nil error — nil path wrongly short-circuited", name)
		} else if !strings.Contains(err.Error(), "syntax error") {
			t.Errorf("%s(nil): unexpected error (want PG syntax error proving SQL reached): %v", name, err)
		}

		// 空（非 nil）切片：必须短路，不打 SQL、干净返回空
		if err := query([]uint{}); err != nil {
			t.Errorf("%s(empty): expected short-circuit with nil error, got: %v", name, err)
		}
	}
}
