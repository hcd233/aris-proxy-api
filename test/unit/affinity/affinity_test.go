package affinity

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestAffinityKeyPrefersSessionHeader(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, map[string]string{
		"X-Session-Id": "sess-1",
	})
	key, ok := usecase.AffinityKey(ctx, "gpt-4", "hello")
	if !ok || key != "sess-1" {
		t.Fatalf("key = %q ok=%v, want sess-1", key, ok)
	}
}

func TestAffinityKeyOpencodeSessionHeader(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, map[string]string{
		"x-opencode-session": "oc-9",
	})
	key, ok := usecase.AffinityKey(ctx, "gpt-4", "hello")
	if !ok || key != "oc-9" {
		t.Fatalf("key = %q ok=%v, want oc-9", key, ok)
	}
}

func TestAffinityKeyFingerprintStable(t *testing.T) {
	t.Parallel()
	k1, ok1 := usecase.AffinityKey(context.Background(), "gpt-4", "hello")
	k2, ok2 := usecase.AffinityKey(context.Background(), "gpt-4", "hello")
	k3, _ := usecase.AffinityKey(context.Background(), "gpt-4", "other")
	if !ok1 || !ok2 || k1 != k2 {
		t.Fatalf("同输入指纹应稳定: %q %q (ok=%v %v)", k1, k2, ok1, ok2)
	}
	if k1 == k3 {
		t.Fatal("不同输入指纹应不同")
	}
}

func TestAffinityKeyNoInput(t *testing.T) {
	t.Parallel()
	if _, ok := usecase.AffinityKey(context.Background(), "gpt-4", ""); ok {
		t.Fatal("无会话头且无文本应返回 ok=false")
	}
}

func TestAffinityStoreGetPut(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := usecase.NewAffinityStore(rdb)
	ctx := context.Background()

	if _, ok := store.Get(ctx, 1, "gpt-4", "k1"); ok {
		t.Fatal("未写入应 miss")
	}
	store.Put(ctx, 1, "gpt-4", "k1", 42)
	got, ok := store.Get(ctx, 1, "gpt-4", "k1")
	if !ok || got != 42 {
		t.Fatalf("Get = %d,%v want 42,true", got, ok)
	}
	// 隔离：不同用户/alias/key 不串
	if _, ok := store.Get(ctx, 2, "gpt-4", "k1"); ok {
		t.Fatal("跨用户不应命中")
	}
	if _, ok := store.Get(ctx, 1, "gpt-4", "k2"); ok {
		t.Fatal("不同 key 不应命中")
	}
	// TTL 到期失效
	mr.FastForward(constant.AffinityTTL + time.Second)
	if _, ok := store.Get(ctx, 1, "gpt-4", "k1"); ok {
		t.Fatal("TTL 过期后应 miss")
	}
}
