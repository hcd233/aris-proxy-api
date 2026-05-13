package headerpassthrough

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humago "github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func TestGetPassthroughHeaders_NoHeaders(t *testing.T) {
	ctx := context.Background()
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil, got %v", headers)
	}
}

func TestGetPassthroughHeaders_WithHeaders(t *testing.T) {
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
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, "not-a-map")
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil for wrong type, got %v", headers)
	}
}

func TestGetPassthroughHeaders_EmptyMap(t *testing.T) {
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
	headers := http.Header{
		constant.HTTPTitleHeaderAuthorization:      {"Bearer raw-secret-token"},
		constant.HTTPTitleHeaderCookie:             {"session=raw-secret-cookie"},
		constant.HTTPLowerHeaderProxyAuthorization: {"Basic raw-proxy-secret"},
		constant.HTTPTitleHeaderAPIKey:             {"raw-api-key"},
		"X-Custom-Header":                          {"keep-me"},
	}

	got := util.MaskHTTPHeadersForLog(headers)

	for _, key := range []string{
		constant.HTTPTitleHeaderAuthorization,
		constant.HTTPTitleHeaderCookie,
		constant.HTTPLowerHeaderProxyAuthorization,
		constant.HTTPTitleHeaderAPIKey,
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

	req := httptest.NewRequest(http.MethodPost, "/capture", nil)
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

func TestOpenAIProxy_PreservesPassthroughHeaderCase(t *testing.T) {
	const headerName = "X-CUSTOM-header"
	const headerValue = "keep-me"

	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	if !strings.Contains(request, "\r\n"+headerName+": "+headerValue+"\r\n") {
		t.Fatalf("expected raw upstream request to contain preserved header %q, got:\n%s", headerName, request)
	}
	if strings.Contains(request, "\r\nX-Custom-Header: "+headerValue+"\r\n") {
		t.Fatalf("passthrough header was canonicalized unexpectedly, got:\n%s", request)
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
