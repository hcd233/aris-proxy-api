// Package transport_test 验证优雅退出 drain 广播对上游请求的取消链路：
// soft deadline 到点后，CancelOnDrain 派生的 ctx 被取消，阻塞的 SSE 读循环
// 返回 context.Canceled（且 UpstreamConnectionError.Unwrap 穿透，errors.Is 可判定）。
package transport_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestMain 初始化全局 HTTP 客户端（transport 的 doUpstreamRequest 依赖该单例）。
func TestMain(m *testing.M) {
	httpclient.InitHTTPClient()
	os.Exit(m.Run())
}

// hangingUpstream 返回一个写首帧后挂起的上游 SSE 服务：
// 客户端（代理）取消请求时 handler 返回，模拟真实上游长流。
func hangingUpstream(t *testing.T, firstFrame string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, firstFrame); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenAIProxy_DrainCancelInterruptsStream(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()
	proxy := transport.NewOpenAIProxy(tracker, transport.NewEndpointGuard(nil))

	srv := hangingUpstream(t, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n")
	ep := vo.UpstreamEndpoint{BaseURL: srv.URL, Model: "test-model", APIKey: "test-key"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 模拟 InflightMiddleware 的 Track：Drain 依赖 inflight 计数
	// 走 soft 窗口并触发广播。
	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	stream, err := proxy.OpenChatCompletionStream(ctx, ep, []byte(`{}`))
	if err != nil {
		t.Fatalf("OpenChatCompletionStream err = %v, want nil", err)
	}

	readResult := make(chan error, 1)
	go func() {
		_, err := proxy.ReadChatCompletionStream(ctx, stream, func(*dto.OpenAIChatCompletionChunk) error { return nil })
		readResult <- err
	}()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(50*time.Millisecond, 2*time.Second)
	}()

	select {
	case err := <-readResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadChatCompletionStream err = %v, want errors.Is(err, context.Canceled)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream read should be interrupted by drain soft deadline broadcast")
	}

	tracker.Untrack()
	if result := <-drained; !result {
		t.Fatal("Drain should return true after stream released")
	}
}

// TestUpstreamRequestBodyCloseReleasesDrainGoroutine 回归（2026-09-02 生产 goroutine 泄漏）：
// CancelOnDrain 派生 ctx 的守护 goroutine 必须随上游 body 的 Close 退出。
// 缺陷形态：fiber/fasthttp 请求 ctx 不随请求结束而 Done，若 cancel 不绑定
// body 生命周期，每个上游请求泄漏一个守护 goroutine（生产 24h 随 LLM 流量
// 阶梯上涨 55→126）。判定：N 次「请求 + body Close」后 goroutine 数必须回落。
func TestUpstreamRequestBodyCloseReleasesDrainGoroutine(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()
	proxy := transport.NewOpenAIProxy(tracker, transport.NewEndpointGuard(nil))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	ep := vo.UpstreamEndpoint{BaseURL: srv.URL, Model: "test-model", APIKey: "test-key"}

	// 预热建立基线（连接池、guard 等懒初始化）
	for range 2 {
		stream, err := proxy.OpenChatCompletionStream(context.Background(), ep, []byte(`{}`))
		if err != nil {
			t.Fatalf("warmup open stream: %v", err)
		}
		_, _ = proxy.ReadChatCompletionStream(context.Background(), stream, func(*dto.OpenAIChatCompletionChunk) error { return nil })
	}
	runtime.GC()
	baseline := runtime.NumGoroutine()

	const rounds = 20
	for range rounds {
		stream, err := proxy.OpenChatCompletionStream(context.Background(), ep, []byte(`{}`))
		if err != nil {
			t.Fatalf("open stream: %v", err)
		}
		// Read 内部 defer stream.Close()：body 生命周期结束即 drain 派生 ctx 结束
		_, _ = proxy.ReadChatCompletionStream(context.Background(), stream, func(*dto.OpenAIChatCompletionChunk) error { return nil })
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+2 || time.Now().After(deadline) {
			break
		}
		<-time.After(100 * time.Millisecond) //nolint:revive // goroutine 回落轮询间隔
	}
	after := runtime.NumGoroutine()
	if after > baseline+2 {
		t.Fatalf("drain guard goroutines leaked: baseline=%d after=%d (%d closed stream requests)", baseline, after, rounds)
	}
}

func TestAnthropicProxy_DrainCancelInterruptsStream(t *testing.T) {
	t.Parallel()
	tracker := inflight.NewTracker()
	proxy := transport.NewAnthropicProxy(tracker, transport.NewEndpointGuard(nil))

	srv := hangingUpstream(t, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[]}}\n\n")
	ep := vo.UpstreamEndpoint{BaseURL: srv.URL, Model: "test-model", APIKey: "test-key"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 模拟 InflightMiddleware 的 Track：Drain 依赖 inflight 计数
	// 走 soft 窗口并触发广播。
	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	stream, err := proxy.OpenCreateMessageStream(ctx, ep, []byte(`{}`))
	if err != nil {
		t.Fatalf("OpenCreateMessageStream err = %v, want nil", err)
	}

	readResult := make(chan error, 1)
	go func() {
		_, err := proxy.ReadCreateMessageStream(ctx, stream, func(dto.AnthropicSSEEvent) error { return nil })
		readResult <- err
	}()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(50*time.Millisecond, 2*time.Second)
	}()

	select {
	case err := <-readResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadCreateMessageStream err = %v, want errors.Is(err, context.Canceled)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream read should be interrupted by drain soft deadline broadcast")
	}

	tracker.Untrack()
	if result := <-drained; !result {
		t.Fatal("Drain should return true after stream released")
	}
}
