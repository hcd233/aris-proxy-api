package inflight_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

func TestGetTracker_DefaultIsUsable(t *testing.T) {
	t.Parallel()
	tracker := inflight.GetTracker()
	if tracker == nil {
		t.Fatal("GetTracker should return a usable default tracker before InitTracker")
	}
	if !tracker.Track() {
		t.Fatal("default tracker should accept requests")
	}
	tracker.Untrack()
}
