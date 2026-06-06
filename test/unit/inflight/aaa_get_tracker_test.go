package inflight_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

func TestNewTracker_IsUsable(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()
	if tracker == nil {
		t.Fatal("NewTracker should return a usable tracker")
	}
	if !tracker.Track() {
		t.Fatal("new tracker should accept requests")
	}
	tracker.Untrack()
}
