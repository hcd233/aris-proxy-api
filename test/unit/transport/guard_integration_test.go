// Package transport_test 验证 guard 接入 proxy 链路后的端到端行为：
// 上游持续 5xx → 熔断打开 → 请求不达上游（快速失败）→ 上游恢复 → 半开探测通过 → 全量恢复。
package transport_test

import (
	"context"
	"errors"
	"io"
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

// TestGuardIntegration_StreamBreakCountsAsFailure 验证「上游流建立后中断」计入熔断失败：
// 上游返回 200 响应头后立即断开连接（模拟半死），响应头正常到达但 body 读取失败；
// 3 次（MinRequests=3）后熔断应打开，后续请求快速失败。
// 回归背景：曾实现 BindLease 时把 success 固定为 true，流中断被上报为成功，
// 熔断器对上游半死状态系统性失明。
func TestGuardIntegration_StreamBreakCountsAsFailure(t *testing.T) { //nolint:paralleltest // modifies global config
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
	config.UpstreamRetryMaxAttempts = 0
	config.UpstreamCircuitWindow = 6 * time.Second
	config.UpstreamCircuitMinRequests = 3
	config.UpstreamCircuitErrorThreshold = 0.5
	config.UpstreamCircuitOpenTimeout = time.Hour // 本用例不进入恢复阶段
	config.UpstreamCircuitHalfOpenMaxRequests = 1
	config.UpstreamBulkheadEnabled = false // 本用例只验证熔断路径

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"1\"}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), guard)
	ep := vo.UpstreamEndpoint{Model: "m", APIKey: "k", BaseURL: mock.URL}
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		stream, err := proxy.OpenChatCompletionStream(ctx, ep, []byte(`{"messages":[]}`))
		if err != nil {
			t.Fatalf("stream #%d should get response headers: %v", i, err)
		}
		if _, rerr := io.ReadAll(stream); rerr == nil {
			t.Fatalf("stream #%d read should fail (upstream broke), got nil error", i)
		}
		_ = stream.Close()
	}

	// 3 次流中断均计入失败 → 熔断打开，后续请求不达上游
	_, err := proxy.OpenChatCompletionStream(ctx, ep, []byte(`{"messages":[]}`))
	var ce *model.CircuitOpenError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CircuitOpenError after stream breaks", err)
	}
}

// TestGuardLease_BulkheadHeldUntilBodyClose 验证 bulkhead 槽位在响应 body Close 前持续占用：
// MaxConcurrent=1 时，第一个流未消费完（body 未 Close）则第二个请求满载拒绝；
// Close 后槽位释放，后续请求恢复放行。
func TestGuardLease_BulkheadHeldUntilBodyClose(t *testing.T) { //nolint:paralleltest // modifies global config
	orig := []any{
		config.UpstreamCircuitEnabled,
		config.UpstreamBulkheadEnabled,
		config.UpstreamBulkheadMaxConcurrent,
		config.UpstreamBulkheadAcquireTimeout,
	}
	t.Cleanup(func() {
		config.UpstreamCircuitEnabled = orig[0].(bool)
		config.UpstreamBulkheadEnabled = orig[1].(bool)
		config.UpstreamBulkheadMaxConcurrent = orig[2].(int)
		config.UpstreamBulkheadAcquireTimeout = orig[3].(time.Duration)
	})
	config.UpstreamCircuitEnabled = false // 只验证 bulkhead 路径
	config.UpstreamBulkheadEnabled = true
	config.UpstreamBulkheadMaxConcurrent = 1
	config.UpstreamBulkheadAcquireTimeout = 20 * time.Millisecond

	// 上游挂起不返回，模拟慢流：响应头已到但 body 持续可读
	releaseBody := make(chan struct{})
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-releaseBody // 挂住连接，直到测试放行
	}))
	defer func() { close(releaseBody); mock.Close() }()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), guard)
	ep := vo.UpstreamEndpoint{Model: "m", APIKey: "k", BaseURL: mock.URL}
	ctx := context.Background()

	stream, err := proxy.OpenChatCompletionStream(ctx, ep, []byte(`{"messages":[]}`))
	if err != nil {
		t.Fatalf("first stream should pass: %v", err)
	}

	// body 未 Close：唯一槽位被占用，第二个请求应满载拒绝
	_, err = proxy.OpenChatCompletionStream(ctx, ep, []byte(`{"messages":[]}`))
	var bf *model.BulkheadFullError
	if !errors.As(err, &bf) {
		t.Fatalf("err = %v, want BulkheadFullError while lease held", err)
	}

	// Close body → 租约结束、槽位释放 → 恢复放行
	if cerr := stream.Close(); cerr != nil {
		t.Fatalf("close stream: %v", cerr)
	}
	waitUntil(t, 2*time.Second, func() bool {
		s, serr := proxy.OpenChatCompletionStream(ctx, ep, []byte(`{"messages":[]}`))
		if serr != nil {
			return false
		}
		_ = s.Close()
		return true
	}, "slot released after body close")
}
