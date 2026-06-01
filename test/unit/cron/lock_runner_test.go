package cron_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/redis/go-redis/v9"
)

func newMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	return mr
}

func newRealLocker(t *testing.T) (lock.Locker, *miniredis.Miniredis) {
	mr := newMiniredis(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return lock.NewLocker(rdb), mr
}

type mockLocker struct {
	lockFunc   func(ctx context.Context, key, value string, expire time.Duration) (bool, error)
	refreshOK  atomic.Bool
	refreshErr atomic.Value
	refreshCnt atomic.Int32
	unlockCnt  atomic.Int32
}

func (m *mockLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	if m.lockFunc != nil {
		return m.lockFunc(ctx, key, value, expire)
	}
	return true, nil
}
func (m *mockLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	m.refreshCnt.Add(1)
	if v := m.refreshErr.Load(); v != nil {
		return false, v.(error)
	}
	return m.refreshOK.Load(), nil
}
func (m *mockLocker) Unlock(ctx context.Context, key, value string) error {
	m.unlockCnt.Add(1)
	return nil
}

func TestRunWithLock_LockFailed_SkipsFn(t *testing.T) {
	locker, mr := newRealLocker(t)
	called := false
	key := "test:lockfail"

	mr.Set(key, "other-instance")

	cron.RunWithLock(context.Background(), locker, key, cron.LockOptions{}, func(ctx context.Context) {
		called = true
	})

	if called {
		t.Fatal("fn must not run when lock failed")
	}
}

func TestRunWithLock_LockSuccess_RunsFnAndUnlocks(t *testing.T) {
	locker, _ := newRealLocker(t)
	key := "test:success"
	called := false

	cron.RunWithLock(context.Background(), locker, key, cron.LockOptions{
		TTL:           500 * time.Millisecond,
		RenewInterval: 100 * time.Millisecond,
	}, func(ctx context.Context) {
		called = true
	})

	if !called {
		t.Fatal("fn must run when lock acquired")
	}
}

func TestRunWithLock_RefreshesLock(t *testing.T) {
	locker, mr := newRealLocker(t)
	key := "test:refresh"

	done := make(chan struct{})
	cron.RunWithLock(context.Background(), locker, key, cron.LockOptions{
		TTL:           300 * time.Millisecond,
		RenewInterval: 80 * time.Millisecond,
	}, func(ctx context.Context) {
		mr.FastForward(1 * time.Second)
		close(done)
	})

	<-done
}

func TestRunWithLock_RenewFailure_StopsRenewal_KeepsFnRunning(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(false)
	m.refreshErr.Store(errors.New("redis down"))
	key := "test:renewfail"

	fnReturned := make(chan struct{})
	cron.RunWithLock(context.Background(), m, key, cron.LockOptions{
		TTL:           200 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
	}, func(ctx context.Context) {
		// 等待至少 3 次续期尝试（约 90ms）
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if m.refreshCnt.Load() >= 3 {
				break
			}
		}
		close(fnReturned)
	})

	select {
	case <-fnReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("fn must run to completion even when renewal keeps failing")
	}

	if m.refreshCnt.Load() < 3 {
		t.Fatalf("expected at least 3 refresh attempts, got %d", m.refreshCnt.Load())
	}
	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected exactly 1 unlock, got %d", got)
	}
}

func TestRunWithLock_LockLost_StopsRenewal_KeepsFnRunning(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(false)
	key := "test:locklost"

	fnReturned := make(chan struct{})
	cron.RunWithLock(context.Background(), m, key, cron.LockOptions{
		TTL:           500 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
	}, func(ctx context.Context) {
		close(fnReturned)
	})

	select {
	case <-fnReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("fn must run to completion when lock lost")
	}
}

func TestRunWithLock_DeferUnlockAlways(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(true)
	key := "test:unlock"

	cron.RunWithLock(context.Background(), m, key, cron.LockOptions{
		TTL:           1 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	}, func(ctx context.Context) {})

	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected 1 unlock after fn returns, got %d", got)
	}
}
