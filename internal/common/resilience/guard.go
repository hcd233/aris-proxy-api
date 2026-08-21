package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// GuardConfig Guard 组合配置（熔断 + 信号量）。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type GuardConfig struct {
	CircuitEnabled             bool
	CircuitWindow              time.Duration
	CircuitMinRequests         int
	CircuitErrorThreshold      float64
	CircuitOpenTimeout         time.Duration
	CircuitHalfOpenMaxRequests int
	BulkheadEnabled            bool
	BulkheadMaxConcurrent      int
	BulkheadAcquireTimeout     time.Duration
}

// Metrics 容错事件指标回调（由接入方实现并注册到 Prometheus；nil 表示不采集）。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type Metrics interface {
	// SetBreakerState 熔断状态变化（0=closed, 1=open, 2=half-open）。
	SetBreakerState(key string, state enum.BreakerState)
	// IncCircuitOpen 熔断打开次数。
	IncCircuitOpen(key string)
	// IncCircuitRejected 熔断拒绝请求数。
	IncCircuitRejected(key string)
	// IncBulkheadRejected 信号量满载拒绝请求数。
	IncBulkheadRejected(key string)
}

// Guard 组合熔断与信号量：Allow 先过熔断再取信号量，Report 回写熔断结果。
// 按 key 懒创建熔断器，key 数量 = 接入方配置的 endpoint 数，常驻生命周期。
//
// ponytail: registry 不做 idle 清理，如出现 endpoint 动态增删场景再做 GC。
type Guard struct {
	cfg      GuardConfig
	metrics  Metrics
	mu       sync.Mutex
	breakers map[string]*Breaker
	sem      *Semaphore
}

// NewGuard 创建 Guard。metrics 可为 nil（跳过指标采集）。
//
//	@param cfg GuardConfig
//	@param metrics Metrics
//	@return *Guard
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewGuard(cfg GuardConfig, metrics Metrics) *Guard {
	return &Guard{
		cfg:      cfg,
		metrics:  metrics,
		breakers: make(map[string]*Breaker),
		sem: NewSemaphore(SemaphoreConfig{
			MaxConcurrent:  cfg.BulkheadMaxConcurrent,
			AcquireTimeout: cfg.BulkheadAcquireTimeout,
		}),
	}
}

// Allow 放行则返回幂等 release（调用方 defer 执行）；否则返回熔断/满载错误。
//
//	@param ctx context.Context 请求上下文
//	@param key string 熔断与隔离维度标识
//	@return release func() 幂等释放函数
//	@return err error *model.CircuitOpenError / *model.BulkheadFullError / ctx.Err()
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *Guard) Allow(ctx context.Context, key string) (func(), error) {
	if g.cfg.CircuitEnabled {
		b := g.breaker(key)
		if !b.Allow() {
			if g.metrics != nil {
				g.metrics.IncCircuitRejected(key)
			}
			return nil, &model.CircuitOpenError{Key: key, RetryAfter: b.RetryAfter()}
		}
	}
	if g.cfg.BulkheadEnabled {
		release, err := g.sem.Acquire(ctx, key)
		if err != nil {
			var bf *model.BulkheadFullError
			if errors.As(err, &bf) && g.metrics != nil {
				g.metrics.IncBulkheadRejected(key)
			}
			return nil, err
		}
		return release, nil
	}
	return func() {}, nil
}

// Report 上报一次调用结果（success 由调用方按业务判定）。
//
//	@param key string
//	@param success bool
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *Guard) Report(key string, success bool) {
	if !g.cfg.CircuitEnabled {
		return
	}
	g.breaker(key).Report(success)
}

func (g *Guard) breaker(key string) *Breaker {
	g.mu.Lock()
	defer g.mu.Unlock()
	b, ok := g.breakers[key]
	if !ok {
		b = NewBreaker(key, BreakerConfig{
			Window:              g.cfg.CircuitWindow,
			MinRequests:         g.cfg.CircuitMinRequests,
			ErrorThreshold:      g.cfg.CircuitErrorThreshold,
			OpenTimeout:         g.cfg.CircuitOpenTimeout,
			HalfOpenMaxRequests: g.cfg.CircuitHalfOpenMaxRequests,
		}, func(s enum.BreakerState) {
			if g.metrics != nil {
				g.metrics.SetBreakerState(key, s)
				if s == enum.StateOpen {
					g.metrics.IncCircuitOpen(key)
				}
			}
		})
		g.breakers[key] = b
	}
	return b
}
