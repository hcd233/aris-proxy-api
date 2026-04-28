package header_passthrough

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
