package headerpassthrough

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	humago "github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func TestGetPassthroughHeaders_NoHeaders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil, got %v", headers)
	}
}

func TestGetPassthroughHeaders_WithHeaders(t *testing.T) {
	t.Parallel()
	expected := map[string]string{
		"User-Agent": "test-agent",
		"X-Custom":   "custom-value",
	}
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, expected)
	headers := util.GetPassthroughHeaders(ctx)
	if headers == nil {
		t.Fatal("expected non-nil headers")
	}
	if headers["User-Agent"] != "test-agent" {
		t.Errorf("expected test-agent, got %s", headers["User-Agent"])
	}
	if headers["X-Custom"] != "custom-value" {
		t.Errorf("expected custom-value, got %s", headers["X-Custom"])
	}
}

func TestGetPassthroughHeaders_WrongType(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, "not-a-map")
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil for wrong type, got %v", headers)
	}
}

func TestGetPassthroughHeaders_EmptyMap(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, map[string]string{})
	headers := util.GetPassthroughHeaders(ctx)
	if headers == nil {
		t.Fatal("expected non-nil headers")
	}
	if len(headers) != 0 {
		t.Errorf("expected empty map, got %d entries", len(headers))
	}
}

func TestMaskHTTPHeadersForLog_MasksSensitiveHeaders(t *testing.T) {
	t.Parallel()
	headers := http.Header{
		constant.HTTPHeaderAuthorization:      {"Bearer raw-secret-token"},
		constant.HTTPHeaderCookie:             {"session=raw-secret-cookie"},
		constant.HTTPHeaderProxyAuthorization: {"Basic raw-proxy-secret"},
		constant.HTTPHeaderAPIKey:             {"raw-api-key"},
		"X-Custom-Header":                     {"keep-me"},
	}

	got := util.MaskHTTPHeadersForLog(headers)

	for _, key := range []string{
		constant.HTTPHeaderAuthorization,
		constant.HTTPHeaderCookie,
		constant.HTTPHeaderProxyAuthorization,
		constant.HTTPHeaderAPIKey,
	} {
		if got[key] != constant.MaskSecretPlaceholder {
			t.Errorf("%s = %v, want %s", key, got[key], constant.MaskSecretPlaceholder)
		}
	}
	if got["X-Custom-Header"] != "keep-me" {
		t.Errorf("X-Custom-Header = %v, want keep-me", got["X-Custom-Header"])
	}
}

func TestHeaderPassthroughMiddleware_ExcludesAcceptEncoding(t *testing.T) {
	t.Parallel()
	var got map[string]string
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(middleware.HeaderPassthroughMiddleware())
	huma.Register(api, huma.Operation{
		OperationID: "captureHeaders",
		Method:      http.MethodPost,
		Path:        "/capture",
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body string
	}, error) {
		got = util.GetPassthroughHeaders(ctx)
		return &struct{ Body string }{Body: "ok"}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/capture", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Custom-Header", "keep-me")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got == nil {
		t.Fatal("expected passthrough headers")
	}
	if got["X-Custom-Header"] != "keep-me" {
		t.Errorf("X-Custom-Header = %q, want keep-me", got["X-Custom-Header"])
	}
	if _, ok := got["Accept-Encoding"]; ok {
		t.Fatalf("Accept-Encoding should not be passthrough headers: %v", got)
	}
}

func TestHeaderPassthroughMiddleware_ExcludesReverseProxyHeaders(t *testing.T) {
	t.Parallel()
	var got map[string]string
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(middleware.HeaderPassthroughMiddleware())
	huma.Register(api, huma.Operation{
		OperationID: "captureHeaders",
		Method:      http.MethodPost,
		Path:        "/capture",
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body string
	}, error) {
		got = util.GetPassthroughHeaders(ctx)
		return &struct{ Body string }{Body: "ok"}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/capture", http.NoBody)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "1.2.3.4")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Port", "443")
	req.Header.Set("Remote-Host", "1.2.3.4")
	req.Header.Set("X-Custom-Header", "keep-me")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got == nil {
		t.Fatal("expected passthrough headers")
	}
	if got["X-Custom-Header"] != "keep-me" {
		t.Errorf("X-Custom-Header = %q, want keep-me", got["X-Custom-Header"])
	}
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "X-Forwarded-Proto", "X-Forwarded-Port", "REMOTE-HOST"} {
		if _, ok := got[h]; ok {
			t.Fatalf("%s should not be passthrough headers: %v", h, got)
		}
	}
}

func TestWrapStreamResponse_AppliesPassthroughResponseHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyPassthroughResponseHeaders, map[string]string{
			"X-Upstream-Cache": "hit",
		})
		next(ctx)
	})
	huma.Register(api, huma.Operation{
		OperationID: "streamHeaders",
		Method:      http.MethodGet,
		Path:        "/stream",
	}, func(_ context.Context, _ *struct{}) (*huma.StreamResponse, error) {
		return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
			_, _ = fmt.Fprintf(w, constant.SSEDataFrameTemplate, []byte(`{"ok":true}`))
			_ = w.Flush()
		}), nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stream", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.Header.Get("X-Upstream-Cache") != "hit" {
		t.Fatalf("X-Upstream-Cache = %q, want hit", resp.Header.Get("X-Upstream-Cache"))
	}
}

func TestOpenAIProxy_CanonicalizesPassthroughHeader(t *testing.T) {
	t.Parallel()
	const headerName = "X-CUSTOM-header"
	const headerValue = "keep-me"

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
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, map[string]string{
		headerName: headerValue,
	})
	proxy := transport.NewOpenAIProxy()
	_, err = proxy.ForwardChatCompletion(ctx, vo.UpstreamEndpoint{
		Model:   "test-model",
		APIKey:  "test-key",
		BaseURL: "http://" + listener.Addr().String(),
	}, []byte(`{}`))
	if err != nil {
		t.Fatalf("forward chat completion: %v", err)
	}

	request := receiveRawRequest(t, rawRequest)
	if strings.Contains(request, "\r\n"+headerName+": "+headerValue+"\r\n") {
		t.Fatalf("expected header %q to be canonicalized, but was preserved in raw request:\n%s", headerName, request)
	}
	if !strings.Contains(request, "\r\nX-Custom-Header: "+headerValue+"\r\n") {
		t.Fatalf("expected canonicalized header \"X-Custom-Header\" in raw upstream request, got:\n%s", request)
	}

	if err := receiveServerDone(t, serverDone); err != nil {
		t.Fatalf("raw upstream server: %v", err)
	}
}

func serveRawOpenAIResponse(listener net.Listener, rawRequest chan<- string, serverDone chan<- error) {
	conn, err := listener.Accept()
	if err != nil {
		serverDone <- err
		return
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		serverDone <- err
		return
	}

	reader := bufio.NewReader(conn)
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			serverDone <- err
			return
		}
		request.WriteString(line)
		if line == "\r\n" {
			break
		}
	}
	rawRequest <- request.String()

	_, err = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}"))
	serverDone <- err
}

func receiveRawRequest(t *testing.T, rawRequest <-chan string) string {
	t.Helper()
	select {
	case request := <-rawRequest:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for raw upstream request")
	}
	return ""
}

func receiveServerDone(t *testing.T, serverDone <-chan error) error {
	t.Helper()
	select {
	case err := <-serverDone:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for raw upstream server")
	}
	return nil
}
