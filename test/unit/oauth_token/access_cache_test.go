package oauth_token

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

// TestOAuthAccessCacheExpiryGuard 缓存读取侧必须按安全余量判定有效期。
//
// 回归（CR I3）：Get 的注释承诺"已过安全余量返回 false"，实现原先只做反序列化，
// 完全依赖 Redis TTL——这里把 TTL 故意设得比剩余有效期长，模拟 TTL 计算偏差。
func TestOAuthAccessCacheExpiryGuard(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	accessCache := cache.NewOAuthAccessCache(rdb)
	ctx := t.Context()

	if _, ok := accessCache.Get(ctx, 7); ok {
		t.Fatal("empty cache must miss")
	}

	// 剩余有效期 30s < 安全余量 60s：即使 Redis key 还有 10 分钟 TTL 也必须视为未命中
	accessCache.Put(ctx, 7, service.OAuthAccess{
		Token:      "at-stale",
		AccountRef: "org-1",
		ExpiresAt:  time.Now().Add(30 * time.Second),
	}, 10*time.Minute)
	if _, ok := accessCache.Get(ctx, 7); ok {
		t.Fatalf("cache hit with less than %s remaining validity (guard must reject)", constant.OAuthAccessCacheGuard)
	}

	// 剩余有效期充足：命中且字段完整
	accessCache.Put(ctx, 8, service.OAuthAccess{
		Token:      "at-valid",
		AccountRef: "org-8",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}, 9*time.Minute)
	got, ok := accessCache.Get(ctx, 8)
	if !ok || got.Token != "at-valid" || got.AccountRef != "org-8" {
		t.Fatalf("cache miss on valid entry: %+v ok=%v", got, ok)
	}

	// Delete 后必须失效（重新授权路径依赖该语义）
	accessCache.Delete(ctx, 8)
	if _, ok := accessCache.Get(ctx, 8); ok {
		t.Fatal("cache must be invalidated after Delete")
	}
}
