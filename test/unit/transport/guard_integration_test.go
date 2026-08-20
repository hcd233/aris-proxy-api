// Package transport_test 验证 guard 接入 proxy 链路后的端到端行为：
// 上游持续 5xx → 熔断打开 → 请求不达上游（快速失败）→ 上游恢复 → 半开探测通过 → 全量恢复。
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

// TestGuardIntegration_OpenThenRecover 用 mock 上游验证熔断全流程：
// 3 次 5xx 触发打开 → 打开期间请求不达上游 → 上游恢复 + OpenTimeout 期满 → 半开探测成功 → 正常放行。
func TestGuardIntegration_OpenThenRecover(t *testing.T) { //nolint:paralleltest // modifies global config
	// 覆盖 config 全局变量，结束后恢复（与 retry_test 同模式）
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
	config.UpstreamBulkheadEnabled = false // 本用例只验证熔断路径

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
	openAt := time.Now()
	for i := 0; i < 3; i++ {
		_, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`))
		if err == nil {
			t.Fatalf("request #%d should fail", i)
		}
	}

	// 熔断打开：请求快速失败，且不达上游
	before := calls.Load()
	_, err := proxy.ForwardChatCompletion(ctx, ep, []byte(`{"messages":[]}`))
	var ce *model.CircuitOpenError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CircuitOpenError", err)
	}
	if calls.Load() != before {
		t.Fatalf("open state must not reach upstream: calls %d -> %d", before, calls.Load())
	}

	// 上游恢复；等待 OpenTimeout 期满后，半开探测应成功并通过 → 恢复
	healthy.Store(true)
	waitUntil(t, 2*time.Second, func() bool { return time.Since(openAt) >= 350*time.Millisecond }, "open timeout elapsed")
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
