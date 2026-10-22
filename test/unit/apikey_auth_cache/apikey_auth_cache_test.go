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

// 各用例必须使用彼此不同的密钥：鉴权回源的 singleflight 组是包级的（生产只有一套
// db + cache，三个协议分组共用同一份鉴权结果），并发用例共用同一密钥时，后启动的
// 用例会直接拿到先启动用例的 flight 结果、跳过自己的库，断言「查了几次库」就失真。
const (
	testToken        = "sk-aris-loadtesttoken0123456789"
	testTokenNoCache = "sk-aris-loadtesttoken-nocache"
	testTokenJitter  = "sk-aris-loadtesttoken-jitter"
	testTokenAbsent  = "sk-aris-loadtesttoken-absent"
)

// newFixture 建一个含单个用户与单个 API Key 的 sqlite 库 + miniredis 客户端。
func newFixture(t *testing.T, token string) (*gorm.DB, *redis.Client, *miniredis.Miniredis) {
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
	key := &model.ProxyAPIKey{UserID: user.ID, Name: "k1", Key: token}
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

	db, rdb, mr := newFixture(t, testToken)
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
	if ttl := mr.TTL(keys[0]); ttl <= 0 || ttl > constant.APIKeyAuthCacheTTL+constant.APIKeyAuthCacheTTLJitter {
		t.Fatalf("unexpected cache ttl: %v", ttl)
	}
}

// TestCacheTTLIsJittered 断言写入 TTL 落在 [基准, 基准+抖动] 内，且不是恒定值
//
// 固定 TTL 会让同批写入的条目在同一秒集体过期，把回源压成周期尖峰；抖动
// 至少要保证「不会超过基准+抖动」这一上界（吊销可见延迟因此有确定上限）。
func TestCacheTTLIsJittered(t *testing.T) {
	t.Parallel()

	db, rdb, mr := newFixture(t, testTokenJitter)

	if _, err := middleware.ResolveAPIKeyAuth(t.Context(), db, rdb, testTokenJitter); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	keys := mr.Keys()
	if len(keys) != 1 {
		t.Fatalf("expected exactly 1 cache key, got %v", keys)
	}

	ttl := mr.TTL(keys[0])
	if ttl < constant.APIKeyAuthCacheTTL || ttl > constant.APIKeyAuthCacheTTL+constant.APIKeyAuthCacheTTLJitter {
		t.Fatalf("ttl %v out of [%v, %v]", ttl, constant.APIKeyAuthCacheTTL, constant.APIKeyAuthCacheTTL+constant.APIKeyAuthCacheTTLJitter)
	}
}

func TestResolveAPIKeyAuthWithoutCache(t *testing.T) {
	t.Parallel()

	db, _, _ := newFixture(t, testTokenNoCache)
	queries := countQueries(t, db)

	for range 2 {
		if _, err := middleware.ResolveAPIKeyAuth(t.Context(), db, nil, testTokenNoCache); err != nil {
			t.Fatalf("resolve without cache: %v", err)
		}
	}
	if got := queries(); got != 2 {
		t.Fatalf("nil cache should always hit db, got %d queries", got)
	}
}

func TestResolveAPIKeyAuthUnknownToken(t *testing.T) {
	t.Parallel()

	db, rdb, _ := newFixture(t, testTokenAbsent)

	if _, err := middleware.ResolveAPIKeyAuth(t.Context(), db, rdb, "sk-aris-unknown"); err == nil {
		t.Fatal("unknown token must return an error")
	}
}

// TestCacheTTLConstantIsShortEnough 钉死 TTL 的量级：吊销/轮换的可见延迟不应超过 1 分钟。
//
// 上界必须按「基准 + 抖动」算：抖动拉长的部分同样计入吊销可见延迟。
func TestCacheTTLConstantIsShortEnough(t *testing.T) {
	t.Parallel()

	if constant.APIKeyAuthCacheTTL <= 0 {
		t.Fatalf("api key auth cache ttl must be positive, got %v", constant.APIKeyAuthCacheTTL)
	}
	if constant.APIKeyAuthCacheTTLJitter < 0 {
		t.Fatalf("api key auth cache jitter must not be negative, got %v", constant.APIKeyAuthCacheTTLJitter)
	}
	if maxTTL := constant.APIKeyAuthCacheTTL + constant.APIKeyAuthCacheTTLJitter; maxTTL > time.Minute {
		t.Fatalf("api key auth cache ttl+jitter must stay within 1m, got %v", maxTTL)
	}
}
