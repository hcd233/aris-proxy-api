// Package apikey_auth_cache API Key 鉴权缓存回归测试。
//
// 背景：APIKeyMiddleware 原先每请求查两次库（key + user），是 /models 这类 Proxy 快接口的
// 主要成本（BENCHMARK §3.7）。改造后 key 归属走 Redis 缓存、用户信息复用 JwtMiddleware 的
// jwt:user:{id} 缓存。本测试用 sqlite + miniredis 断言：
//  1. 首次调用回源 1 次 DB，第二次命中缓存不再查库；
//  2. 无缓存（nil）时退化为每次查库；
//  3. 未知密钥返回错误；
//  4. 缓存 key 不含明文密钥（只存 SHA-256 摘要），且带 TTL。
package apikey_auth_cache

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

const testToken = "sk-aris-loadtesttoken0123456789"

// newFixture 建一个含单个用户与单个 API Key 的 sqlite 库 + miniredis 客户端。
func newFixture(t *testing.T) (*gorm.DB, *redis.Client, *miniredis.Miniredis) {
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

	if err := db.AutoMigrate(&model.User{}, &model.ProxyAPIKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := &model.User{Name: "alice", Email: "alice@example.com", Permission: enum.PermissionUser}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	key := &model.ProxyAPIKey{UserID: user.ID, Name: "k1", Key: testToken}
	if err := db.Create(key).Error; err != nil {
		t.Fatalf("seed api key: %v", err)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return db, rdb, mr
}

// countQueries 注册 GORM 查询计数器，返回读取函数。必须在种子数据之后调用。
func countQueries(t *testing.T, db *gorm.DB) func() int64 {
	t.Helper()

	var counter atomic.Int64
	if err := db.Callback().Query().Before("gorm:query").Register("test:count_queries", func(*gorm.DB) {
		counter.Add(1)
	}); err != nil {
		t.Fatalf("register query counter: %v", err)
	}
	return counter.Load
}

func TestResolveAPIKeyAuthCachesLookup(t *testing.T) {
	t.Parallel()

	db, rdb, mr := newFixture(t)
	queries := countQueries(t, db)

	first, err := middleware.ResolveAPIKeyAuth(t.Context(), db, rdb, testToken)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("first resolve should hit db once, got %d", got)
	}
	if first.APIKeyName != "k1" || first.UserID == 0 {
		t.Fatalf("unexpected auth payload: %+v", first)
	}

	second, err := middleware.ResolveAPIKeyAuth(t.Context(), db, rdb, testToken)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if got := queries(); got != 1 {
		t.Fatalf("second resolve must be served by cache, got %d db queries", got)
	}
	if *second != *first {
		t.Fatalf("cached payload mismatch: first=%+v second=%+v", *first, *second)
	}

	keys := mr.Keys()
	if len(keys) != 1 {
		t.Fatalf("expected exactly 1 cache key, got %v", keys)
	}
	if strings.Contains(keys[0], testToken) {
		t.Fatalf("cache key must not contain plaintext token: %s", keys[0])
	}
	if ttl := mr.TTL(keys[0]); ttl <= 0 || ttl > constant.APIKeyAuthCacheTTL {
		t.Fatalf("unexpected cache ttl: %v", ttl)
	}
}

func TestResolveAPIKeyAuthWithoutCache(t *testing.T) {
	t.Parallel()

	db, _, _ := newFixture(t)
	queries := countQueries(t, db)

	for range 2 {
		if _, err := middleware.ResolveAPIKeyAuth(t.Context(), db, nil, testToken); err != nil {
			t.Fatalf("resolve without cache: %v", err)
		}
	}
	if got := queries(); got != 2 {
		t.Fatalf("nil cache should always hit db, got %d queries", got)
	}
}

func TestResolveAPIKeyAuthUnknownToken(t *testing.T) {
	t.Parallel()

	db, rdb, _ := newFixture(t)

	if _, err := middleware.ResolveAPIKeyAuth(t.Context(), db, rdb, "sk-aris-unknown"); err == nil {
		t.Fatal("unknown token must return an error")
	}
}

// TestCacheTTLConstantIsShortEnough 钉死 TTL 的量级：吊销/轮换的可见延迟不应超过 1 分钟。
func TestCacheTTLConstantIsShortEnough(t *testing.T) {
	t.Parallel()

	if constant.APIKeyAuthCacheTTL <= 0 || constant.APIKeyAuthCacheTTL > time.Minute {
		t.Fatalf("api key auth cache ttl must be in (0, 1m], got %v", constant.APIKeyAuthCacheTTL)
	}
}
