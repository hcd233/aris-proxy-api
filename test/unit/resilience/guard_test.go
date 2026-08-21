// Package resilience 验证 Guard 对熔断与信号量的组合编排、指标回调与开关语义。
package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
)

func guardCfg() resilience.GuardConfig {
	return resilience.GuardConfig{
		CircuitEnabled:             true,
		CircuitWindow:              6 * time.Second,
		CircuitMinRequests:         3,
		CircuitErrorThreshold:      0.5,
		CircuitOpenTimeout:         200 * time.Millisecond,
		CircuitHalfOpenMaxRequests: 1,
		BulkheadEnabled:            true,
		BulkheadMaxConcurrent:      1,
		BulkheadAcquireTimeout:     30 * time.Millisecond,
	}
}

type recordingMetrics struct {
	mu        sync.Mutex
	states    map[string]enum.BreakerState
	openCalls int
	rejected  int
	bulkhead  int
}

func (m *recordingMetrics) SetBreakerState(key string, s enum.BreakerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states == nil {
		m.states = map[string]enum.BreakerState{}
	}
	m.states[key] = s
}

func (m *recordingMetrics) IncCircuitOpen(string) {
	m.mu.Lock()
	m.openCalls++
	m.mu.Unlock()
}

func (m *recordingMetrics) IncCircuitRejected(string) {
	m.mu.Lock()
	m.rejected++
	m.mu.Unlock()
}

func (m *recordingMetrics) IncBulkheadRejected(string) {
	m.mu.Lock()
	m.bulkhead++
	m.mu.Unlock()
}

func TestGuard_AllowsAndReports(t *testing.T) {
	t.Parallel()
	g := resilience.NewGuard(guardCfg(), &recordingMetrics{})
	release, err := g.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	release()
	g.Report("k", true)
}

func TestGuard_OpenRejectsWithCircuitOpenError(t *testing.T) {
	t.Parallel()
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
	if m.states["k"] != enum.StateOpen {
		t.Fatalf("state metric = %v, want open", m.states["k"])
	}
}

func TestGuard_BulkheadFull(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	cfg := guardCfg()
	cfg.BulkheadEnabled = false
	g := resilience.NewGuard(cfg, nil)
	release, err := g.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	release()
}
