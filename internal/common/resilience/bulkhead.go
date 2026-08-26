package resilience

import (
	"context"
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// SemaphoreConfig 信号量配置。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type SemaphoreConfig struct {
	// MaxConcurrent 每 key 最大并发数。
	MaxConcurrent int
	// AcquireTimeout 获取信号量的最长等待时间，超时返回 BulkheadFullError。
	AcquireTimeout time.Duration
}

// Semaphore 按 key 隔离的并发信号量（bulkhead）。每 key 一个 buffered channel 容量槽。
type Semaphore struct {
	cfg SemaphoreConfig
	mu  sync.Mutex
	// slots 每 key 的容量槽 channel（容量 = MaxConcurrent）。
	slots map[string]chan struct{}
}

// NewSemaphore 创建信号量。
//
//	@param cfg SemaphoreConfig
//	@return *Semaphore
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewSemaphore(cfg SemaphoreConfig) *Semaphore {
	return &Semaphore{cfg: cfg, slots: make(map[string]chan struct{})}
}

// Acquire 获取 key 的并发槽。等待 AcquireTimeout 内获得则返回幂等 release 闭包；
// 超时返回 *model.BulkheadFullError；ctx 取消返回 ctx.Err()。
//
//	@param ctx context.Context 请求上下文（等待期间监听取消）
//	@param key string 隔离维度标识
//	@return release func() 幂等释放函数（未获得槽时为 nil）
//	@return err error
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (s *Semaphore) Acquire(ctx context.Context, key string) (func(), error) {
	s.mu.Lock()
	ch, ok := s.slots[key]
	if !ok {
		// 软上限防无界增长（与 Guard.breaker 的重建策略一致）。在途请求持有
		// 旧 channel 引用不受影响，旧 channel 由 GC 回收。
		if len(s.slots) >= constant.ResilienceRegistryMaxKeys {
			s.slots = make(map[string]chan struct{}, constant.ResilienceRegistryMaxKeys/2)
		}
		ch = make(chan struct{}, s.cfg.MaxConcurrent)
		s.slots[key] = ch
	}
	s.mu.Unlock()

	select {
	case ch <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-ch }) }, nil
	case <-time.After(s.cfg.AcquireTimeout):
		return nil, &model.BulkheadFullError{Key: key, Limit: s.cfg.MaxConcurrent}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
