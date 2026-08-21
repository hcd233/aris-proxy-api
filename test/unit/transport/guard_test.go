// Package transport_test 验证 proxy 适配层：endpoint key 生成、熔断错误分类、EndpointGuard 组装。
package transport_test

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

func TestEndpointKey(t *testing.T) {
	t.Parallel()
	ep := vo.UpstreamEndpoint{Model: "m", APIKey: "secret-key", BaseURL: "https://api.example.com"}
	got := transport.EndpointKey(ep)
	// APIKey 不得明文出现在 key 中（key 会进入 Prometheus label 与错误消息）
	if strings.Contains(got, "secret-key") {
		t.Fatalf("EndpointKey = %q, must not contain raw APIKey", got)
	}
	// 同 baseURL 同 key 稳定；不同 key 可区分
	if got != transport.EndpointKey(ep) {
		t.Fatal("EndpointKey not stable for same endpoint")
	}
	ep2 := vo.UpstreamEndpoint{Model: "m", APIKey: "other-key", BaseURL: "https://api.example.com"}
	if got == transport.EndpointKey(ep2) {
		t.Fatal("EndpointKey must differ for different APIKeys")
	}
}

func TestIsCircuitError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"5xx 计入", &model.UpstreamError{StatusCode: 503}, true},
		{"502 计入", &model.UpstreamError{StatusCode: 502}, true},
		{"网络错误计入", &model.UpstreamConnectionError{Cause: ierr.New(ierr.ErrProxyRequest, "timeout")}, true},
		{"429 不计入", &model.UpstreamError{StatusCode: 429}, false},
		{"404 不计入", &model.UpstreamError{StatusCode: 404}, false},
		{"ierr 不计入", ierr.New(ierr.ErrProxyRequest, "build request"), false},
		{"nil 不计入", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := transport.IsCircuitError(tt.err); got != tt.want {
				t.Errorf("IsCircuitError(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNewEndpointGuardNilRegistry(t *testing.T) {
	t.Parallel()
	// nil registry 应跳过指标注册且不 panic
	g := transport.NewEndpointGuard(nil)
	if g == nil {
		t.Fatal("NewEndpointGuard(nil) = nil")
	}
}
