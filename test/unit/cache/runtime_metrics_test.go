package cache_test

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/redis/go-redis/v9"
)

// 回归背景：WriteSnapshot 原实现只裁剪 ZSET 成员，data key 本身永不过期。
// pod 消亡后其 data key（满窗口时约 10MB）永久残留，生产已积累 645 个僵尸 key。

func newRuntimeMetricsFixture(t *testing.T) (*miniredis.Miniredis, *cache.RuntimeMetricsCache) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, cache.NewRuntimeMetricsCache(rdb)
}

func writeSnapshotAt(t *testing.T, c *cache.RuntimeMetricsCache, instanceID string, score int64) {
	t.Helper()
	retention := int64(constant.RuntimeMetricsRetention / time.Second)
	if err := c.WriteSnapshot(instanceID, score, []byte(`{"ts":1}`), score-retention); err != nil {
		t.Fatalf("WriteSnapshot(%s): %v", instanceID, err)
	}
}

func dataKeyOf(instanceID string) string {
	return "metrics:runtime:data:" + instanceID
}

func TestWriteSnapshot_SetsTTL(t *testing.T) {
	t.Parallel()
	mr, c := newRuntimeMetricsFixture(t)

	writeSnapshotAt(t, c, "pod-a", 1788417460)

	// 期望 TTL = retention + grace
	want := constant.RuntimeMetricsRetention + constant.RuntimeMetricsKeyGracePeriod
	ttl := mr.TTL(dataKeyOf("pod-a"))
	if ttl != want {
		t.Fatalf("data key TTL = %v, want %v (retention + grace)", ttl, want)
	}
}

func TestWriteSnapshot_RenewsTTLWhileActive(t *testing.T) {
	t.Parallel()
	mr, c := newRuntimeMetricsFixture(t)
	key := dataKeyOf("pod-a")

	// 活跃 pod 每 5s 写一次即续期：写入后即使快进超过 retention，只要间隔小于 TTL 就不应消失
	writeSnapshotAt(t, c, "pod-a", 1788417460)
	mr.FastForward(20 * time.Hour)
	writeSnapshotAt(t, c, "pod-a", 1788417460+int64((20*time.Hour)/time.Second))
	mr.FastForward(20 * time.Hour)
	if !mr.Exists(key) {
		t.Fatal("data key expired while instance kept writing (TTL not renewed)")
	}

	// 停止写入（pod 消亡）后，最后一个 TTL 周期内必须自动过期
	mr.FastForward(constant.RuntimeMetricsRetention + constant.RuntimeMetricsKeyGracePeriod + time.Second)
	if mr.Exists(key) {
		t.Fatal("data key survived past retention + grace after instance stopped writing")
	}
}
