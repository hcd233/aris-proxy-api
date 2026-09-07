package headerpassthrough

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// forwardAndCaptureRawUpstreamRequest 通过真实 OpenAIProxy 转发一次请求，
// 返回上游收到的原始 HTTP 请求头文本。
func forwardAndCaptureRawUpstreamRequest(t *testing.T, passthrough map[string]string) string {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen raw upstream: %v", err)
	}
	defer func() { _ = listener.Close() }()

	rawRequest := make(chan string, 1)
	serverDone := make(chan error, 1)
	go serveRawOpenAIResponse(listener, rawRequest, serverDone)

	httpclient.InitHTTPClient()
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, passthrough)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), transport.NewEndpointGuard(nil))
	_, err = proxy.ForwardChatCompletion(ctx, vo.UpstreamEndpoint{
		Model:   "test-model",
		APIKey:  "test-key",
		BaseURL: "http://" + listener.Addr().String(),
	}, []byte(`{}`))
	if err != nil {
		t.Fatalf("forward chat completion: %v", err)
	}

	request := receiveRawRequest(t, rawRequest)
	if err := receiveServerDone(t, serverDone); err != nil {
		t.Fatalf("raw upstream server: %v", err)
	}
	return request
}

// TestOpenAIProxy_BackfillsOpencodeSessionFromSessionID 复现上游（Console Go）对缺少
// x-opencode-session 的请求返回 400 MissingSessionID 的问题：客户端未带 x-opencode-session、
// 只带 X-Session-Id（如 ZCode）时，网关应把该值补为 x-opencode-session 再发给上游。
func TestOpenAIProxy_BackfillsOpencodeSessionFromSessionID(t *testing.T) {
	t.Parallel()
	request := forwardAndCaptureRawUpstreamRequest(t, map[string]string{
		"X-Session-Id": "eab80896-7069-493f-bfc6-b7826b4fad36",
	})
	if !strings.Contains(request, "\r\nX-Opencode-Session: eab80896-7069-493f-bfc6-b7826b4fad36\r\n") {
		t.Fatalf("expected backfilled X-Opencode-Session header in raw upstream request, got:\n%s", request)
	}
}

// TestOpenAIProxy_BackfillsOpencodeSessionFromLowercaseSessionID 覆盖客户端以
// x-session-id（小写）携带会话头的场景（如 openrouter 形态客户端）。
func TestOpenAIProxy_BackfillsOpencodeSessionFromLowercaseSessionID(t *testing.T) {
	t.Parallel()
	request := forwardAndCaptureRawUpstreamRequest(t, map[string]string{
		"x-session-id": "eab80896-7069-493f-bfc6-b7826b4fad36",
	})
	if !strings.Contains(request, "\r\nX-Opencode-Session: eab80896-7069-493f-bfc6-b7826b4fad36\r\n") {
		t.Fatalf("expected backfilled X-Opencode-Session header in raw upstream request, got:\n%s", request)
	}
}

// TestOpenAIProxy_KeepsClientOpencodeSession 客户端已带 x-opencode-session 时不得被
// X-Session-Id 覆盖（x-opencode-session 优先）。
func TestOpenAIProxy_KeepsClientOpencodeSession(t *testing.T) {
	t.Parallel()
	request := forwardAndCaptureRawUpstreamRequest(t, map[string]string{
		"X-Opencode-Session": "direct-session",
		"X-Session-Id":       "fallback-session",
	})
	if !strings.Contains(request, "\r\nX-Opencode-Session: direct-session\r\n") {
		t.Fatalf("expected client X-Opencode-Session preserved in raw upstream request, got:\n%s", request)
	}
	if strings.Contains(request, "\r\nX-Opencode-Session: fallback-session\r\n") {
		t.Fatalf("client X-Opencode-Session must win over X-Session-Id, got:\n%s", request)
	}
}

// TestOpenAIProxy_NoOpencodeSessionWithoutClientSessionHeader 客户端未带任何会话头时，
// 网关不虚构 x-opencode-session（保持原行为，不注入）。
func TestOpenAIProxy_NoOpencodeSessionWithoutClientSessionHeader(t *testing.T) {
	t.Parallel()
	request := forwardAndCaptureRawUpstreamRequest(t, map[string]string{
		"X-Custom-Header": "custom-value",
	})
	if strings.Contains(request, "\r\nX-Opencode-Session:") {
		t.Fatalf("expected no X-Opencode-Session header when client sends no session header, got:\n%s", request)
	}
}
