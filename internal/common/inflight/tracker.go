package inflight

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

const (
	stateRunning  int32 = 0
	stateDraining int32 = 1
)

type Tracker struct {
	wg    sync.WaitGroup
	state atomic.Int32
}

var globalTracker *Tracker

func InitTracker() *Tracker {
	t := &Tracker{}
	t.state.Store(stateRunning)
	globalTracker = t
	return t
}

func GetTracker() *Tracker {
	return globalTracker
}

func (t *Tracker) Track() bool {
	if t.state.Load() == stateDraining {
		return false
	}
	t.wg.Add(1)
	if t.state.Load() == stateDraining {
		t.wg.Done()
		return false
	}
	return true
}

func (t *Tracker) Untrack() {
	t.wg.Done()
}

func (t *Tracker) Drain(timeout time.Duration) bool {
	t.state.Store(stateDraining)

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
	return t.state.Load() == stateDraining
}
