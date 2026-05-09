package headerpassthrough

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	humago "github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
