package inflight_test

import (
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

func TestTracker_TrackAndUntrack(t *testing.T) {
	tracker := inflight.InitTracker()

	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	tracker.Untrack()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Drain(time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete quickly when no inflight requests")
	}
}

func TestTracker_TrackReturnsFalseDuringDraining(t *testing.T) {
	tracker := inflight.InitTracker()

	tracker.Track()

	untrackCh := make(chan struct{})
	go func() {
		<-untrackCh
		tracker.Untrack()
	}()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(2 * time.Second)
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
	tracker := inflight.InitTracker()

	tracker.Track()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(100 * time.Millisecond)
	}()

	result := <-drained
	if result {
		t.Fatal("Drain should return false on timeout")
	}

	tracker.Untrack()
}

func TestTracker_ConcurrentTrackUntrack(t *testing.T) {
	tracker := inflight.InitTracker()

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
		tracker.Drain(time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete after all Track/Untrack pairs resolve")
	}
}
