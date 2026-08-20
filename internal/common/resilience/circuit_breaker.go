// Package resilience 通用服务容错组件（熔断/信号量隔离），不依赖任何领域类型。
package resilience

import (
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// BreakerConfig 熔断器配置。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type BreakerConfig struct {
	// Window 滑动窗口时长。
	Window time.Duration
	// MinRequests 窗口内最少请求数，低于该值不做熔断判定（防小流量误熔断）。
	MinRequests int
	// ErrorThreshold 错误率阈值（0~1），窗口内失败/总数 ≥ 该值且请求数达标时打开。
	ErrorThreshold float64
	// OpenTimeout 打开后保持时长，期满进入半开。
	OpenTimeout time.Duration
	// HalfOpenMaxRequests 半开期允许的并发探测请求数。
	HalfOpenMaxRequests int
}

// bucket 单个时间桶的成败计数。
type bucket struct {
	start   time.Time
	success int64
	failure int64
}

// Breaker 按 key 的三态熔断器。内部用互斥锁保护，可并发调用。
type Breaker struct {
	key string
	cfg BreakerConfig
	mu  sync.Mutex

	state        enum.BreakerState
	buckets      [constant.ResilienceWindowBucketCount]bucket
	openSince    time.Time
	halfOpenSent int
	halfOpenOK   int

	onStateChange func(enum.BreakerState)
}

// NewBreaker 创建熔断器。onStateChange 在状态转换后回调（锁内调用，实现需原子、不阻塞）。
//
//	@param key string 熔断维度标识（如上游 BaseURL|APIKey）
//	@param cfg BreakerConfig 配置
//	@param onStateChange func(enum.BreakerState) 状态转换回调，可为 nil
//	@return *Breaker
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewBreaker(key string, cfg BreakerConfig, onStateChange func(enum.BreakerState)) *Breaker {
	return &Breaker{key: key, cfg: cfg, onStateChange: onStateChange}
}

// State 返回当前状态。
//
//	@receiver b *Breaker
//	@return enum.BreakerState
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (b *Breaker) State() enum.BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Allow 判断是否放行新请求。Open 状态在 OpenTimeout 期满时自动转 HalfOpen 并放行探测；
// HalfOpen 状态限量放行 HalfOpenMaxRequests 个并发探测。
//
//	@receiver b *Breaker
//	@return bool 是否放行
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case enum.StateClosed:
		return true
	case enum.StateOpen:
		if time.Since(b.openSince) >= b.cfg.OpenTimeout {
			b.halfOpenSent = 0
			b.halfOpenOK = 0
			b.transitionTo(enum.StateHalfOpen)
			b.halfOpenSent++ // 本次探测请求计入半开配额
			return true
		}
		return false
	case enum.StateHalfOpen:
		if b.halfOpenSent < b.cfg.HalfOpenMaxRequests {
			b.halfOpenSent++
			return true
		}
		return false
	}
	return false
}

// Report 上报一次调用结果（success 由调用方判定），驱动窗口计数与状态转换。
//
//	@receiver b *Breaker
//	@param success bool 调用是否成功
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (b *Breaker) Report(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == enum.StateHalfOpen {
		if success {
			b.halfOpenOK++
			if b.halfOpenOK >= b.cfg.HalfOpenMaxRequests {
				b.resetWindow()
				b.transitionTo(enum.StateClosed)
			}
		} else {
			b.openSince = time.Now()
			b.transitionTo(enum.StateOpen)
		}
		return
	}

	b.record(success)
	if b.state == enum.StateClosed {
		successTotal, failureTotal := b.counts()
		if successTotal+failureTotal >= int64(b.cfg.MinRequests) &&
			float64(failureTotal)/float64(successTotal+failureTotal) >= b.cfg.ErrorThreshold {
			b.openSince = time.Now()
			b.transitionTo(enum.StateOpen)
		}
	}
}

// RetryAfter 返回 Open 状态的剩余时间（非 Open 状态返回 0），供 Retry-After 响应头使用。
//
//	@receiver b *Breaker
//	@return time.Duration
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (b *Breaker) RetryAfter() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != enum.StateOpen {
		return 0
	}
	remain := b.cfg.OpenTimeout - time.Since(b.openSince)
	if remain < 0 {
		return 0
	}
	return remain
}

func (b *Breaker) transitionTo(s enum.BreakerState) {
	b.state = s
	if b.onStateChange != nil {
		b.onStateChange(s)
	}
}

// record 把一次结果写入当前时间桶；桶过期（槽位滚动）时重置。
func (b *Breaker) record(success bool) {
	now := time.Now()
	bw := b.cfg.Window / constant.ResilienceWindowBucketCount
	idx := int(now.Unix()/int64(bw.Seconds())) % constant.ResilienceWindowBucketCount
	cur := &b.buckets[idx]
	if cur.start.IsZero() || now.Sub(cur.start) >= bw {
		*cur = bucket{start: now.Truncate(bw)}
	}
	if success {
		cur.success++
	} else {
		cur.failure++
	}
}

// counts 统计窗口内（从当前时间回看 Window）的成败总数。
func (b *Breaker) counts() (success, failure int64) {
	now := time.Now()
	for i := range b.buckets {
		bu := &b.buckets[i]
		if bu.start.IsZero() || now.Sub(bu.start) > b.cfg.Window {
			continue
		}
		success += bu.success
		failure += bu.failure
	}
	return success, failure
}

func (b *Breaker) resetWindow() {
	for i := range b.buckets {
		b.buckets[i] = bucket{}
	}
}
