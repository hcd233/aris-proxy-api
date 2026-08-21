// Package llmproxy_usecase 验证降级映射：熔断打开/信号量满载错误转 503 + Retry-After 协议错误体。
package llmproxy_usecase

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

func TestProxyErrorFromUpstream_CircuitOpen(t *testing.T) {
	t.Parallel()
	err := &model.CircuitOpenError{Key: "k", RetryAfter: 3 * time.Second}
	pe := usecase.ProxyErrorFromUpstream(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "3" {
		t.Fatalf("Retry-After = %q, want 3", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	// type 为官方枚举 server_error；内部实现语义仅保留在 code 字段
	if !strings.Contains(string(pe.Body), `"code":"circuit_open"`) {
		t.Fatalf("Body = %s, want circuit_open error code", pe.Body)
	}
	if pe.Cause != err {
		t.Fatalf("Cause not preserved")
	}
}

func TestProxyErrorFromUpstream_CircuitOpenAnthropic(t *testing.T) {
	t.Parallel()
	err := &model.CircuitOpenError{Key: "k", RetryAfter: 1500 * time.Millisecond}
	pe := usecase.ProxyErrorFromUpstream(err, enum.ProtocolKindAnthropic, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "2" {
		t.Fatalf("Retry-After = %q, want 2 (ceil 1.5s)", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	if !strings.Contains(string(pe.Body), `"overloaded_error"`) {
		t.Fatalf("Body = %s, want overloaded_error error type", pe.Body)
	}
}

func TestProxyErrorFromUpstream_BulkheadFull(t *testing.T) {
	t.Parallel()
	err := &model.BulkheadFullError{Key: "k", Limit: 32}
	pe := usecase.ProxyErrorFromUpstream(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", pe.StatusCode)
	}
	if pe.Headers[constant.HTTPHeaderRetryAfter] != "5" {
		t.Fatalf("Retry-After = %q, want 5", pe.Headers[constant.HTTPHeaderRetryAfter])
	}
	if !strings.Contains(string(pe.Body), `"code":"bulkhead_full"`) {
		t.Fatalf("Body = %s, want bulkhead_full error code", pe.Body)
	}
}

func TestProxyErrorFromUpstream_UpstreamErrorStillMapped(t *testing.T) {
	t.Parallel()
	err := &model.UpstreamError{StatusCode: 502, Body: "bad gateway"}
	pe := usecase.ProxyErrorFromUpstream(err, enum.ProtocolKindOpenAI, []byte(`{}`))
	if pe.StatusCode != 502 {
		t.Fatalf("StatusCode = %d, want 502", pe.StatusCode)
	}
}

func TestProxyErrorFromUpstream_UnknownErrorFallback(t *testing.T) {
	t.Parallel()
	err := model.Error{Code: 1, Message: "boom"}
	pe := usecase.ProxyErrorFromUpstream(&err, enum.ProtocolKindOpenAI, []byte(`{"fallback":true}`))
	if pe.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode = %d, want 502", pe.StatusCode)
	}
	if string(pe.Body) != `{"fallback":true}` {
		t.Fatalf("Body = %s, want fallback body", pe.Body)
	}
}
