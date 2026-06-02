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
	lockFunc     func(ctx context.Context, key, value string, expire time.Duration) (bool, error)
	refreshOK    bool
	refreshErr   error
	refreshCnt   atomic.Int32
	refreshAtCnt chan struct{}
	unlockCnt    atomic.Int32
}

func (m *mockLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	if m.lockFunc != nil {
		return m.lockFunc(ctx, key, value, expire)
	}
	return true, nil
}
func (m *mockLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	m.refreshCnt.Add(1)
	if m.refreshAtCnt != nil {
		select {
		case m.refreshAtCnt <- struct{}{}:
		default:
		}
	}
	if m.refreshErr != nil {
		return false, m.refreshErr
	}
	return m.refreshOK, nil
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
	m := &mockLocker{
		refreshOK:    false,
		refreshErr:   errors.New("redis down"),
		refreshAtCnt: make(chan struct{}, 16),
	}
	key := "test:renewfail"

	fnReturned := make(chan struct{})
	cron.RunWithLock(context.Background(), m, key, cron.LockOptions{
		TTL:           200 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
	}, func(ctx context.Context) {
		for i := 0; i < 3; i++ {
			select {
			case <-m.refreshAtCnt:
			case <-time.After(2 * time.Second):
				t.Errorf("refresh attempt %d not signaled within 2s", i+1)
				close(fnReturned)
				return
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
	m := &mockLocker{refreshOK: false}
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
	m := &mockLocker{refreshOK: true}
	key := "test:unlock"

	cron.RunWithLock(context.Background(), m, key, cron.LockOptions{
		TTL:           1 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	}, func(ctx context.Context) {})

	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected 1 unlock after fn returns, got %d", got)
	}
}

func TestRunWithLock_ContextCancelReleasesLock(t *testing.T) {
	m := &mockLocker{refreshOK: true, refreshAtCnt: make(chan struct{}, 16)}
	key := "test:cancel"

	ctx, cancel := context.WithCancel(context.Background())
	runReturned := make(chan struct{})

	go func() {
		defer close(runReturned)
		cron.RunWithLock(ctx, m, key, cron.LockOptions{
			TTL:           200 * time.Millisecond,
			RenewInterval: 30 * time.Millisecond,
		}, func(ctx context.Context) {
			<-ctx.Done()
		})
	}()

	// 等至少一次续期再取消，确保 renewLoop 已启动。
	select {
	case <-m.refreshAtCnt:
	case <-time.After(1 * time.Second):
		t.Fatal("expected at least one refresh before cancel")
	}

	cancel()

	select {
	case <-runReturned:
	case <-time.After(1 * time.Second):
		t.Fatal("RunWithLock should return after context cancel")
	}

	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected exactly 1 unlock, got %d", got)
	}
}
