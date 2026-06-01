package lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/redis/go-redis/v9"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestLocker_Refresh_OwnerOnly(t *testing.T) {
	mr, rdb := newRedis(t)
	locker := lock.NewLocker(rdb)
	ctx := context.Background()
	key := "test:lock:" + uuid.New().String()
	value := "owner-1"
	other := "owner-2"

	ok, err := locker.Lock(ctx, key, value, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("lock: ok=%v err=%v", ok, err)
	}

	ok, err = locker.Refresh(ctx, key, value, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("owner refresh: ok=%v err=%v", ok, err)
	}

	ok, err = locker.Refresh(ctx, key, other, 5*time.Second)
	if err != nil {
		t.Fatalf("non-owner refresh err: %v", err)
	}
	if ok {
		t.Fatal("non-owner refresh must not succeed")
	}

	mr.FastForward(6 * time.Second)
	_ = mr
}

func TestLocker_Unlock_OwnerOnly(t *testing.T) {
	_, rdb := newRedis(t)
	locker := lock.NewLocker(rdb)
	ctx := context.Background()
	key := "test:lock:" + uuid.New().String()

	if ok, err := locker.Lock(ctx, key, "owner-1", 5*time.Second); !ok || err != nil {
		t.Fatalf("lock: ok=%v err=%v", ok, err)
	}

	if err := locker.Unlock(ctx, key, "owner-2"); err != nil {
		t.Fatalf("non-owner unlock err: %v", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 1 {
		t.Fatal("non-owner unlock must not delete the key")
	}

	if err := locker.Unlock(ctx, key, "owner-1"); err != nil {
		t.Fatalf("owner unlock err: %v", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 0 {
		t.Fatal("owner unlock must delete the key")
	}
}
