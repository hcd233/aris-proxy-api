// Package resilience 验证通用熔断器状态机、滑动窗口统计与半开探测。
package resilience_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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

// waitUntil 以 ticker 轮询等待条件成立（lint 的 testing.sleep 规则禁止用固定 time.Sleep 同步）。
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", msg)
		}
		<-ticker.C
	}
}

// waitOpenTimeoutElapsed 等待熔断器打开时长超过 OpenTimeout（openAt 是触发打开前的参考时刻）。
func waitOpenTimeoutElapsed(t *testing.T, openAt time.Time) {
	t.Helper()
	waitUntil(t, time.Second, func() bool { return time.Since(openAt) >= 250*time.Millisecond }, "open timeout elapsed")
}

func TestBreaker_AllowClosed(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	if !b.Allow() {
		t.Fatal("closed state should allow requests")
	}
	if b.State() != enum.StateClosed {
		t.Fatalf("state = %v, want closed", b.State())
	}
}

func TestBreaker_OpenWhenErrorRateExceedsThreshold(t *testing.T) {
	t.Parallel()
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
	if b.State() != enum.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	if b.Allow() {
		t.Fatal("open breaker must reject requests")
	}
	if b.RetryAfter() <= 0 || b.RetryAfter() > 200*time.Millisecond {
		t.Fatalf("RetryAfter = %v, want in (0, 200ms]", b.RetryAfter())
	}
}

func TestBreaker_NotOpenBelowMinRequests(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	b.Allow()
	b.Report(false)
	b.Allow()
	b.Report(false)
	if b.State() != enum.StateClosed {
		t.Fatalf("2 requests below MinRequests=3 must stay closed, got %v", b.State())
	}
}

func TestBreaker_NotOpenBelowThreshold(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	for i := 0; i < 4; i++ {
		b.Allow()
		b.Report(true)
	}
	b.Allow()
	b.Report(false) // 1/5 = 20% < 50%
	if b.State() != enum.StateClosed {
		t.Fatalf("state = %v, want closed (below threshold)", b.State())
	}
}

func TestBreaker_HalfOpenProbeSuccessCloses(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	openAt := time.Now()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	if b.State() != enum.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	waitOpenTimeoutElapsed(t, openAt) // 等待 OpenTimeout 期满
	if !b.Allow() {
		t.Fatal("after open timeout, breaker should allow half-open probe")
	}
	if b.State() != enum.StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
	b.Report(true)
	if b.State() != enum.StateClosed {
		t.Fatalf("probe success should close, got %v", b.State())
	}
	if !b.Allow() {
		t.Fatal("closed breaker must allow requests")
	}
}

func TestBreaker_HalfOpenProbeFailureReopens(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	openAt := time.Now()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	waitOpenTimeoutElapsed(t, openAt)
	b.Allow() // 半开探测
	b.Report(false)
	if b.State() != enum.StateOpen {
		t.Fatalf("probe failure should reopen, got %v", b.State())
	}
	if b.Allow() {
		t.Fatal("reopened breaker must reject requests")
	}
}

func TestBreaker_HalfOpenLimitsProbes(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	openAt := time.Now()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false)
	}
	waitOpenTimeoutElapsed(t, openAt)
	if !b.Allow() {
		t.Fatal("first half-open probe must be allowed")
	}
	if b.Allow() {
		t.Fatal("second concurrent probe must be rejected (HalfOpenMaxRequests=1)")
	}
}

func TestBreaker_WindowSlides(t *testing.T) {
	t.Parallel()
	b := resilience.NewBreaker("k", cfg(), nil)
	openAt := time.Now()
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Report(false) // 3 失败，达到阈值触发 Open
	}
	if b.State() != enum.StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	waitOpenTimeoutElapsed(t, openAt)
	b.Allow() // 半开
	b.Report(true)
	if b.State() != enum.StateClosed {
		t.Fatalf("state = %v, want closed after probe", b.State())
	}
	// 窗口清空后，单独 1 次失败不应再次打开
	b.Allow()
	b.Report(false)
	if b.State() != enum.StateClosed {
		t.Fatalf("single failure after reset must not open, got %v", b.State())
	}
}

func TestCircuitOpenError(t *testing.T) {
	t.Parallel()
	e := &model.CircuitOpenError{Key: "k", RetryAfter: 3 * time.Second}
	if e.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
	var target *model.CircuitOpenError
	if !errors.As(e, &target) {
		t.Fatal("errors.As must match CircuitOpenError")
	}
}
