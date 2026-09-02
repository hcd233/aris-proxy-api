package inflight

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

type Tracker struct {
	wg         sync.WaitGroup
	state      atomic.Int32
	cancelCh   chan struct{}
	cancelOnce sync.Once
}

func NewTracker() *Tracker {
	t := &Tracker{}
	t.state.Store(constant.InflightStateRunning)
	t.cancelCh = make(chan struct{})
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

// Drain 两阶段排空：soft 窗口内等待所有请求自然完成；soft 到点广播取消信号
// （CancelOnDrain 派生的 ctx 被取消，使阻塞的上游读返回 context canceled），
// 再等 hard 窗口让被截断的请求写完错误帧、计量并 Untrack。
//
//	@param soft time.Duration 自然等待窗口
//	@param hard time.Duration 广播后的收尾窗口
//	@return bool 所有请求是否已释放（hard 超时返回 false，由 HTTP shutdown 兜底）
//	@author centonhuang
//	@update 2026-08-15 10:00:00
func (t *Tracker) Drain(soft, hard time.Duration) bool {
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
	case <-time.After(soft):
		t.broadcastCancel()
		logger.Logger().Warn("[Inflight] Drain soft deadline reached, canceling inflight requests",
			zap.Duration("softTimeout", soft))
		select {
		case <-done:
			logger.Logger().Info("[Inflight] All inflight requests completed after cancel")
			return true
		case <-time.After(hard):
			logger.Logger().Warn("[Inflight] Drain hard deadline reached, some requests may not have completed",
				zap.Duration("hardTimeout", hard))
			return false
		}
	}
}

// CancelOnDrain 返回 ctx 的派生 context 及其 cancel：drain soft deadline 广播时取消派生 ctx，
// 使依赖该 ctx 的阻塞操作（如上游 SSE 读）退出。
//
// 调用方必须在请求生命周期结束（上游 body Close 或出错立即返回）时调用 cancel：
// fiber/fasthttp 的请求 ctx 不随请求结束而 Done（RequestCtx.Done 返回 nil channel），
// 派生 ctx 不会自然结束，守护 goroutine 与 ctx 若无人显式 cancel 会随请求累积泄漏
// （生产实测每个上游请求泄漏一个 goroutine，24h 内 goroutine 随 LLM 代理流量阶梯上涨）。
//
//	@param ctx context.Context
//	@return context.Context 派生 ctx
//	@return context.CancelFunc 结束函数，幂等，body Close 时必须调用
//	@author centonhuang
//	@update 2026-09-02 10:00:00
func (t *Tracker) CancelOnDrain(ctx context.Context) (context.Context, context.CancelFunc) {
	derived, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-t.cancelCh:
			cancel()
		case <-derived.Done():
		}
	}()
	return derived, cancel
}

func (t *Tracker) broadcastCancel() {
	t.cancelOnce.Do(func() { close(t.cancelCh) })
}

func (t *Tracker) IsDraining() bool {
	return t.state.Load() == constant.InflightStateDraining
}
