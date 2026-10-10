package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/samber/mo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

// 本文件覆盖通用读缓存（cache.GetOrLoad）的四类语义：
//   - 命中后不回源
//   - 空结果写空值标记（防缓存穿透）
//   - TTL 随机抖动（防缓存雪崩）
//   - singleflight 合并并发回源（防缓存击穿）+ 写后失效 + Redis 故障 fail-open

func newReadCache(t *testing.T) (*miniredis.Miniredis, *cache.ReadCache) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, cache.NewReadCache(rdb)
}

func TestReadCache_CacheHitSkipsLoader(t *testing.T) {
	t.Parallel()
	_, rc := newReadCache(t)

	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[[]string], error) {
		loads.Add(1)
		return mo.Some([]string{"gpt-a"}), nil
	}

	first, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", loader)
	if err != nil {
		t.Fatalf("first GetOrLoad: %v", err)
	}
	if got := first.OrEmpty(); len(got) != 1 || got[0] != "gpt-a" {
		t.Fatalf("first GetOrLoad = %v, want [gpt-a]", got)
	}

	second, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", loader)
	if err != nil {
		t.Fatalf("second GetOrLoad: %v", err)
	}
	if got := second.OrEmpty(); len(got) != 1 || got[0] != "gpt-a" {
		t.Fatalf("second GetOrLoad = %v, want [gpt-a]", got)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1 (cache hit must skip loader)", loads.Load())
	}
}

// TestReadCache_EmptyResultCached 防穿透：空结果写空值标记（短 TTL），重复读不回源
func TestReadCache_EmptyResultCached(t *testing.T) {
	t.Parallel()
	mr, rc := newReadCache(t)
	const key = "cache:rd:modellist:1:absent"

	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[[]string], error) {
		loads.Add(1)
		return mo.None[[]string](), nil
	}

	first, err := cache.GetOrLoad(t.Context(), rc, key, loader)
	if err != nil {
		t.Fatalf("first GetOrLoad: %v", err)
	}
	if first.IsPresent() {
		t.Fatalf("first GetOrLoad = %v, want none (empty result)", first)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1", loads.Load())
	}

	ttl := mr.TTL(key)
	if ttl <= 0 {
		t.Fatalf("empty marker not written, ttl = %v", ttl)
	}
	if ttl > constant.ReadCacheEmptyTTL+constant.ReadCacheEmptyTTLJitter {
		t.Fatalf("empty marker ttl %v exceeds short TTL budget %v", ttl, constant.ReadCacheEmptyTTL+constant.ReadCacheEmptyTTLJitter)
	}

	second, err := cache.GetOrLoad(t.Context(), rc, key, loader)
	if err != nil {
		t.Fatalf("second GetOrLoad: %v", err)
	}
	if second.IsPresent() {
		t.Fatalf("second GetOrLoad = %v, want none (empty marker hit)", second)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1 (empty marker must absorb repeat queries)", loads.Load())
	}
}

// TestReadCache_TTLJitter 防雪崩：同批写入的多个条目 TTL 互不相同（随机抖动打散过期时刻）
func TestReadCache_TTLJitter(t *testing.T) {
	t.Parallel()
	mr, rc := newReadCache(t)

	seen := map[time.Duration]bool{}
	for i := range 8 {
		key := "cache:rd:alias:" + string(rune('a'+i))
		if _, err := cache.GetOrLoad(t.Context(), rc, key, func(_ context.Context) (mo.Option[int], error) {
			return mo.Some(i), nil
		}); err != nil {
			t.Fatalf("GetOrLoad(%s): %v", key, err)
		}
		ttl := mr.TTL(key)
		if ttl <= 0 {
			t.Fatalf("key %s not written", key)
		}
		if ttl < constant.ReadCacheTTL || ttl > constant.ReadCacheTTL+constant.ReadCacheTTLJitter {
			t.Fatalf("key %s ttl %v out of jittered range [%v, %v]", key, ttl, constant.ReadCacheTTL, constant.ReadCacheTTL+constant.ReadCacheTTLJitter)
		}
		seen[ttl] = true
	}
	if len(seen) < 2 {
		t.Fatalf("all 8 keys share the same TTL %v, jitter not applied (avalanche risk)", mr.TTL("cache:rd:alias:a"))
	}
}

// TestReadCache_SingleflightMergesConcurrentLoad 防击穿：同 key 并发回源只查库一次
//
// 判定口径：不依赖调度的确定性断言——
//   - 无合并时回源次数必然等于并发调用者数（每个调用各查一次库）；
//   - 合并时至少两个调用者拿到同一个共享结果指针。
//
// 「恰好只回源一次」取决于所有调用是否都落在同一飞行窗口内，受调度影响，
// 不作为断言（否则在 CPU 高负载下 flake）。
func TestReadCache_SingleflightMergesConcurrentLoad(t *testing.T) {
	t.Parallel()
	_, rc := newReadCache(t)
	const key = "cache:rd:detail:7"

	var loads atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(_ context.Context) (mo.Option[*int], error) { //nolint:unparam // 单测 loader 恒成功，错误分支由 TestReadCache_LoaderErrorNotCached 覆盖
		if loads.Add(1) == 1 {
			close(started)
		}
		<-release
		// 返回指针载荷：未合并的并发回源会各自返回不同指针
		v := 1
		return mo.Some(&v), nil
	}

	const callers = 8
	results := make([]mo.Option[*int], callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i := range callers {
		wg.Go(func() {
			<-gate
			results[i], errs[i] = cache.GetOrLoad(t.Context(), rc, key, loader)
		})
	}
	close(gate)
	<-started
	close(release)
	wg.Wait()

	if got := loads.Load(); got >= callers {
		t.Fatalf("loader called %d times for %d concurrent callers, singleflight did not merge loads", got, callers)
	}
	shared := false
	for i := range callers {
		if errs[i] != nil {
			t.Fatalf("caller %d error: %v", i, errs[i])
		}
		got, ok := results[i].Get()
		if !ok || got == nil {
			t.Fatalf("caller %d got empty result", i)
		}
		for j := range i {
			other, _ := results[j].Get()
			if got == other {
				shared = true
			}
		}
	}
	if !shared {
		t.Fatal("no caller received the shared singleflight result")
	}
}

// TestReadCache_InvalidateAll 写后失效：清空命名空间后回源重建，且不误删其它 key
func TestReadCache_InvalidateAll(t *testing.T) {
	t.Parallel()
	mr, rc := newReadCache(t)

	if _, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", func(_ context.Context) (mo.Option[int], error) {
		return mo.Some(1), nil
	}); err != nil {
		t.Fatalf("GetOrLoad: %v", err)
	}
	if err := mr.Set("share:abc", "42"); err != nil {
		t.Fatalf("seed unrelated key: %v", err)
	}

	rc.InvalidateAll(t.Context())

	if mr.Exists("cache:rd:alias:1") {
		t.Fatal("read cache key survives InvalidateAll")
	}
	if !mr.Exists("share:abc") {
		t.Fatal("InvalidateAll deleted keys outside the read cache namespace")
	}
}

// TestReadCache_FailOpen Redis 故障 fail-open：缓存读写失败仍正确回源
func TestReadCache_FailOpen(t *testing.T) {
	t.Parallel()
	mr, rc := newReadCache(t)
	mr.SetError("redis down")

	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[int], error) {
		loads.Add(1)
		return mo.Some(42), nil
	}
	got, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", loader)
	if err != nil {
		t.Fatalf("GetOrLoad with broken redis: %v", err)
	}
	if v, ok := got.Get(); !ok || v != 42 {
		t.Fatalf("GetOrLoad = %v, want some(42)", got)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1", loads.Load())
	}
}

// TestReadCache_LoaderErrorNotCached 回源失败不写缓存、错误原样上抛
func TestReadCache_LoaderErrorNotCached(t *testing.T) {
	t.Parallel()
	_, rc := newReadCache(t)

	boom := ierr.New(ierr.ErrInternal, "db down")
	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[int], error) {
		loads.Add(1)
		return mo.None[int](), boom
	}
	if _, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", loader); !errors.Is(err, boom) {
		t.Fatalf("GetOrLoad error = %v, want %v", err, boom)
	}
	if _, err := cache.GetOrLoad(t.Context(), rc, "cache:rd:alias:1", loader); !errors.Is(err, boom) {
		t.Fatalf("second GetOrLoad error = %v, want %v (failure must not be cached)", err, boom)
	}
	if loads.Load() != 2 {
		t.Fatalf("loader called %d times, want 2", loads.Load())
	}
}

// TestReadCache_NilCacheBypasses 无缓存装配直接回源
func TestReadCache_NilCacheBypasses(t *testing.T) {
	t.Parallel()
	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[int], error) {
		loads.Add(1)
		return mo.Some(7), nil
	}
	got, err := cache.GetOrLoad(t.Context(), nil, "cache:rd:alias:1", loader)
	if err != nil {
		t.Fatalf("GetOrLoad: %v", err)
	}
	if v, ok := got.Get(); !ok || v != 7 {
		t.Fatalf("GetOrLoad = %v, want some(7)", got)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1", loads.Load())
	}
}
