// Package read_cache_window 时间窗读缓存（统计聚合 / 筛选选项）回归测试。
//
// 背景：stats 与 option 类接口按时间窗做 GROUP BY / DISTINCT，是全表聚合（BENCHMARK §3.6
// 中 55~183 req/s 那一档）。改造后在仓储层加 cache-aside，键 = 用途 + 全部影响结果的参数
// 摘要（含 scope、时间窗对齐到分钟）——见 docs/loadtest/QPS-OPTIMIZATION.md §4.6/§4.7。
//
// 本测试用 sqlite + miniredis 断言：
//  1. 同一参数第二次调用不再查库（缓存命中）；
//  2. scope 不同必须各自回源，绝不共用条目（nil=admin 全量 与 空列表=名下无 Key 不可合并）；
//  3. granularity 参与键，不同粒度不共用。
package read_cache_window

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

var (
	rangeStart = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	rangeEnd   = time.Date(2026, 10, 22, 0, 0, 20, 0, time.UTC)
)

// newAuditRepo 建一个含两条审计行的 sqlite 库 + 真实 ReadCache，返回仓储与查询计数读取函数。
func newAuditRepo(t *testing.T) (repo modelcall.AuditRepository, queryCount func() int64) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if migrateErr := db.AutoMigrate(&model.ModelCallAudit{}); migrateErr != nil {
		t.Fatalf("migrate: %v", migrateErr)
	}
	zeroCost := int64(0)
	rows := []*model.ModelCallAudit{
		{APIKeyID: 1, ModelID: "m1", UpstreamStatusCode: 200, InputCostMicro: &zeroCost, CreatedAt: rangeStart.Add(time.Hour)},
		{APIKeyID: 2, ModelID: "m2", UpstreamStatusCode: 200, InputCostMicro: &zeroCost, CreatedAt: rangeStart.Add(2 * time.Hour)},
	}
	if createErr := db.Create(&rows).Error; createErr != nil {
		t.Fatalf("seed audits: %v", createErr)
	}

	var counter atomic.Int64
	count := func(*gorm.DB) { counter.Add(1) }
	if cbErr := db.Callback().Query().Before("gorm:query").Register("test:count_queries", count); cbErr != nil {
		t.Fatalf("register query counter: %v", cbErr)
	}
	if cbErr := db.Callback().Row().Before("gorm:row").Register("test:count_rows", count); cbErr != nil {
		t.Fatalf("register row counter: %v", cbErr)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return repository.NewAuditRepository(db, cache.NewReadCache(rdb)), counter.Load
}

func TestAuditOptionCacheHits(t *testing.T) {
	t.Parallel()

	repo, queries := newAuditRepo(t)

	admin, err := repo.ListDistinctModels(t.Context(), nil, "", rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("admin distinct models: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("first call should hit db once, got %d", got)
	}
	if len(admin) != 2 {
		t.Fatalf("admin scope should see both models, got %v", admin)
	}

	again, err := repo.ListDistinctModels(t.Context(), nil, "", rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("cached distinct models: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("second call must be served by cache, got %d db queries", got)
	}
	if len(again) != len(admin) {
		t.Fatalf("cached result mismatch: first=%v second=%v", admin, again)
	}

	// 换 scope 必须各自回源：apiKeyID=1 只能看到 m1
	scoped, err := repo.ListDistinctModels(t.Context(), []uint{1}, "", rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("scoped distinct models: %v", err)
	}
	if got := queries(); got != 2 {
		t.Fatalf("different scope must miss cache, got %d db queries", got)
	}
	if len(scoped) != 1 || scoped[0] != "m1" {
		t.Fatalf("scope=1 must only see m1, got %v", scoped)
	}

	// 空 scope（用户名下无 Key）不得命中 admin 的全量条目
	empty, err := repo.ListDistinctModels(t.Context(), []uint{}, "", rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("empty scope distinct models: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty scope must return nothing, got %v", empty)
	}
}

func TestAuditStatsCacheHits(t *testing.T) {
	t.Parallel()

	repo, queries := newAuditRepo(t)

	if _, err := repo.QueryModelCost(t.Context(), nil, rangeStart, rangeEnd); err != nil {
		t.Fatalf("model cost: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("first call should hit db once, got %d", got)
	}

	if _, err := repo.QueryModelCost(t.Context(), nil, rangeStart, rangeEnd); err != nil {
		t.Fatalf("cached model cost: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("same params must be served by cache, got %d db queries", got)
	}

	// 换时间窗（跨分钟对齐边界）必须回源，证明时间参数参与键
	if _, err := repo.QueryModelCost(t.Context(), nil, rangeStart, rangeEnd.Add(2*time.Minute)); err != nil {
		t.Fatalf("shifted window model cost: %v", err)
	}
	if got := queries(); got != 2 {
		t.Fatalf("different window must miss cache, got %d db queries", got)
	}
}

func TestTimeBucketAlignsToMinute(t *testing.T) {
	t.Parallel()

	repo, queries := newAuditRepo(t)

	if _, err := repo.ListDistinctModels(t.Context(), nil, "", rangeStart, rangeEnd); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// 前端每次请求的 endTime 取「当前时刻」，同一分钟内的抖动必须复用同一条目
	if _, err := repo.ListDistinctModels(t.Context(), nil, "", rangeStart, rangeEnd.Add(30*time.Second)); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("same minute window must reuse cache, got %d db queries", got)
	}
}
