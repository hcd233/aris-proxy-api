// Package session_list_split 钉住 session 列表「COUNT 与分页分离」的 SQL 形态。
//
// 背景（2026-10-23 压测，BENCHMARK §3.6）：session/list 在 11.3K 会话、约 180K 消息上
// p50 3.7s、13 req/s。EXPLAIN 显示旧的 COUNT(*) OVER () 投影必须对 WHERE 命中的
// 全部 3225 行求值 jsonb_array_length（每行一次 TOAST 解压）后再 top-N 排序：
//
//	Seq Scan 3225 rows → WindowAgg → Sort → Limit   Execution 119.6ms
//
// 拆分后独立 COUNT(*) 走 idx_sessions_first_msg 部分索引 index-only scan（1.0ms），
// 分页 SELECT 走 sessions_pkey 只解压 20 行（4.1ms）。
//
// 本测试只钉 SQL 形态（PG 专有 ::jsonb 语法，sqlite 无法执行），行为由 e2e 覆盖。
package session_list_split

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// newDryRunDB 打开 postgres DryRun 会话：只构建 SQL 不执行（不建连接）。
func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 user=guard dbname=guard port=5432 sslmode=disable"), &gorm.Config{
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open dryrun pg db: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

// captureSQL 在 Query 回调完成后收集本次执行的 SQL。
func captureSQL(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()
	sqls := make([]string, 0, 2)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_session_list_sql", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	return &sqls
}

func defaultParam() model.CommonParam {
	return model.CommonParam{
		PageParam: model.PageParam{Page: 1, PageSize: 20},
		SortParam: model.SortParam{Sort: enum.SortAsc, SortField: "id"},
	}
}

func assertNoWindowCount(t *testing.T, sqlText string) {
	t.Helper()
	if strings.Contains(sqlText, "COUNT(*) OVER") || strings.Contains(sqlText, "total_count") {
		t.Errorf("SQL must not use COUNT(*) OVER () (forces full scan + jsonb detoast), got: %s", sqlText)
	}
}

// TestListAllSessions_SeparatesCountFromPage admin 全量路径：必须产生两条 SQL，
// COUNT 不带 jsonb 表达式，分页 SELECT 不带窗口函数且带 LIMIT。
func TestListAllSessions_SeparatesCountFromPage(t *testing.T) {
	t.Parallel()

	dry := newDryRunDB(t)
	sqls := captureSQL(t, dry)

	repo := repository.NewSessionReadRepository(dry, nil)
	_, pageInfo, err := repo.ListAllSessions(context.Background(), defaultParam(), time.Time{}, time.Time{}, "", nil)
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}
	if pageInfo == nil {
		t.Fatal("pageInfo must not be nil")
	}
	if len(*sqls) != 2 {
		t.Fatalf("want 2 SQL statements (COUNT + page), got %d: %v", len(*sqls), *sqls)
	}

	countSQL, pageSQL := (*sqls)[0], (*sqls)[1]
	if !strings.Contains(strings.ToLower(countSQL), "count(*)") {
		t.Errorf("first SQL must be the COUNT query, got: %s", countSQL)
	}
	if strings.Contains(countSQL, "jsonb_array_length") {
		t.Errorf("COUNT query must not compute jsonb_array_length, got: %s", countSQL)
	}
	assertNoWindowCount(t, countSQL)

	if !strings.Contains(pageSQL, "jsonb_array_length") {
		t.Errorf("page SQL must compute message/tool counts for the page, got: %s", pageSQL)
	}
	if !strings.Contains(pageSQL, "LIMIT") {
		t.Errorf("page SQL must be bounded by LIMIT, got: %s", pageSQL)
	}
	assertNoWindowCount(t, pageSQL)
}

// TestListSessionsByOwnerIDs_CountKeepsScope 租户路径：COUNT 与分页都必须保留
// api_key_id IN (...) 条件，拆分不能丢掉租户隔离。
func TestListSessionsByOwnerIDs_CountKeepsScope(t *testing.T) {
	t.Parallel()

	dry := newDryRunDB(t)
	sqls := captureSQL(t, dry)

	repo := repository.NewSessionReadRepository(dry, nil)
	_, _, err := repo.ListSessionsByOwnerIDs(context.Background(), []uint{7, 9}, defaultParam(), time.Time{}, time.Time{}, "", nil)
	if err != nil {
		t.Fatalf("ListSessionsByOwnerIDs failed: %v", err)
	}
	if len(*sqls) != 2 {
		t.Fatalf("want 2 SQL statements (COUNT + page), got %d: %v", len(*sqls), *sqls)
	}

	for _, sqlText := range *sqls {
		if !strings.Contains(sqlText, "api_key_id") || !strings.Contains(sqlText, "IN (") {
			t.Errorf("both COUNT and page SQL must keep api_key_id scope, got: %s", sqlText)
		}
		assertNoWindowCount(t, sqlText)
	}
}
