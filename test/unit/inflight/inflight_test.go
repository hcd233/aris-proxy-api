package inflight_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

func TestTracker_TrackAndUntrack(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	tracker.Untrack()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Drain(time.Second, time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete quickly when no inflight requests")
	}
}

func TestTracker_TrackReturnsFalseDuringDraining(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	tracker.Track()

	untrackCh := make(chan struct{})
	go func() {
		<-untrackCh
		tracker.Untrack()
	}()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(2*time.Second, 2*time.Second)
	}()

	untrackCh <- struct{}{}

	drainResult := <-drained
	if !drainResult {
		t.Fatal("Drain should complete after Untrack")
	}

	if tracker.Track() {
		t.Fatal("Track should return false during draining")
	}
}

func TestTracker_DrainTimeout(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	tracker.Track()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(100*time.Millisecond, 100*time.Millisecond)
	}()

	result := <-drained
	if result {
		t.Fatal("Drain should return false on timeout")
	}

	tracker.Untrack()
}

func TestTracker_ConcurrentTrackUntrack(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tracker.Track() {
				tracker.Untrack()
			}
		}()
	}
	wg.Wait()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Drain(time.Second, time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete after all Track/Untrack pairs resolve")
	}
}

// TestTracker_DrainSoftBroadcastCancel 验证 soft deadline 到点后广播取消：
// CancelOnDrain 派生的 ctx 被取消；请求在 hard 窗口内释放后 Drain 返回 true。
func TestTracker_DrainSoftBroadcastCancel(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	derived, cancelDerived := tracker.CancelOnDrain(context.Background())

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(50*time.Millisecond, time.Second)
	}()

	select {
	case <-derived.Done():
	case <-time.After(time.Second):
		t.Fatal("CancelOnDrain ctx should be canceled after soft deadline broadcast")
	}
	cancelDerived()

	tracker.Untrack()

	if result := <-drained; !result {
		t.Fatal("Drain should return true when request released within hard window")
	}
}

// TestTracker_CancelOnDrainNoBroadcast 验证请求自然完成时（soft 内全部释放）
// 广播不触发：派生 ctx 不被取消，Drain 返回 true。
func TestTracker_CancelOnDrainNoBroadcast(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()

	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}
	derived, cancelDerived := tracker.CancelOnDrain(context.Background())
	tracker.Untrack()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(50*time.Millisecond, time.Second)
	}()

	if result := <-drained; !result {
		t.Fatal("Drain should return true when all requests complete in soft window")
	}

	select {
	case <-derived.Done():
		t.Fatal("CancelOnDrain ctx must not be canceled when no broadcast happens")
	default:
	}

	// 请求方显式结束后派生 ctx 关闭（body Close 语义），守护 goroutine 退出
	cancelDerived()
	select {
	case <-derived.Done():
	default:
		t.Fatal("CancelOnDrain ctx should be canceled after caller cancel")
	}
}
