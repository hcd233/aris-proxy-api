package oauth_flow

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

// TestOAuthFlowCacheSingleUseAndTTL 真实 Redis（miniredis）语义：GETDEL 单次消费 + TTL 过期（Review Focus 5）。
func TestOAuthFlowCacheSingleUseAndTTL(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := cache.NewOAuthFlowCache(rdb)

	flow := vo.OAuthFlow{UserID: 7, Provider: enum.OAuthProviderAnthropic, Verifier: "v", CreatedAt: time.Now()}
	if err := store.Put(t.Context(), "state-1", flow); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := store.Take(t.Context(), "state-1")
	if err != nil || !ok || got.Verifier != "v" {
		t.Fatalf("first take: ok=%v err=%v flow=%+v", ok, err, got)
	}
	if _, ok, _ := store.Take(t.Context(), "state-1"); ok {
		t.Fatal("second take must fail (GETDEL single use)")
	}

	// TTL 过期：写入后快进超过 OAuthFlowTTL，流记录不可再消费
	if err := store.Put(t.Context(), "state-2", flow); err != nil {
		t.Fatalf("put 2: %v", err)
	}
	mr.FastForward(constant.OAuthFlowTTL + time.Second)
	if _, ok, _ := store.Take(t.Context(), "state-2"); ok {
		t.Fatal("expired flow must not be consumable")
	}
}
