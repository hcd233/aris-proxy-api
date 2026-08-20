# API 服务容错（降级/熔断/隔离）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 aris-proxy-api 增加按上游 endpoint 的熔断、并发隔离（bulkhead）与快速失败降级（503 + Retry-After），组件通用化供其他调用点复用。

**Architecture:** 通用组件层 `internal/common/resilience/`（纯参数化：三态熔断器 + 信号量 + Guard 组合，不依赖领域类型、不读 config），proxy 适配层 `internal/infrastructure/transport/guard.go`（endpoint key 生成、错误分类、从 config 全局变量组装、注册 Prometheus 指标）。接入点在 `doUpstreamRequest`/`sendRequest`，位于现有 `SendUpstreamWithRetry` 外层；熔断打开/bulkhead 满载时经 `upstreamProxyError` 映射为 503 + Retry-After 降级响应。

**Tech Stack:** Go 1.25、sync、context、prometheus/client_golang（已有依赖）、fx（已有）、viper（已有）。**不引入新依赖。**

## Global Constraints

- 设计文档：`docs/superpowers/specs/2026-08-20-api-resilience-design.md`（实现前通读）
- 通用层（`internal/common/resilience/`）不得 import：`internal/config`、`internal/domain/llmproxy/vo`、`internal/common/model` 以外的领域包；`Report(key, success)` 的成功/失败由调用方判定
- 429 不计入熔断失败；5xx 与 `*model.UpstreamConnectionError` 计入
- 每个非测试文件的函数/方法注释必须带 `@param`/`@return`/`@author centonhuang`/`@update YYYY-MM-DD HH:MM:SS`（lint-conv 强制），类型/字段注释带 `@update`
- 测试命令：聚焦 `go test -count=1 ./test/unit/resilience/`（或具体目录）；全量 `make test`；规范 `make lint`
- 配置默认值（viper key / 环境变量名以 `.`→`_` 映射）：见 spec 第 4 节表格，实现时逐字一致
- 提交信息前缀风格：`feat(resilience):`、`feat(transport):`、`feat(usecase):` 等
- 本计划在 feature worktree（如 `.worktrees/feature/api-resilience-2026-08-20`）中执行，不直接改主工作区；master 已有 `docs/superpowers/specs/2026-08-20-api-resilience-design.md`（如未提交需先提交）

---

### Task 1: 通用熔断器 + CircuitOpenError 错误类型

**Files:**
- Create: `internal/common/resilience/circuit_breaker.go`
- Create: `test/unit/resilience/circuit_breaker_test.go`
- Modify: `internal/common/model/error.go`（追加 `CircuitOpenError`）

**Interfaces:**
- Consumes: `github.com/hcd233/aris-proxy-api/internal/common/model`（仅追加错误类型）
- Produces（后续 Task 依赖的精确签名）:
  - `type BreakerState int`，常量 `StateClosed BreakerState = iota; StateOpen; StateHalfOpen`
  - `type BreakerConfig struct { Window time.Duration; MinRequests int; ErrorThreshold float64; OpenTimeout time.Duration; HalfOpenMaxRequests int }`
  - `func NewBreaker(key string, cfg BreakerConfig, onStateChange func(BreakerState)) *Breaker`
  - `func (b *Breaker) Allow() bool` — 是否放行（Open 超时自动转 HalfOpen；HalfOpen 限量放行）
  - `func (b *Breaker) Report(success bool)` — 上报结果，驱动状态转换
  - `func (b *Breaker) State() BreakerState`
  - `func (b *Breaker) RetryAfter() time.Duration` — Open 状态剩余时间
  - `type CircuitOpenError struct { Key string; RetryAfter time.Duration }`（`internal/common/model`，实现 `Error() string`）

- [ ] **Step 1: 写失败测试** `test/unit/resilience/circuit_breaker_test.go`

```go
// Package resilience 验证通用熔断器状态机、滑动窗口统计与半开探测。
package resilience_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
)

func cfg() resilience.BreakerConfig {
	return resilience.BreakerConfig{
		Window:              6 * time.Second,
		MinRequests:         3,
		ErrorThreshold:      0.5,
		OpenTimeout:         200 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
}

func TestBreaker_AllowClosed(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	if !b.Allow() {
		t.Fatal("closed state should allow requests")
	}
	if b.State() != resilience.StateClosed {
		t.Fatalf("state = %v, want closed", b.State())
	}
}

func TestBreaker_OpenWhenErrorRateExceedsThreshold(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	// 2 次成功 + 3 次失败：失败率 60% ≥ 50%，且总数 5 ≥ 3
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Report(true)
	}
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	if b.State() != resilience.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	if !b.Allow() {
		t.Fatal("open breaker must reject requests")
	}
	if b.RetryAfter() <= 0 || b.RetryAfter() > 200*time.Millisecond {
		t.Fatalf("RetryAfter = %v, want in (0, 200ms]", b.RetryAfter())
	}
}

func TestBreaker_NotOpenBelowMinRequests(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	b.Allow()
	b.Report(false)
	b.Allow()
	b.Report(false)
	if b.State() != resilience.StateClosed {
		t.Fatalf("2 requests below MinRequests=3 must stay closed, got %v", b.State())
	}
}

func TestBreaker_NotOpenBelowThreshold(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 4; i++ {
		b.Allow()
		b.Report(true)
	}
	b.Allow()
	b.Report(false) // 1/5 = 20% < 50%
	if b.State() != resilience.StateClosed {
		t.Fatalf("state = %v, want closed (below threshold)", b.State())
	}
}

func TestBreaker_HalfOpenProbeSuccessCloses(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	if b.State() != resilience.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	time.Sleep(250 * time.Millisecond) // 超过 OpenTimeout
	if !b.Allow() {
		t.Fatal("after open timeout, breaker should allow half-open probe")
	}
	if b.State() != resilience.StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
	b.Report(true)
	if b.State() != resilience.StateClosed {
		t.Fatalf("probe success should close, got %v", b.State())
	}
	if !b.Allow() {
		t.Fatal("closed breaker must allow requests")
	}
}

func TestBreaker_HalfOpenProbeFailureReopens(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	time.Sleep(250 * time.Millisecond)
	b.Allow() // 半开探测
	b.Report(false)
	if b.State() != resilience.StateOpen {
		t.Fatalf("probe failure should reopen, got %v", b.State())
	}
	if b.Allow() {
		t.Fatal("reopened breaker must reject requests")
	}
}

func TestBreaker_HalfOpenLimitsProbes(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	time.Sleep(250 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("first half-open probe must be allowed")
	}
	if b.Allow() {
		t.Fatal("second concurrent probe must be rejected (HalfOpenMaxRequests=1)")
	}
}

func TestBreaker_WindowSlides(t *testing.T) {
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false) // 3 失败，达到阈值触发 Open
	}
	if b.State() != resilience.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	time.Sleep(250 * time.Millisecond)
	b.Allow() // 半开
	b.Report(true)
	if b.State() != resilience.StateClosed {
		t.Fatalf("state = %v, want closed after probe", b.State())
	}
	// 窗口清空后，单独 1 次失败不应再次打开
	b.Allow()
	b.Report(false)
	if b.State() != resilience.StateClosed {
		t.Fatalf("single failure after reset must not open, got %v", b.State())
	}
}

func TestCircuitOpenError(t *testing.T) {
	e := &model.CircuitOpenError{Key: "k", RetryAfter: 3 * time.Second}
	if e.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
	var target *model.CircuitOpenError
	if !errors.As(e, &target) {
		t.Fatal("errors.As must match CircuitOpenError")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: 编译失败（`package github.com/hcd233/aris-proxy-api/internal/common/resilience is not in std` / `undefined: model.CircuitOpenError`）

- [ ] **Step 3: 实现熔断器** `internal/common/resilience/circuit_breaker.go`

```go
// Package resilience 通用服务容错组件（熔断/信号量隔离），不依赖任何领域类型。
package resilience

import (
	"sync"
	"time"
)

// windowBucketCount 滑动窗口固定时间桶数（窗口按此均分，统计误差 ≤ 桶宽）。
const windowBucketCount = 6

// BreakerState 熔断器状态。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type BreakerState int

const (
	// StateClosed 关闭：请求正常放行，统计滑动窗口错误率。
	StateClosed BreakerState = iota
	// StateOpen 打开：请求快速失败，等待 OpenTimeout 后进入半开。
	StateOpen
	// StateHalfOpen 半开：限量放行探测请求，成功即关闭，失败重新打开。
	StateHalfOpen
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

	state        BreakerState
	buckets      [windowBucketCount]bucket
	openSince    time.Time
	halfOpenSent int
	halfOpenOK   int

	onStateChange func(BreakerState)
}

// NewBreaker 创建熔断器。onStateChange 在状态转换后回调（锁内调用，实现需原子、不阻塞）。
//
//	@param key string 熔断维度标识（如上游 BaseURL|APIKey）
//	@param cfg BreakerConfig 配置
//	@param onStateChange func(BreakerState) 状态转换回调，可为 nil
//	@return *Breaker
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewBreaker(key string, cfg BreakerConfig, onStateChange func(BreakerState)) *Breaker {
	return &Breaker{key: key, cfg: cfg, onStateChange: onStateChange}
}

// State 返回当前状态。
//
//	@receiver b *Breaker
//	@return BreakerState
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (b *Breaker) State() BreakerState {
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
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.openSince) >= b.cfg.OpenTimeout {
			b.halfOpenSent = 0
			b.halfOpenOK = 0
			b.transitionTo(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
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

	if b.state == StateHalfOpen {
		if success {
			b.halfOpenOK++
			if b.halfOpenOK >= b.cfg.HalfOpenMaxRequests {
				b.resetWindow()
				b.transitionTo(StateClosed)
			}
		} else {
			b.openSince = time.Now()
			b.transitionTo(StateOpen)
		}
		return
	}

	b.record(success)
	if b.state == StateClosed {
		successTotal, failureTotal := b.counts()
		if successTotal+failureTotal >= int64(b.cfg.MinRequests) &&
			float64(failureTotal)/float64(successTotal+failureTotal) >= b.cfg.ErrorThreshold {
			b.openSince = time.Now()
			b.transitionTo(StateOpen)
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
	if b.state != StateOpen {
		return 0
	}
	remain := b.cfg.OpenTimeout - time.Since(b.openSince)
	if remain < 0 {
		return 0
	}
	return remain
}

func (b *Breaker) transitionTo(s BreakerState) {
	b.state = s
	if b.onStateChange != nil {
		b.onStateChange(s)
	}
}

// record 把一次结果写入当前时间桶；桶过期（槽位滚动）时重置。
func (b *Breaker) record(success bool) {
	now := time.Now()
	bw := b.cfg.Window / windowBucketCount
	idx := int(now.Unix()/int64(bw.Seconds())) % windowBucketCount
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
```

追加错误类型到 `internal/common/model/error.go`：

```go
// CircuitOpenError 熔断器打开导致的快速失败错误。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type CircuitOpenError struct {
	Key        string
	RetryAfter time.Duration
}

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("circuit breaker open for upstream %s, retry after %s", e.Key, e.RetryAfter)
}
```

（`error.go` 已 import `time` 则直接用；否则补充 `"time"` import。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: `ok`（全部用例通过；`TestBreaker_HalfOpen*` 含 sleep，总时长 < 2s）

- [ ] **Step 5: lint 并提交**

Run: `make lint`
Expected: 无新增问题（注意新文件函数注释带 @param/@return/@author/@update）

```bash
git add internal/common/model/error.go internal/common/resilience/circuit_breaker.go test/unit/resilience/circuit_breaker_test.go
git commit -m "feat(resilience): 通用三态熔断器（滑动窗口错误率 + 半开探测）与 CircuitOpenError"
```

---

### Task 2: 通用信号量 bulkhead + BulkheadFullError 错误类型

**Files:**
- Create: `internal/common/resilience/bulkhead.go`
- Create: `test/unit/resilience/bulkhead_test.go`
- Modify: `internal/common/model/error.go`（追加 `BulkheadFullError`）

**Interfaces:**
- Consumes: Task 1 的 `internal/common/model` 错误类型追加位置
- Produces:
  - `type SemaphoreConfig struct { MaxConcurrent int; AcquireTimeout time.Duration }`
  - `func NewSemaphore(cfg SemaphoreConfig) *Semaphore`
  - `func (s *Semaphore) Acquire(ctx context.Context, key string) (release func(), err error)` — 成功返回幂等 release；等待超时返回 `*model.BulkheadFullError`；ctx 取消返回 `ctx.Err()`
  - `type BulkheadFullError struct { Key string; Limit int }`（`internal/common/model`，实现 `Error() string`）

- [ ] **Step 1: 写失败测试** `test/unit/resilience/bulkhead_test.go`

```go
// Package resilience 验证通用信号量 bulkhead 的并发上限、等待超时与幂等释放。
package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
)

func TestSemaphore_AcquireRelease(t *testing.T) {
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 50 * time.Millisecond})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	release() // 幂等，不得 panic
	_, err = s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("after release should acquire again: %v", err)
	}
}

func TestSemaphore_ExceedsLimitReturnsBulkheadFull(t *testing.T) {
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 30 * time.Millisecond})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()
	_, err = s.Acquire(context.Background(), "k")
	var bf *model.BulkheadFullError
	if !errors.As(err, &bf) {
		t.Fatalf("second Acquire err = %v, want BulkheadFullError", err)
	}
	if bf.Key != "k" || bf.Limit != 1 {
		t.Fatalf("BulkheadFullError = %+v, want key=k limit=1", bf)
	}
}

func TestSemaphore_KeysIsolated(t *testing.T) {
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: 30 * time.Millisecond})
	r1, err := s.Acquire(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Acquire k1: %v", err)
	}
	defer r1()
	r2, err := s.Acquire(context.Background(), "k2") // 不同 key 不受 k1 影响
	if err != nil {
		t.Fatalf("Acquire k2 should succeed: %v", err)
	}
	r2()
}

func TestSemaphore_ContextCancel(t *testing.T) {
	s := resilience.NewSemaphore(resilience.SemaphoreConfig{MaxConcurrent: 1, AcquireTimeout: time.Second})
	release, err := s.Acquire(context.Background(), "k")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Acquire(ctx, "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire with canceled ctx err = %v, want context.Canceled", err)
	}
}

func TestBulkheadFullError(t *testing.T) {
	e := &model.BulkheadFullError{Key: "k", Limit: 3}
	if e.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: 编译失败（`undefined: resilience.NewSemaphore` 等）

- [ ] **Step 3: 实现信号量** `internal/common/resilience/bulkhead.go`

```go
package resilience

import (
	"context"
	"sync"
	"time"

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
```

追加错误类型到 `internal/common/model/error.go`：

```go
// BulkheadFullError 信号量满载（等待超时）导致的快速失败错误。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type BulkheadFullError struct {
	Key   string
	Limit int
}

func (e *BulkheadFullError) Error() string {
	return fmt.Sprintf("bulkhead full for upstream %s (limit %d)", e.Key, e.Limit)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: `ok`

- [ ] **Step 5: lint 并提交**

Run: `make lint`

```bash
git add internal/common/model/error.go internal/common/resilience/bulkhead.go test/unit/resilience/bulkhead_test.go
git commit -m "feat(resilience): 通用信号量 bulkhead（按 key 并发隔离）与 BulkheadFullError"
```

---

### Task 3: 通用 Guard 组合（Allow + Report）

**Files:**
- Create: `internal/common/resilience/guard.go`
- Create: `test/unit/resilience/guard_test.go`

**Interfaces:**
- Consumes: Task 1（`Breaker`/`BreakerConfig`/`BreakerState`）、Task 2（`Semaphore`/`SemaphoreConfig`）、`model.CircuitOpenError`/`model.BulkheadFullError`
- Produces:
  - `type GuardConfig struct { CircuitEnabled bool; CircuitWindow time.Duration; CircuitMinRequests int; CircuitErrorThreshold float64; CircuitOpenTimeout time.Duration; CircuitHalfOpenMaxRequests int; BulkheadEnabled bool; BulkheadMaxConcurrent int; BulkheadAcquireTimeout time.Duration }`
  - `type Metrics interface { SetBreakerState(key string, state BreakerState); IncCircuitOpen(key string); IncCircuitRejected(key string); IncBulkheadRejected(key string) }`
  - `func NewGuard(cfg GuardConfig, metrics Metrics) *Guard`
  - `func (g *Guard) Allow(ctx context.Context, key string) (release func(), err error)` — 熔断拒绝返回 `*model.CircuitOpenError`；bulkhead 满载返回 `*model.BulkheadFullError`；成功返回幂等 release
  - `func (g *Guard) Report(key string, success bool)`

- [ ] **Step 1: 写失败测试** `test/unit/resilience/guard_test.go`

```go
// Package resilience 验证 Guard 对熔断与信号量的组合编排、指标回调与开关语义。
package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
)

func guardCfg() resilience.GuardConfig {
	return resilience.GuardConfig{
		CircuitEnabled:           true,
		CircuitWindow:            6 * time.Second,
		CircuitMinRequests:       3,
		CircuitErrorThreshold:    0.5,
		CircuitOpenTimeout:       200 * time.Millisecond,
		CircuitHalfOpenMaxRequests: 1,
		BulkheadEnabled:          true,
		BulkheadMaxConcurrent:    1,
		BulkheadAcquireTimeout:   30 * time.Millisecond,
	}
}

type recordingMetrics struct {
	mu        sync.Mutex
	states    map[string]resilience.BreakerState
	openCalls int
	rejected  int
	bulkhead  int
}

func (m *recordingMetrics) SetBreakerState(key string, s resilience.BreakerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states == nil {
		m.states = map[string]resilience.BreakerState{}
	}
	m.states[key] = s
}
func (m *recordingMetrics) IncCircuitOpen(string)   { m.mu.Lock(); m.openCalls++; m.mu.Unlock() }
func (m *recordingMetrics) IncCircuitRejected(string) { m.mu.Lock(); m.rejected++; m.mu.Unlock() }
func (m *recordingMetrics) IncBulkheadRejected(string) { m.mu.Lock(); m.bulkhead++; m.mu.Unlock() }

func TestGuard_AllowsAndReports(t *testing.T) {
	g := resilience.NewGuard(guardCfg(), &recordingMetrics{})
	release, err := g.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	release()
	g.Report("k", true)
}

func TestGuard_OpenRejectsWithCircuitOpenError(t *testing.T) {
	m := &recordingMetrics{}
	g := resilience.NewGuard(guardCfg(), m)
	for i := 0; i < 3; i++ {
		release, err := g.Allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		release()
		g.Report("k", false)
	}
	_, err := g.Allow(context.Background(), "k")
	var ce *model.CircuitOpenError
	if !errors.As(err, &ce) {
		t.Fatalf("Allow err = %v, want CircuitOpenError", err)
	}
	if ce.Key != "k" {
		t.Fatalf("Key = %q, want k", ce.Key)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rejected != 1 {
		t.Fatalf("rejected metric = %d, want 1", m.rejected)
	}
	if m.states["k"] != resilience.StateOpen {
		t.Fatalf("state metric = %v, want open", m.states["k"])
	}
}

func TestGuard_BulkheadFull(t *testing.T) {
	g := resilience.NewGuard(guardCfg(), &recordingMetrics{})
	release, err := g.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("first Allow: %v", err)
	}
	defer release()
	_, err = g.Allow(context.Background(), "k")
	var bf *model.BulkheadFullError
	if !errors.As(err, &bf) {
		t.Fatalf("second Allow err = %v, want BulkheadFullError", err)
	}
}

func TestGuard_CircuitDisabledAlwaysAllows(t *testing.T) {
	cfg := guardCfg()
	cfg.CircuitEnabled = false
	g := resilience.NewGuard(cfg, nil)
	for i := 0; i < 5; i++ {
		release, err := g.Allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("Allow #%d with circuit disabled: %v", i, err)
		}
		release()
		g.Report("k", false) // 熔断关闭时 Report 不生效
	}
}

func TestGuard_BulkheadDisabled(t *testing.T) {
	cfg := guardCfg()
	cfg.BulkheadEnabled = false
	g := resilience.NewGuard(cfg, nil)
	release, err := g.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	release()
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: 编译失败（`undefined: resilience.GuardConfig` 等）

- [ ] **Step 3: 实现 Guard** `internal/common/resilience/guard.go`

```go
package resilience

import (
	"context"
	"errors"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// GuardConfig Guard 组合配置（熔断 + 信号量）。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type GuardConfig struct {
	CircuitEnabled            bool
	CircuitWindow             time.Duration
	CircuitMinRequests        int
	CircuitErrorThreshold     float64
	CircuitOpenTimeout        time.Duration
	CircuitHalfOpenMaxRequests int
	BulkheadEnabled           bool
	BulkheadMaxConcurrent     int
	BulkheadAcquireTimeout    time.Duration
}

// Metrics 容错事件指标回调（由接入方实现并注册到 Prometheus；nil 表示不采集）。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type Metrics interface {
	// SetBreakerState 熔断状态变化（0=closed, 1=open, 2=half-open）。
	SetBreakerState(key string, state BreakerState)
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
	cfg     GuardConfig
	metrics Metrics
	mu      sync.Mutex
	breakers map[string]*Breaker
	sem     *Semaphore
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
			MaxConcurrent: cfg.BulkheadMaxConcurrent,
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
	if g.cfg.CircuitEnabled && !g.breaker(key).Allow() {
		if g.metrics != nil {
			g.metrics.IncCircuitRejected(key)
		}
		return nil, &model.CircuitOpenError{Key: key, RetryAfter: g.breaker(key).RetryAfter()}
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
		}, func(s BreakerState) {
			if g.metrics != nil {
				g.metrics.SetBreakerState(key, s)
				if s == StateOpen {
					g.metrics.IncCircuitOpen(key)
				}
			}
		})
		g.breakers[key] = b
	}
	return b
}
```

注意 `guard.go` 需要 import `"time"`（GuardConfig 使用）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/resilience/`
Expected: `ok`（guard_test + 前两任务用例全绿）

- [ ] **Step 5: lint 并提交**

Run: `make lint`

```bash
git add internal/common/resilience/guard.go test/unit/resilience/guard_test.go
git commit -m "feat(resilience): Guard 组合（熔断 Allow + 信号量 Acquire + Report）与指标回调接口"
```

---

### Task 4: 熔断/bulkhead 配置项

**Files:**
- Modify: `internal/config/config.go`（3 处：字段声明区 ≈L180-200、`SetDefault` 区 ≈L258-261、读取区 ≈L325-330）
- Create: `test/unit/config/resilience_config_test.go`

**Interfaces:**
- Consumes: Task 3 的 `GuardConfig` 字段名（本任务只提供全局变量，不依赖 resilience 包）
- Produces（供 Task 5 的 `NewEndpointGuard` 读取）:
  - `config.UpstreamCircuitEnabled bool`（默认 true）
  - `config.UpstreamCircuitWindow time.Duration`（默认 60s）
  - `config.UpstreamCircuitMinRequests int`（默认 10）
  - `config.UpstreamCircuitErrorThreshold float64`（默认 0.5）
  - `config.UpstreamCircuitOpenTimeout time.Duration`（默认 30s）
  - `config.UpstreamCircuitHalfOpenMaxRequests int`（默认 1）
  - `config.UpstreamBulkheadEnabled bool`（默认 true）
  - `config.UpstreamBulkheadMaxConcurrent int`（默认 32）
  - `config.UpstreamBulkheadAcquireTimeout time.Duration`（默认 1s）

- [ ] **Step 1: 写失败测试** `test/unit/config/resilience_config_test.go`

```go
// Package config 验证熔断/bulkhead 配置默认值与环境变量覆盖。
package config

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/config"
)

// TestUpstreamCircuitEnv 验证环境变量可覆盖熔断/bulkhead 配置（子进程方式，与 HTTPClientTimeout 测试同模式）。
func TestUpstreamCircuitEnv(t *testing.T) {
	t.Parallel()
	if os.Getenv("_UPSTREAM_CIRCUIT_CHECK") == "1" {
		config.InitEnvironment()
		if config.UpstreamCircuitWindow != 10*time.Second {
			t.Fatalf("UpstreamCircuitWindow = %v, want 10s", config.UpstreamCircuitWindow)
		}
		if config.UpstreamCircuitMinRequests != 5 {
			t.Fatalf("UpstreamCircuitMinRequests = %d, want 5", config.UpstreamCircuitMinRequests)
		}
		if config.UpstreamBulkheadMaxConcurrent != 16 {
			t.Fatalf("UpstreamBulkheadMaxConcurrent = %d, want 16", config.UpstreamBulkheadMaxConcurrent)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestUpstreamCircuitEnv")
	cmd.Env = append(os.Environ(),
		"UPSTREAM_CIRCUIT_WINDOW=10s",
		"UPSTREAM_CIRCUIT_MIN_REQUESTS=5",
		"UPSTREAM_BULKHEAD_MAX_CONCURRENT=16",
		"_UPSTREAM_CIRCUIT_CHECK=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestUpstreamCircuitDefaults 验证未配置时使用 spec 默认值。
func TestUpstreamCircuitDefaults(t *testing.T) {
	t.Parallel()
	config.InitEnvironment()
	if !config.UpstreamCircuitEnabled {
		t.Fatal("UpstreamCircuitEnabled = false, want true")
	}
	if config.UpstreamCircuitWindow != 60*time.Second {
		t.Fatalf("UpstreamCircuitWindow = %v, want 60s", config.UpstreamCircuitWindow)
	}
	if config.UpstreamCircuitMinRequests != 10 {
		t.Fatalf("UpstreamCircuitMinRequests = %d, want 10", config.UpstreamCircuitMinRequests)
	}
	if config.UpstreamCircuitErrorThreshold != 0.5 {
		t.Fatalf("UpstreamCircuitErrorThreshold = %v, want 0.5", config.UpstreamCircuitErrorThreshold)
	}
	if config.UpstreamCircuitOpenTimeout != 30*time.Second {
		t.Fatalf("UpstreamCircuitOpenTimeout = %v, want 30s", config.UpstreamCircuitOpenTimeout)
	}
	if config.UpstreamCircuitHalfOpenMaxRequests != 1 {
		t.Fatalf("UpstreamCircuitHalfOpenMaxRequests = %d, want 1", config.UpstreamCircuitHalfOpenMaxRequests)
	}
	if !config.UpstreamBulkheadEnabled {
		t.Fatal("UpstreamBulkheadEnabled = false, want true")
	}
	if config.UpstreamBulkheadMaxConcurrent != 32 {
		t.Fatalf("UpstreamBulkheadMaxConcurrent = %d, want 32", config.UpstreamBulkheadMaxConcurrent)
	}
	if config.UpstreamBulkheadAcquireTimeout != time.Second {
		t.Fatalf("UpstreamBulkheadAcquireTimeout = %v, want 1s", config.UpstreamBulkheadAcquireTimeout)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/config/`
Expected: 编译失败（`config.UpstreamCircuitWindow undefined`）

- [ ] **Step 3: 实现配置项** `internal/config/config.go`

三处修改（字段声明区、`SetDefault`、`InitEnvironment` 读取区），逐字插入：

字段声明（放在 `UpstreamRetryJitterFactor` 之后、`HTTPClientTimeout` 之前）：

```go
	// UpstreamCircuitEnabled bool 是否启用上游熔断
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitEnabled bool

	// UpstreamCircuitWindow time.Duration 熔断滑动窗口时长
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitWindow time.Duration

	// UpstreamCircuitMinRequests int 窗口内最少请求数（低于该值不熔断）
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitMinRequests int

	// UpstreamCircuitErrorThreshold float64 熔断错误率阈值 (0~1)
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitErrorThreshold float64

	// UpstreamCircuitOpenTimeout time.Duration 熔断打开保持时长
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitOpenTimeout time.Duration

	// UpstreamCircuitHalfOpenMaxRequests int 半开期并发探测请求数
	//	@update 2026-08-20 10:00:00
	UpstreamCircuitHalfOpenMaxRequests int

	// UpstreamBulkheadEnabled bool 是否启用每上游并发隔离
	//	@update 2026-08-20 10:00:00
	UpstreamBulkheadEnabled bool

	// UpstreamBulkheadMaxConcurrent int 每上游最大并发转发数
	//	@update 2026-08-20 10:00:00
	UpstreamBulkheadMaxConcurrent int

	// UpstreamBulkheadAcquireTimeout time.Duration 获取信号量等待超时
	//	@update 2026-08-20 10:00:00
	UpstreamBulkheadAcquireTimeout time.Duration
```

`SetDefault`（放在 `upstream.retry.*` 之后）：

```go
	config.SetDefault("upstream.circuit.enabled", true)
	config.SetDefault("upstream.circuit.window", 60*time.Second)
	config.SetDefault("upstream.circuit.min_requests", 10)
	config.SetDefault("upstream.circuit.error_threshold", 0.5)
	config.SetDefault("upstream.circuit.open_timeout", 30*time.Second)
	config.SetDefault("upstream.circuit.halfopen_max_requests", 1)
	config.SetDefault("upstream.bulkhead.enabled", true)
	config.SetDefault("upstream.bulkhead.max_concurrent", 32)
	config.SetDefault("upstream.bulkhead.acquire_timeout", time.Second)
```

读取（放在 `UpstreamRetryJitterFactor` 读取之后）：

```go
	UpstreamCircuitEnabled = config.GetBool("upstream.circuit.enabled")
	UpstreamCircuitWindow = config.GetDuration("upstream.circuit.window")
	UpstreamCircuitMinRequests = config.GetInt("upstream.circuit.min_requests")
	UpstreamCircuitErrorThreshold = config.GetFloat64("upstream.circuit.error_threshold")
	UpstreamCircuitOpenTimeout = config.GetDuration("upstream.circuit.open_timeout")
	UpstreamCircuitHalfOpenMaxRequests = config.GetInt("upstream.circuit.halfopen_max_requests")
	UpstreamBulkheadEnabled = config.GetBool("upstream.bulkhead.enabled")
	UpstreamBulkheadMaxConcurrent = config.GetInt("upstream.bulkhead.max_concurrent")
	UpstreamBulkheadAcquireTimeout = config.GetDuration("upstream.bulkhead.acquire_timeout")
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/config/`
Expected: `ok`（新用例 + 既有 `HTTPClientTimeout` 用例全绿）

- [ ] **Step 5: lint 并提交**

Run: `make lint`

```bash
git add internal/config/config.go test/unit/config/resilience_config_test.go
git commit -m "feat(config): 熔断/bulkhead 9 个配置项（UPSTREAM_CIRCUIT_*/UPSTREAM_BULKHEAD_*）"
```

---

### Task 5: 指标常量 + proxy 适配层（EndpointGuard）

**Files:**
- Modify: `internal/common/constant/metrics.go`（追加 4 组指标名/Help 常量 + `MetricLabelKey`）
- Create: `internal/infrastructure/transport/guard.go`
- Create: `test/unit/transport/guard_test.go`

**Interfaces:**
- Consumes: Task 3 的 `resilience.Guard`/`resilience.GuardConfig`/`resilience.Metrics`/`resilience.BreakerState`；Task 4 的 9 个 config 全局变量；`vo.UpstreamEndpoint`；`model.UpstreamError`/`model.UpstreamConnectionError`
- Produces（供 Task 6 接线）:
  - `func NewEndpointGuard(registry *prometheus.Registry) *EndpointGuard`
  - `type EndpointGuard struct`（含 `guard *resilience.Guard` 与 `Allow(ctx, key) (func(), error)`、`Report(key, success)` 委托方法）
  - 包级 `func endpointKey(ep vo.UpstreamEndpoint) string`
  - 包级 `func isCircuitError(err error) bool`

- [ ] **Step 1: 写失败测试** `test/unit/transport/guard_test.go`

```go
// Package transport_test 验证 proxy 适配层：endpoint key 生成、熔断错误分类、EndpointGuard 组装。
package transport_test

import (
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

func TestEndpointKey(t *testing.T) {
	t.Parallel()
	ep := vo.UpstreamEndpoint{Model: "m", APIKey: "secret-key", BaseURL: "https://api.example.com"}
	if got := transport.EndpointKey(ep); got != "https://api.example.com|secret-key" {
		t.Fatalf("EndpointKey = %q", got)
	}
}
```

注意：若 `endpointKey` 以未导出函数实现，测试需走白盒；本计划按**导出** `EndpointKey` 实现（见 Step 3），便于测试与复用。

```go
func TestIsCircuitError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"5xx 计入", &model.UpstreamError{StatusCode: 503}, true},
		{"502 计入", &model.UpstreamError{StatusCode: 502}, true},
		{"网络错误计入", &model.UpstreamConnectionError{Cause: errors.New("timeout")}, true},
		{"429 不计入", &model.UpstreamError{StatusCode: 429}, false},
		{"404 不计入", &model.UpstreamError{StatusCode: 404}, false},
		{"ierr 不计入", ierr.New(ierr.ErrProxyRequest, "build request"), false},
		{"nil 不计入", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := transport.IsCircuitError(tt.err); got != tt.want {
				t.Errorf("IsCircuitError(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNewEndpointGuardNilRegistry(t *testing.T) {
	t.Parallel()
	// nil registry 应跳过指标注册且不 panic
	g := transport.NewEndpointGuard(nil)
	if g == nil {
		t.Fatal("NewEndpointGuard(nil) = nil")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/transport/`
Expected: 编译失败（`undefined: transport.EndpointKey` 等）

- [ ] **Step 3: 追加指标常量** `internal/common/constant/metrics.go`

```go
	// MetricNamespaceUpstream 上游容错指标命名空间（最终指标名形如 upstream_circuit_state）
	//	@update 2026-08-20 10:00:00
	MetricNamespaceUpstream = "upstream"

	// MetricUpstreamCircuitStateName 熔断状态 gauge（0=closed, 1=open, 2=half-open）
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitStateName = "circuit_state"
	// MetricUpstreamCircuitStateHelp
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitStateHelp = "Upstream circuit breaker state (0=closed, 1=open, 2=half-open)"

	// MetricUpstreamCircuitOpenTotalName 熔断打开次数 counter
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitOpenTotalName = "circuit_open_total"
	// MetricUpstreamCircuitOpenTotalHelp
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitOpenTotalHelp = "Total circuit breaker open transitions"

	// MetricUpstreamCircuitRejectedTotalName 熔断拒绝请求数 counter
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitRejectedTotalName = "circuit_rejected_total"
	// MetricUpstreamCircuitRejectedTotalHelp
	//	@update 2026-08-20 10:00:00
	MetricUpstreamCircuitRejectedTotalHelp = "Total requests rejected by open circuit breaker"

	// MetricUpstreamBulkheadRejectedTotalName 信号量满载拒绝请求数 counter
	//	@update 2026-08-20 10:00:00
	MetricUpstreamBulkheadRejectedTotalName = "bulkhead_rejected_total"
	// MetricUpstreamBulkheadRejectedTotalHelp
	//	@update 2026-08-20 10:00:00
	MetricUpstreamBulkheadRejectedTotalHelp = "Total requests rejected by full bulkhead"

	// MetricLabelKey 容错指标的 key 标签（上游 BaseURL|APIKey）
	//	@update 2026-08-20 10:00:00
	MetricLabelKey = "key"
```

（指标全名 = `upstream_circuit_state`、`upstream_circuit_open_total`、`upstream_circuit_rejected_total`、`upstream_bulkhead_rejected_total`。）

- [ ] **Step 4: 实现适配层** `internal/infrastructure/transport/guard.go`

```go
package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointKey 生成上游 endpoint 的熔断/隔离 key：BaseURL|APIKey。
// 同源（同 baseURL 同 key）的不同模型共享熔断状态与并发限制。
//
//	@param ep vo.UpstreamEndpoint
//	@return string
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func EndpointKey(ep vo.UpstreamEndpoint) string {
	return ep.BaseURL + "|" + ep.APIKey
}

// IsCircuitError 判断上游错误是否计入熔断失败。
//
// 计入：网络层错误（*model.UpstreamConnectionError）、5xx（*model.UpstreamError StatusCode >= 500）。
// 不计入：429（上游存活仅限流）、其他 4xx、本地构建错误。
//
//	@param err error
//	@return bool
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func IsCircuitError(err error) bool {
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return true
	}
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode >= http.StatusInternalServerError
	}
	return false
}

// EndpointGuard proxy 链路的容错守卫：熔断 + 信号量组合，对上游 endpoint 维度生效。
type EndpointGuard struct {
	guard *resilience.Guard
}

// NewEndpointGuard 从 config 全局变量组装通用 Guard，并把指标注册到 registry。
// registry 为 nil 时跳过指标注册（测试/无指标环境）。
//
//	@param registry *prometheus.Registry
//	@return *EndpointGuard
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewEndpointGuard(registry *prometheus.Registry) *EndpointGuard {
	var m resilience.Metrics
	if registry != nil {
		m = newGuardMetrics(registry)
	}
	cfg := resilience.GuardConfig{
		CircuitEnabled:            config.UpstreamCircuitEnabled,
		CircuitWindow:             config.UpstreamCircuitWindow,
		CircuitMinRequests:        config.UpstreamCircuitMinRequests,
		CircuitErrorThreshold:     config.UpstreamCircuitErrorThreshold,
		CircuitOpenTimeout:        config.UpstreamCircuitOpenTimeout,
		CircuitHalfOpenMaxRequests: config.UpstreamCircuitHalfOpenMaxRequests,
		BulkheadEnabled:           config.UpstreamBulkheadEnabled,
		BulkheadMaxConcurrent:     config.UpstreamBulkheadMaxConcurrent,
		BulkheadAcquireTimeout:    config.UpstreamBulkheadAcquireTimeout,
	}
	return &EndpointGuard{guard: resilience.NewGuard(cfg, m)}
}

// Allow 放行则返回幂等 release；熔断打开/满载返回错误。
//
//	@param ctx context.Context
//	@param key string
//	@return release func()
//	@return err error
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *EndpointGuard) Allow(ctx context.Context, key string) (func(), error) {
	return g.guard.Allow(ctx, key)
}

// Report 上报一次调用结果。
//
//	@param key string
//	@param success bool
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *EndpointGuard) Report(key string, success bool) {
	g.guard.Report(key, success)
}

// guardMetrics resilience.Metrics 的 Prometheus 实现。
type guardMetrics struct {
	state     *prometheus.GaugeVec
	open      *prometheus.CounterVec
	rejected  *prometheus.CounterVec
	bulkhead  *prometheus.CounterVec
}

func newGuardMetrics(registry *prometheus.Registry) *guardMetrics {
	m := &guardMetrics{
		state: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitStateName,
			Help:      constant.MetricUpstreamCircuitStateHelp,
		}, []string{constant.MetricLabelKey}),
		open: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitOpenTotalName,
			Help:      constant.MetricUpstreamCircuitOpenTotalHelp,
		}, []string{constant.MetricLabelKey}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitRejectedTotalName,
			Help:      constant.MetricUpstreamCircuitRejectedTotalHelp,
		}, []string{constant.MetricLabelKey}),
		bulkhead: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamBulkheadRejectedTotalName,
			Help:      constant.MetricUpstreamBulkheadRejectedTotalHelp,
		}, []string{constant.MetricLabelKey}),
	}
	registry.MustRegister(m.state, m.open, m.rejected, m.bulkhead)
	return m
}

func (m *guardMetrics) SetBreakerState(key string, state resilience.BreakerState) {
	m.state.WithLabelValues(key).Set(float64(state))
}

func (m *guardMetrics) IncCircuitOpen(key string) {
	m.open.WithLabelValues(key).Inc()
}

func (m *guardMetrics) IncCircuitRejected(key string) {
	m.rejected.WithLabelValues(key).Inc()
}

func (m *guardMetrics) IncBulkheadRejected(key string) {
	m.bulkhead.WithLabelValues(key).Inc()
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/transport/`
Expected: `ok`（guard_test + 既有 retry_test 全绿）

- [ ] **Step 6: lint 并提交**

Run: `make lint`

```bash
git add internal/common/constant/metrics.go internal/infrastructure/transport/guard.go test/unit/transport/guard_test.go
git commit -m "feat(transport): 上游 endpoint 容错适配层（EndpointKey/IsCircuitError/EndpointGuard + Prometheus 指标）"
```

---

### Task 6: proxy 链路接入 + fx 接线 + 集成测试

**Files:**
- Modify: `internal/infrastructure/transport/openai.go`（`openAIProxy` 结构、`NewOpenAIProxy`、`doUpstreamRequest`）
- Modify: `internal/infrastructure/transport/anthropic.go`（`anthropicProxy` 结构、`NewAnthropicProxy`、`sendRequest`）
- Modify: `internal/bootstrap/modules/repository.go`（`NewEndpointGuard`、`NewOpenAIProxy`/`NewAnthropicProxy` 接线）
- Create: `test/unit/transport/guard_integration_test.go`

**Interfaces:**
- Consumes: Task 5 的 `EndpointGuard`/`EndpointKey`/`IsCircuitError`；`inflight.Tracker`
- Produces: 修改后的 `transport.NewOpenAIProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.OpenAIProxyPort`、`transport.NewAnthropicProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.AnthropicProxyPort`

- [ ] **Step 1: 写失败集成测试** `test/unit/transport/guard_integration_test.go`

```go
// Package transport_test 验证 guard 接入 proxy 链路后的端到端行为：
// 上游持续 5xx → 熔断打开 → 请求不达上游（503 快速失败）→ 上游恢复 → 半开探测通过 → 全量恢复。
package transport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// guardIntegrationEnv 用测试参数覆盖 config 全局变量，并在结束后恢复。
func guardIntegrationEnv(t *testing.T) {
	t.Helper()
	orig := []any{
		config.UpstreamRetryMaxAttempts,
		config.UpstreamCircuitWindow,
		config.UpstreamCircuitMinRequests,
		config.UpstreamCircuitErrorThreshold,
		config.UpstreamCircuitOpenTimeout,
		config.UpstreamCircuitHalfOpenMaxRequests,
		config.UpstreamBulkheadEnabled,
	}
	t.Cleanup(func() {
		config.UpstreamRetryMaxAttempts = orig[0].(int)
		config.UpstreamCircuitWindow = orig[1].(time.Duration)
		config.UpstreamCircuitMinRequests = orig[2].(int)
		config.UpstreamCircuitErrorThreshold = orig[3].(float64)
		config.UpstreamCircuitOpenTimeout = orig[4].(time.Duration)
		config.UpstreamCircuitHalfOpenMaxRequests = orig[5].(int)
		config.UpstreamBulkheadEnabled = orig[6].(bool)
	})
	config.UpstreamRetryMaxAttempts = 0 // 关闭重试，让单次 5xx 直接计入熔断
	config.UpstreamCircuitWindow = 6 * time.Second
	config.UpstreamCircuitMinRequests = 3
	config.UpstreamCircuitErrorThreshold = 0.5
	config.UpstreamCircuitOpenTimeout = 300 * time.Millisecond
	config.UpstreamCircuitHalfOpenMaxRequests = 1
	config.UpstreamBulkheadEnabled = false // 隔离本用例只验证熔断
}

func TestGuardIntegration_OpenThenRecover(t *testing.T) {
	guardIntegrationEnv(t)

	var calls atomic.Int32
	var healthy atomic.Bool
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"m","choices":[]}`))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), guard)
	ep := vo.UpstreamEndpoint{Model: "m", APIKey: "k", BaseURL: mock.URL}
	ctx := context.Background()

	// 3 次失败（MinRequests=3）→ 熔断打开
	for i := 0; i < 3; i++ {
		_, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`))
		if err == nil {
			t.Fatalf("request #%d should fail", i)
		}
	}

	// 熔断打开：第 4 个请求快速失败，且不达上游
	before := calls.Load()
	_, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`))
	var ce *model.CircuitOpenError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CircuitOpenError", err)
	}
	if calls.Load() != before {
		t.Fatalf("open state must not reach upstream: calls %d -> %d", before, calls.Load())
	}

	// 上游恢复，等待 OpenTimeout 后半开探测通过 → 恢复
	healthy.Store(true)
	time.Sleep(350 * time.Millisecond)
	resp, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`))
	if err != nil {
		t.Fatalf("after recovery should succeed: %v", err)
	}
	if resp == nil || resp.ID != "1" {
		t.Fatalf("resp = %+v, want id=1", resp)
	}

	// 恢复后请求正常放行
	if _, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`)); err != nil {
		t.Fatalf("steady state request failed: %v", err)
	}
}
```

（mock 返回的 `{"id":"1","object":"chat.completion","model":"m","choices":[]}` 与 `dto.OpenAIChatCompletion` 的 `ID string` 字段对应，`ForwardChatCompletion` 解析后 `resp.ID == "1"`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run TestGuardIntegration_OpenThenRecover ./test/unit/transport/`
Expected: 编译失败（`NewOpenAIProxy` 参数个数不匹配 / `NewEndpointGuard` undefined 等，取决于 Step 3/4 执行顺序；先实现 Step 4 依赖再跑）

- [ ] **Step 3: 接入 openai.go**

`internal/infrastructure/transport/openai.go` 修改 3 处：

```go
type openAIProxy struct {
	tracker *inflight.Tracker
	guard   *EndpointGuard
}
```

```go
func NewOpenAIProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.OpenAIProxyPort {
	return &openAIProxy{tracker: tracker, guard: guard}
}
```

`doUpstreamRequest` 整体替换为：

```go
// doUpstreamRequest 构建并发送上游 HTTP 请求：先过容错守卫（熔断/信号量），再对可重试错误自动重试。
// ctx 融合 drain 广播：优雅退出 soft deadline 到达时取消上游连接，
// 使阻塞的 SSE 读循环返回 context canceled（礼貌断流的前半段）。
func (p *openAIProxy) doUpstreamRequest(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
	ctx = p.tracker.CancelOnDrain(ctx)
	key := EndpointKey(ep)
	release, err := p.guard.Allow(ctx, key)
	if err != nil {
		return nil, err
	}
	defer release()

	sendFn := func() (*http.Response, error) {
		return p.sendUpstreamRequestOnce(ctx, ep, body, pathSuffix)
	}
	resp, err := SendUpstreamWithRetry(ctx, constant.ModuleOpenAIProxy, sendFn)
	p.guard.Report(key, !IsCircuitError(err))
	return resp, err
}
```

- [ ] **Step 4: 接入 anthropic.go**

```go
type anthropicProxy struct {
	tracker *inflight.Tracker
	guard   *EndpointGuard
}

func NewAnthropicProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.AnthropicProxyPort {
	return &anthropicProxy{tracker: tracker, guard: guard}
}
```

`sendRequest` 整体替换为：

```go
// sendRequest 构建并发送 Anthropic 协议的上游请求：先过容错守卫（熔断/信号量），再对可重试错误自动重试。
// ctx 融合 drain 广播：优雅退出 soft deadline 到达时取消上游连接，
// 使阻塞的 SSE 读循环返回 context canceled（礼貌断流的前半段）。
func (p *anthropicProxy) sendRequest(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	ctx = p.tracker.CancelOnDrain(ctx)
	key := EndpointKey(ep)
	release, err := p.guard.Allow(ctx, key)
	if err != nil {
		return nil, err
	}
	defer release()

	sendFn := func() (*http.Response, error) {
		return p.sendRequestOnce(ctx, ep, path, body)
	}
	resp, err := SendUpstreamWithRetry(ctx, constant.ModuleAnthropicProxy, sendFn)
	p.guard.Report(key, !IsCircuitError(err))
	return resp, err
}
```

- [ ] **Step 5: fx 接线** `internal/bootstrap/modules/repository.go`

```go
// fx.Provide 列表追加 NewEndpointGuard（放在 NewOpenAIProxy 之前）：
// 		NewEndpointGuard,
```

新增构造与接线（原函数替换）：

```go
func NewEndpointGuard(registry *prometheus.Registry) *transport.EndpointGuard {
	return transport.NewEndpointGuard(registry)
}

func NewOpenAIProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.OpenAIProxyPort {
	return transport.NewOpenAIProxy(tracker, guard)
}

func NewAnthropicProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.AnthropicProxyPort {
	return transport.NewAnthropicProxy(tracker, guard)
}
```

`repository.go` import 增加 `"github.com/prometheus/client_golang/prometheus"`。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/transport/ && go build ./cmd/server`
Expected: `ok` + 构建成功（fx 接线在启动时验证，`go vet ./internal/bootstrap/...` 通过）

Run: `go vet ./internal/bootstrap/... ./internal/infrastructure/transport/...`
Expected: 无输出

- [ ] **Step 7: lint 并提交**

Run: `make lint`

```bash
git add internal/infrastructure/transport/openai.go internal/infrastructure/transport/anthropic.go internal/bootstrap/modules/repository.go test/unit/transport/guard_integration_test.go
git commit -m "feat(transport): proxy 链路接入容错守卫（熔断+信号量）并完成 fx 接线"
```

---

### Task 7: 降级映射（usecase → 503 + Retry-After）

**Files:**
- Modify: `internal/application/llmproxy/usecase/common.go`（`upstreamProxyError` 追加两个分支 + 两个 fallback body 常量）
- Modify: `internal/common/constant/http.go`（追加 `BulkheadRetryAfterSeconds`）
- Create: `internal/application/llmproxy/usecase/upstream_guard_error_test.go`（白盒测试，`package usecase`，与 usecase 包同目录——项目惯例测试放 `test/unit/`，但 `upstreamProxyError` 未导出且无导出入口，白盒是唯一可测途径，`make test` 已覆盖 `./internal/...`）

**Interfaces:**
- Consumes: `model.CircuitOpenError`/`model.BulkheadFullError`、`port.ProxyError`、`enum.ProtocolKind`
- Produces: 修改后的 `upstreamProxyError(err, protocol, fallbackBody)` 行为：熔断/满载错误 → 503 + `Retry-After` + 协议错误体

- [ ] **Step 1: 写失败测试** `internal/application/llmproxy/usecase/upstream_guard_error_test.go`

```go
package usecase

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

func TestUpstreamProxyError_CircuitOpen(t *testing.T) {
	t.Parallel()
	err := &model.CircuitOpenError{Key: "k", RetryAfter: 3 * time.Second}
	pe := upstreamProxyError(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "3" {
		t.Fatalf("Retry-After = %q, want 3", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	if !strings.Contains(string(pe.Body), `"circuit_open"`) {
		t.Fatalf("Body = %s, want circuit_open error type", pe.Body)
	}
	if pe.Cause != err {
		t.Fatalf("Cause not preserved")
	}
}

func TestUpstreamProxyError_CircuitOpenAnthropic(t *testing.T) {
	t.Parallel()
	err := &model.CircuitOpenError{Key: "k", RetryAfter: 1*time.Second + 500*time.Millisecond}
	pe := upstreamProxyError(err, enum.ProtocolKindAnthropic, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "2" {
		t.Fatalf("Retry-After = %q, want 2 (ceil 1.5s)", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	if !strings.Contains(string(pe.Body), `"overloaded_error"`) {
		t.Fatalf("Body = %s, want overloaded_error error type", pe.Body)
	}
}

func TestUpstreamProxyError_BulkheadFull(t *testing.T) {
	t.Parallel()
	err := &model.BulkheadFullError{Key: "k", Limit: 32}
	pe := upstreamProxyError(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "5" {
		t.Fatalf("Retry-After = %q, want 5", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	if !strings.Contains(string(pe.Body), `"bulkhead_full"`) {
		t.Fatalf("Body = %s, want bulkhead_full error type", pe.Body)
	}
}

func TestUpstreamProxyError_UpstreamErrorStillMapped(t *testing.T) {
	t.Parallel()
	err := &model.UpstreamError{StatusCode: 502, Body: "bad gateway"}
	pe := upstreamProxyError(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != 502 {
		t.Fatalf("StatusCode = %d, want 502", pe.StatusCode)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./internal/application/llmproxy/usecase/`
Expected: FAIL（`upstreamProxyError` 返回 502 fallback，Status 断言失败）

- [ ] **Step 3: 实现降级映射**

`internal/common/constant/http.go` 追加：

```go
	// BulkheadRetryAfterSeconds 信号量满载时建议客户端重试的秒数
	//	@update 2026-08-20 10:00:00
	BulkheadRetryAfterSeconds = 5
```

`internal/application/llmproxy/usecase/common.go` 修改：`upstreamProxyError` 在 `errors.As(upstreamErr)` 分支之后追加：

```go
	var circuitErr *model.CircuitOpenError
	if errors.As(err, &circuitErr) {
		return guardRejectedProxyError(circuitErr, protocol, guardOpenFallbackBody(protocol), int(math.Ceil(circuitErr.RetryAfter.Seconds())))
	}
	var bulkheadErr *model.BulkheadFullError
	if errors.As(err, &bulkheadErr) {
		return guardRejectedProxyError(bulkheadErr, protocol, guardFullFallbackBody(protocol), constant.BulkheadRetryAfterSeconds)
	}
```

并在同文件追加辅助（放在 `upstreamProxyError` 之后）：

```go
// guardRejectedProxyError 把熔断/满载错误映射为 503 + Retry-After 的降级响应。
func guardRejectedProxyError(cause error, protocol enum.ProtocolKind, body []byte, retryAfter int) *port.ProxyError {
	if retryAfter < 1 {
		retryAfter = 1
	}
	return &port.ProxyError{
		StatusCode: http.StatusServiceUnavailable,
		Headers: map[string]string{
			constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON,
			constant.HTTPHeaderRetryAfter:  strconv.Itoa(retryAfter),
		},
		Body:     body,
		Cause:    cause,
		Protocol: protocol,
	}
}

// guardOpenFallbackBody 熔断打开的降级错误体（按协议格式）。
func guardOpenFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游服务暂时不可用，请稍后重试或更换模型"}}`)
	}
	return []byte(`{"error":{"message":"上游服务暂时不可用，请稍后重试或更换模型","type":"circuit_open","code":503}}`)
}

// guardFullFallbackBody 信号量满载的降级错误体（按协议格式）。
func guardFullFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游负载过高，请稍后重试"}}`)
	}
	return []byte(`{"error":{"message":"上游负载过高，请稍后重试","type":"bulkhead_full","code":503}}`)
}
```

`common.go` 的 import 增加 `"math"`、`"strconv"`（检查是否已有，无则补）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./internal/application/llmproxy/usecase/`
Expected: `ok`（新用例全绿；注意该目录既有用例需要 `t.Parallel()` 兼容——新用例已带）

Run: `go test -count=1 ./internal/...`（确认无回归）
Expected: `ok`

- [ ] **Step 5: lint 并提交**

Run: `make lint`

```bash
git add internal/common/constant/http.go internal/application/llmproxy/usecase/common.go internal/application/llmproxy/usecase/upstream_guard_error_test.go
git commit -m "feat(usecase): 熔断/满载错误降级为 503 + Retry-After 协议错误体"
```

---

### Task 8: E2E 用例（回归 + 指标存在性）

**Files:**
- Create: `test/e2e/upstream_resilience/upstream_resilience_test.go`
- Create: `test/e2e/upstream_resilience/fixtures/requests/simple_chat.json`（最小聊天请求 fixture，参照 `test/e2e/openai_chat_completion/fixtures/requests/tool_call_non_stream.json` 的字段风格，仅 `model` + `messages`）

**Interfaces:**
- Consumes: 部署后的真实 API（`POST /api/openai/v1/chat/completions` 与 `GET /metrics` 均公开可访问）；环境变量 `BASE_URL`、`API_KEY`（与既有 e2e 相同机制，缺失则 skip）
- Produces: E2E 回归用例（部署后执行）

- [ ] **Step 1: 写 fixture** `test/e2e/upstream_resilience/fixtures/requests/simple_chat.json`

```json
{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}
```

- [ ] **Step 2: 写 E2E 用例** `test/e2e/upstream_resilience/upstream_resilience_test.go`

```go
// Package upstream_resilience E2E 验证容错默认配置下正常链路不受影响、熔断指标已注册。
// 触发熔断/恢复路径由 test/unit/transport/guard_integration_test.go 覆盖（本用例不制造上游故障）。
package upstream_resilience

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// e2eHTTPTimeout 单条请求总超时（与 openai_chat_completion 用例保持一致）。
const e2eHTTPTimeout = 90 * time.Second

// mustE2EEnv 返回 (baseURL, apiKey) 或 t.Skip；E2E 默认离线 skip，显式提供环境变量时才打到生产。
func mustE2EEnv(t *testing.T) (baseURL, apiKey string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

// TestChatCompletionSucceedsWithDefaultGuard 回归：默认配置（熔断开启、bulkhead 开启）下正常请求成功。
func TestChatCompletionSucceedsWithDefaultGuard(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustE2EEnv(t)

	body, err := os.ReadFile("./fixtures/requests/simple_chat.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(respBody), `"choices"`) {
		t.Fatalf("response missing choices: %s", respBody)
	}
}

// TestMetricsExposeCircuitState 验证容错指标已注册（GET /metrics 公开端点）。
func TestMetricsExposeCircuitState(t *testing.T) {
	t.Parallel()
	baseURL, _ := mustE2EEnv(t)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	for _, want := range []string{"upstream_circuit_state", "upstream_circuit_open_total", "upstream_bulkhead_rejected_total"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("metrics missing %q", want)
		}
	}
}
```

- [ ] **Step 3: 本地验证编译**

Run: `go vet ./test/e2e/upstream_resilience/`
Expected: 无输出（离线时 `mustE2EEnv` 使用例 skip，不真正请求）

- [ ] **Step 4: 提交**

```bash
git add test/e2e/upstream_resilience/
git commit -m "test(e2e): 上游容错回归用例（默认配置不误伤 + 熔断指标存在性）"
```

---

## Self-Review 记录

- **Spec 覆盖**：熔断（Task 1）、隔离/信号量（Task 2）、组合与指标回调（Task 3）、配置 9 项（Task 4）、指标与适配层（Task 5）、proxy 接入与 fx（Task 6）、降级 503 + Retry-After（Task 7）、E2E（Task 8）——spec 全部条目有对应任务。
- **已知偏差（有意的）**：
  1. `EndpointKey`/`IsCircuitError` 以导出函数实现（spec 写作包级函数），便于 `test/unit/transport` 外部测试包访问，行为不变。
  2. Task 7 测试采用 usecase 包内白盒（`internal/.../usecase/`），因为 `upstreamProxyError` 未导出且无导出入口；`make test` 已覆盖 `./internal/...`。
  3. 指标名以 Namespace=`upstream` + 短名组成，最终指标名与 spec 一致（`upstream_circuit_state` 等）。
- **类型一致性**：`BreakerState`/`BreakerConfig`/`GuardConfig`/`Metrics`/`SemaphoreConfig` 在 Task 1-3 定义并被 Task 5 使用；`EndpointGuard.Allow/Report` 签名 Task 5 定义、Task 6 使用；config 全局变量名 Task 4 定义、Task 5 读取——均已逐字对齐。
