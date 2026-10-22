// Package audit_count_cache 验证审计列表的 COUNT(*) 读缓存。
//
// 背景（2026-10-23 压测，BENCHMARK §3.6）：audit/model/log/list 的 COUNT(*) 在
// 82K 行的 model_call_audits 上是 Seq Scan（实测 111ms），而分页 SELECT 走
// idx_mca_created_at 只要 0.17ms——58 req/s 的瓶颈几乎全部来自每请求一次的 COUNT。
// 无 filter / 无 keyword 的默认列表按 (scope, 时间窗分钟桶) 缓存 30s±10s；
// 带 filter/keyword 的查询键基数不可控，直接回源。
package audit_count_cache

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

const auditSeedCount = 25

func newFixture(t *testing.T) (*gorm.DB, *cache.ReadCache) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	for i := range auditSeedCount {
		audit := &dbmodel.ModelCallAudit{
			APIKeyID:  1,
			ModelID:   "gpt-4",
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		}
		if err := db.Create(audit).Error; err != nil {
			t.Fatalf("seed audit %d: %v", i, err)
		}
	}

	mr := miniredis.RunT(t)
	rc := cache.NewReadCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	return db, rc
}

// countSQLCollector 记录 Query/Row 回调里执行过的 SQL，供断言 COUNT 次数。
type countSQLCollector struct {
	sqls []string
}

func (c *countSQLCollector) register(t *testing.T, db *gorm.DB) {
	t.Helper()
	capture := func(tx *gorm.DB) {
		c.sqls = append(c.sqls, tx.Statement.SQL.String())
	}
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_count_query", capture); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register("test:capture_count_row", capture); err != nil {
		t.Fatalf("register row callback: %v", err)
	}
}

func (c *countSQLCollector) countQueries() int {
	n := 0
	for _, sqlText := range c.sqls {
		if strings.Contains(strings.ToLower(sqlText), "count(") {
			n++
		}
	}
	return n
}

// TestListAll_CachesUnfilteredCount 默认列表连续两次请求只允许一次 COUNT。
func TestListAll_CachesUnfilteredCount(t *testing.T) {
	t.Parallel()

	db, rc := newFixture(t)
	collector := &countSQLCollector{}
	collector.register(t, db)

	repo := repository.NewAuditRepository(db, rc)
	param := model.CommonParam{
		PageParam: model.PageParam{Page: 1, PageSize: 20},
		SortParam: model.SortParam{Sort: enum.SortDesc, SortField: "created_at"},
	}

	firstTotal := listTotal(t, repo, param)
	secondTotal := listTotal(t, repo, param)
	if firstTotal != auditSeedCount || secondTotal != auditSeedCount {
		t.Fatalf("total must be %d from both calls, got %d and %d", auditSeedCount, firstTotal, secondTotal)
	}
	if got := collector.countQueries(); got != 1 {
		t.Errorf("unfiltered list must execute COUNT once within TTL, got %d COUNT queries: %v", got, collector.sqls)
	}
	for _, sqlText := range collector.sqls {
		isCount := strings.Contains(strings.ToLower(sqlText), "count(")
		if isCount && strings.Contains(sqlText, "ORDER BY") {
			t.Errorf("COUNT must not carry ORDER BY (it would sort all matched rows): %s", sqlText)
		}
		if !isCount && !strings.Contains(sqlText, "ORDER BY") {
			t.Errorf("page query must keep the requested ORDER BY: %s", sqlText)
		}
	}
}

// TestListAll_KeywordBypassesCountCache 带 keyword 的查询不缓存，每次都要 COUNT。
func TestListAll_KeywordBypassesCountCache(t *testing.T) {
	t.Parallel()

	db, rc := newFixture(t)
	collector := &countSQLCollector{}
	collector.register(t, db)

	repo := repository.NewAuditRepository(db, rc)
	param := model.CommonParam{
		PageParam:  model.PageParam{Page: 1, PageSize: 20},
		QueryParam: model.QueryParam{Query: "gpt-4"},
	}

	for range 2 {
		_ = listTotal(t, repo, param)
	}
	if got := collector.countQueries(); got != 2 {
		t.Errorf("keyword query must not be cached; want 2 COUNT queries, got %d: %v", got, collector.sqls)
	}
}

func listTotal(t *testing.T, repo modelcall.AuditRepository, param model.CommonParam) int64 {
	t.Helper()
	_, pageInfo, err := repo.ListAll(context.Background(), param, time.Time{}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	return pageInfo.Total
}
