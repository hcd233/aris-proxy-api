// Package resilience 验证通用信号量 bulkhead 的并发上限、等待超时与幂等释放。
package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
)

func TestSemaphore_AcquireRelease(t *testing.T) {
	t.Parallel()
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 50 * time.Millisecond})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	release() // 幂等，不得 panic
	_, err = s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("after release should acquire again: %v", err)
	}
}

func TestSemaphore_ExceedsLimitReturnsBulkheadFull(t *testing.T) {
	t.Parallel()
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 30 * time.Millisecond})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()
	_, err = s.Acquire(context.Background(), "k")
	var bf *model.BulkheadFullError
	if !errors.As(err, &bf) {
		t.Fatalf("second Acquire err = %v, want BulkheadFullError", err)
	}
	if bf.Key != "k" || bf.Limit != 1 {
		t.Fatalf("BulkheadFullError = %+v, want key=k limit=1", bf)
	}
}

func TestSemaphore_KeysIsolated(t *testing.T) {
	t.Parallel()
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 30 * time.Millisecond})
	r1, err := s.Acquire(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Acquire k1: %v", err)
	}
	defer r1()
	r2, err := s.Acquire(context.Background(), "k2") // 不同 key 不受 k1 影响
	if err != nil {
		t.Fatalf("Acquire k2 should succeed: %v", err)
	}
	r2()
}

func TestSemaphore_ContextCancel(t *testing.T) {
	t.Parallel()
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: time.Second})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Acquire(ctx, "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire with canceled ctx err = %v, want context.Canceled", err)
	}
}

func TestBulkheadFullError(t *testing.T) {
	t.Parallel()
	e := &model.BulkheadFullError{Key: "k", Limit: 3}
	if e.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
}
