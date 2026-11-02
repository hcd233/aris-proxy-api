// dashboard 读缓存 TTL 回归测试。
//
// 背景：dashboard 是「贴近实时」的看板，前端提供 5s 刷新档；读缓存必须短于刷新间隔
// 才有意义（见 constant.ReadCacheDashboardTTL）。本测试用 sqlite + miniredis 断言：
//  1. dashboard 查询写入的条目是秒级 TTL，而不是其他时间窗查询的 60s；
//  2. 条目未过期时命中缓存、不再查库；
//  3. 越过 TTL 后必须回源。
//
// 用 QueryGroupUsage 是因为它是 dashboard 查询里少数能在 sqlite 执行的
// （其他查询用了 percentile_cont / date_trunc / date_bin / LEFT 等 PG 专属语法）。
package read_cache_window

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/domain/modelcall"
	"github.com/hcd233/aris-mass/internal/infrastructure/cache"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/infrastructure/repository"
)

// newDashboardRepo 建 sqlite + miniredis 的 dashboard 仓储，seed 一条窗口内失败调用。
//
//	@param t *testing.T
//	@return modelcall.DashboardRepository
//	@return *miniredis.Miniredis
//	@return func() int64 已执行的 DB 查询计数
func newDashboardRepo(t *testing.T) (modelcall.DashboardRepository, *miniredis.Miniredis, func() int64) {
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
	row := &model.ModelCallAudit{
		APIKeyID:           1,
		ModelID:            "m1",
		UpstreamStatusCode: 200,
		ErrorMessage:       "",
		CreatedAt:          rangeStart.Add(time.Hour),
	}
	if createErr := db.Create(row).Error; createErr != nil {
		t.Fatalf("seed audit: %v", createErr)
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

	return repository.NewDashboardRepository(db, cache.NewReadCache(rdb)), mr, counter.Load
}

// TestDashboardCacheExpiresInSeconds 钉住 dashboard 条目的秒级 TTL：
// 5s（+1s 抖动）后必须回源，否则开 5s 刷新的看板会读到分钟级陈旧数据。
func TestDashboardCacheExpiresInSeconds(t *testing.T) {
	t.Parallel()

	repo, mr, queries := newDashboardRepo(t)

	if _, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, enum.DashboardDimensionModel, ""); err != nil {
		t.Fatalf("first query: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("first call should hit db once, got %d", got)
	}

	keys := mr.Keys()
	if len(keys) != 1 {
		t.Fatalf("expected exactly one cache entry, got %v", keys)
	}
	ttl := mr.TTL(keys[0])
	if ttl <= 0 {
		t.Fatalf("dashboard cache entry not written, ttl = %v", ttl)
	}
	if ttl > constant.ReadCacheDashboardTTL+constant.ReadCacheDashboardTTLJitter {
		t.Fatalf("dashboard ttl %v exceeds second-level budget %v",
			ttl, constant.ReadCacheDashboardTTL+constant.ReadCacheDashboardTTLJitter)
	}
	if ttl >= constant.ReadCacheWindowTTL {
		t.Fatalf("dashboard ttl %v must stay far below window ttl %v", ttl, constant.ReadCacheWindowTTL)
	}

	// 同参数再查必须命中，不再查库
	if _, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, enum.DashboardDimensionModel, ""); err != nil {
		t.Fatalf("cached query: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("second call must hit cache, got %d db queries", got)
	}

	// 越过 TTL 后必须回源
	mr.FastForward(constant.ReadCacheDashboardTTL + constant.ReadCacheDashboardTTLJitter + time.Second)
	if _, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, enum.DashboardDimensionModel, ""); err != nil {
		t.Fatalf("expired query: %v", err)
	}
	if got := queries(); got != 2 {
		t.Fatalf("expired entry must reload from db, got %d db queries", got)
	}
}
