// Package dashboard_repo 验证 dashboard 聚合仓储的 SQL 形状、范围守卫与读缓存。
//
// 聚合 SQL 使用 PG 专属语法（FILTER / percentile_cont / date_trunc / AT TIME ZONE），
// sqlite 无法执行；SQL 形状用 PG DryRun 捕获文本断言，可执行性由生产只读 EXPLAIN 验证。
package dashboard_repo

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

var (
	rangeStart = time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	rangeEnd   = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
)

// capturedSQL 线程安全地收集 DryRun 产出的 SQL 与绑定参数
type capturedSQL struct {
	mu   sync.Mutex
	sqls []string
	vars [][]any
}

func (c *capturedSQL) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.sqls...)
}

func (c *capturedSQL) lastVars() []any {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.vars) == 0 {
		return nil
	}
	return c.vars[len(c.vars)-1]
}

// newDryRunDB 构造不连库的 PG DryRun 会话，并注册 SQL 捕获回调
func newDryRunDB(t *testing.T) (*gorm.DB, *capturedSQL) {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 user=guard dbname=guard port=5432 sslmode=disable"), &gorm.Config{
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open dryrun pg db: %v", err)
	}
	captured := &capturedSQL{}
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_dashboard", func(tx *gorm.DB) {
		captured.mu.Lock()
		defer captured.mu.Unlock()
		captured.sqls = append(captured.sqls, tx.Statement.SQL.String())
		captured.vars = append(captured.vars, append([]any(nil), tx.Statement.Vars...))
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true}), captured
}

// newReadCache 构造 miniredis 支撑的读缓存
func newReadCache(t *testing.T) *cache.ReadCache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return cache.NewReadCache(rdb)
}

// assertSQLContains 断言 SQL（小写比较）包含全部片段
func assertSQLContains(t *testing.T, sqlText string, fragments ...string) {
	t.Helper()
	lower := strings.ToLower(sqlText)
	for _, f := range fragments {
		if !strings.Contains(lower, strings.ToLower(f)) {
			t.Errorf("sql missing %q:\n%s", f, sqlText)
		}
	}
}
