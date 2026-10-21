// Package transport_test 验证订阅账号端点的上游特化：Claude 的 headers 清理/覆盖与 body 注入。
package transport_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// 全局 HTTP 客户端由应用启动时初始化；测试进程需显式初始化。
func init() {
	httpclient.InitHTTPClient()
}

// TestClaudeOAuthUpstreamTransform 转发到 OAuth 端点时：
// 删除透传 x-api-key、覆盖 Authorization、合并 anthropic-beta、伪装 UA；
// body 前置身份块并重写 metadata.user_id（Review Focus 3：aris key 不得到达上游）。
func TestClaudeOAuthUpstreamTransform(t *testing.T) {
	t.Parallel()

	var gotHeader http.Header
	var gotBody map[string]any // 测试内局部容器，用于半结构化断言
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","type":"message","role":"assistant","model":"claude","content":[]}`))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewAnthropicProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("claude-sonnet-4-5", "oauth-token-1", mock.URL, enum.EndpointAuthOAuthAnthropic, "org-1", "oauth:7")

	// 模拟客户端透传：aris 侧密钥与客户端自身 beta 都应被正确处理
	ctx := context.WithValue(t.Context(), constant.CtxKeyPassthroughHeaders, map[string]string{
		constant.HTTPHeaderAPIKey:        "sk-aris-secret",
		constant.HTTPHeaderAnthropicBeta: "context-1m-2025-08-07",
	})
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":64,"metadata":{"user_id":"legacy-id"},"messages":[{"role":"user","content":"hello world"}]}`)
	if _, err := proxy.ForwardCreateMessage(ctx, ep, body); err != nil {
		t.Fatalf("forward: %v", err)
	}

	// headers 断言
	if got := gotHeader.Get(constant.HTTPHeaderAPIKey); got != "" {
		t.Fatalf("x-api-key must be dropped for oauth upstream, got %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderAuthorization); got != constant.HTTPAuthBearerPrefix+"oauth-token-1" {
		t.Fatalf("authorization = %q", got)
	}
	beta := gotHeader.Get(constant.HTTPHeaderAnthropicBeta)
	if !strings.Contains(beta, constant.OAuthClaudeBetaHeader) || !strings.Contains(beta, "context-1m-2025-08-07") {
		t.Fatalf("anthropic-beta must be merged, got %q", beta)
	}
	if got := gotHeader.Get(constant.HTTPHeaderUserAgent); got != constant.OAuthClaudeUserAgent {
		t.Fatalf("user-agent = %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderXApp); got != constant.OAuthClaudeXApp {
		t.Fatalf("x-app = %q", got)
	}

	// body 断言：system 前置身份块；metadata.user_id 重写
	system, ok := gotBody["system"].([]any)
	if !ok || len(system) == 0 {
		t.Fatalf("system must become a block array, got %#v", gotBody["system"])
	}
	first, ok := system[0].(map[string]any)
	if !ok || first["text"] != constant.OAuthClaudeSystemBlock {
		t.Fatalf("first system block must be the identity statement, got %#v", system[0])
	}
	metadata, ok := gotBody["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata missing: %#v", gotBody["metadata"])
	}
	userID, _ := metadata["user_id"].(string)
	if !strings.Contains(userID, `"account_uuid":"org-1"`) || userID == "legacy-id" {
		t.Fatalf("metadata.user_id must be rewritten, got %q", userID)
	}
}

// TestClaudeOAuthUpstreamKeepsExistingSystemArray 既有数组形态 system 保留原文并前置身份块。
func TestClaudeOAuthUpstreamKeepsExistingSystemArray(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","type":"message","role":"assistant","model":"claude","content":[]}`))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewAnthropicProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("claude-sonnet-4-5", "oauth-token-2", mock.URL, enum.EndpointAuthOAuthAnthropic, "org-1", "oauth:8")

	body := []byte(`{"model":"m","max_tokens":8,"system":[{"type":"text","text":"original system"}],"messages":[{"role":"user","content":"hi"}]}`)
	if _, err := proxy.ForwardCreateMessage(t.Context(), ep, body); err != nil {
		t.Fatalf("forward: %v", err)
	}

	system, ok := gotBody["system"].([]any)
	if !ok || len(system) != 2 {
		t.Fatalf("system must keep original block after identity, got %#v", gotBody["system"])
	}
	second, _ := system[1].(map[string]any)
	if second["text"] != "original system" {
		t.Fatalf("original system text lost: %#v", system)
	}
}

// TestClaudeOAuthUpstreamOmitsDuplicateIdentityBlock 客户端本身是 Claude Code 时不得重复插入身份块。
func TestClaudeOAuthUpstreamOmitsDuplicateIdentityBlock(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","type":"message","role":"assistant","model":"claude","content":[]}`))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewAnthropicProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("claude-sonnet-4-5", "oauth-token-3", mock.URL, enum.EndpointAuthOAuthAnthropic, "org-1", "oauth:9")

	body := []byte(`{"model":"m","max_tokens":8,"system":[{"type":"text","text":"` + constant.OAuthClaudeSystemBlock +
		`"},{"type":"text","text":"original system"}],"messages":[{"role":"user","content":"hi"}]}`)
	if _, err := proxy.ForwardCreateMessage(t.Context(), ep, body); err != nil {
		t.Fatalf("forward: %v", err)
	}

	system, ok := gotBody["system"].([]any)
	if !ok || len(system) != 2 {
		t.Fatalf("identity block must not be prepended twice, got %#v", gotBody["system"])
	}
	second, _ := system[1].(map[string]any)
	if second["text"] != "original system" {
		t.Fatalf("client system blocks must be preserved as-is, got %#v", system)
	}
}

// TestAPIKeyEndpointHeadersUnchanged api_key 端点维持原 headers 行为（回归：仍设 x-api-key）。
func TestAPIKeyEndpointHeadersUnchanged(t *testing.T) {
	t.Parallel()

	var gotHeader http.Header
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","type":"message","role":"assistant","model":"claude","content":[]}`))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewAnthropicProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("m", "sk-upstream", mock.URL, enum.EndpointAuthAPIKey, "", "")

	if _, err := proxy.ForwardCreateMessage(t.Context(), ep, []byte(`{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`)); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if got := gotHeader.Get(constant.HTTPHeaderAPIKey); got != "sk-upstream" {
		t.Fatalf("api_key endpoint must keep x-api-key, got %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderUserAgent); got == constant.OAuthClaudeUserAgent {
		t.Fatal("api_key endpoint must not masquerade as claude-cli")
	}
}
