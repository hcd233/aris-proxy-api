// Package llmproxy_resolve_error 守卫模型解析失败的 HTTP 语义：
// 只有「模型/端点确实不存在」（ErrDataNotExists）才是 404；
// DB 查询失败等基础设施错误必须是 503，不能被吞成 model_not_found。
//
// 背景（2026-10-22 压测）：PG 连接打满时 APIKey 查询与模型解析失败，
// 应用分别返回 401 / 404 model_not_found，误导客户端以为 Key 或模型配置有问题；
// 实际是服务不可用，应返回 503 让 SDK 进入重试分支。
package llmproxy_resolve_error

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestModelResolveError_NotFoundMapsTo404(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		protocol enum.ProtocolKind
		wantBody string
	}{
		{name: "openai", protocol: enum.ProtocolKindOpenAI, wantBody: `"model_not_found"`},
		{name: "anthropic", protocol: enum.ProtocolKindAnthropic, wantBody: `"not_found_error"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ierr.New(ierr.ErrDataNotExists, "model missing")
			got := proxyutil.ModelResolveError(err, tc.protocol, "mock-chat")
			if got.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", got.StatusCode)
			}
			if got.Protocol != tc.protocol {
				t.Errorf("protocol = %v, want %v", got.Protocol, tc.protocol)
			}
			if !strings.Contains(string(got.Body), tc.wantBody) {
				t.Errorf("body = %s, want it to contain %s", got.Body, tc.wantBody)
			}
		})
	}
}

func TestModelResolveError_InfraErrorMapsTo503(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		protocol enum.ProtocolKind
		wantBody string
	}{
		{name: "openai", protocol: enum.ProtocolKindOpenAI, wantBody: `"service_unavailable"`},
		{name: "anthropic", protocol: enum.ProtocolKindAnthropic, wantBody: `"overloaded_error"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ierr.New(ierr.ErrDBQuery, "FATAL: sorry, too many clients already")
			got := proxyutil.ModelResolveError(err, tc.protocol, "mock-chat")
			if got.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", got.StatusCode)
			}
			if got.Protocol != tc.protocol {
				t.Errorf("protocol = %v, want %v", got.Protocol, tc.protocol)
			}
			if !strings.Contains(string(got.Body), tc.wantBody) {
				t.Errorf("body = %s, want it to contain %s", got.Body, tc.wantBody)
			}
			if !errors.Is(got.Cause, ierr.ErrDBQuery) {
				t.Errorf("cause must keep the DB error for logging, got %v", got.Cause)
			}
		})
	}
}
