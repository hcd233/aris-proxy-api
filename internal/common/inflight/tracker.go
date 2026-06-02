package inflight

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

type Tracker struct {
	wg    sync.WaitGroup
	state atomic.Int32
}

var (
	globalTracker   *Tracker
	globalTrackerMu sync.Mutex
)

func InitTracker() *Tracker {
	t := newRunningTracker()
	globalTrackerMu.Lock()
	globalTracker = t
	globalTrackerMu.Unlock()
	return t
}

func GetTracker() *Tracker {
	globalTrackerMu.Lock()
	defer globalTrackerMu.Unlock()
	if globalTracker == nil {
		globalTracker = newRunningTracker()
	}
	return globalTracker
}

func newRunningTracker() *Tracker {
	t := &Tracker{}
	t.state.Store(constant.InflightStateRunning)
	return t
}

func (t *Tracker) Track() bool {
	if t.state.Load() == constant.InflightStateDraining {
		return false
	}
	t.wg.Add(1)
	if t.state.Load() == constant.InflightStateDraining {
		t.wg.Done()
		return false
	}
	return true
}

func (t *Tracker) Untrack() {
	t.wg.Done()
}

func (t *Tracker) Drain(timeout time.Duration) bool {
	t.state.Store(constant.InflightStateDraining)

	done := make(chan struct{})
	go func() {
		defer close(done)
		t.wg.Wait()
	}()

	select {
	case <-done:
		logger.Logger().Info("[Inflight] All inflight requests completed")
		return true
	case <-time.After(timeout):
		logger.Logger().Warn("[Inflight] Drain timed out, some requests may not have completed",
			zap.Duration("timeout", timeout))
		return false
	}
}

func (t *Tracker) IsDraining() bool {
	return t.state.Load() == constant.InflightStateDraining
}
