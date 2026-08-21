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
