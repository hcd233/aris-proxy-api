package transport_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestCodexOAuthUpstreamTransform 转发到 Codex OAuth 端点时：
// 补 chatgpt-account-id/originator/UA/accept；body 强制 store=false、stream=true、
// 清除不接受字段、reasoning 存在时补 include。
func TestCodexOAuthUpstreamTransform(t *testing.T) {
	t.Parallel()

	var gotHeader http.Header
	var gotBody map[string]any // 测试内局部容器
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", constant.HTTPContentTypeEventStream)
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("gpt-5-codex", "codex-token-1", mock.URL, enum.EndpointAuthOAuthOpenAI, "acc-1", "oauth:9")

	// 模拟客户端透传 aris 侧密钥（Review Focus 3：OAuth 转发必须删除，不得到达上游）
	ctx := context.WithValue(t.Context(), constant.CtxKeyPassthroughHeaders, map[string]string{
		constant.HTTPHeaderAPIKey: "sk-aris-secret",
	})
	body := []byte(`{"model":"gpt-5-codex","store":true,"stream":false,"temperature":0.9,"top_p":0.5,"max_output_tokens":1234,"metadata":{"k":"v"},"user":"u","truncation":"auto","prompt_cache_retention":"24h","safety_identifier":"s","stream_options":{"include_usage":true},"reasoning":{"effort":"medium"},"input":"hello"}`)
	stream, err := proxy.OpenCreateResponseStream(ctx, ep, body)
	if err != nil {
		t.Fatalf("open response stream: %v", err)
	}
	_, _ = io.Copy(io.Discard, stream) //nolint:errcheck // 读尽以便服务端断言完成
	_ = stream.Close()                 //nolint:errcheck // 测试辅助关闭

	// headers：Codex 身份与账户头；x-api-key 必须被删除
	if got := gotHeader.Get(constant.HTTPHeaderAPIKey); got != "" {
		t.Fatalf("x-api-key must be dropped for codex oauth upstream, got %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderChatGPTAccountID); got != "acc-1" {
		t.Fatalf("chatgpt-account-id = %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderOriginator); got != constant.OAuthCodexOriginator {
		t.Fatalf("originator = %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderUserAgent); got != constant.OAuthCodexUserAgent {
		t.Fatalf("user-agent = %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderAccept); got != constant.HTTPContentTypeEventStream {
		t.Fatalf("accept = %q", got)
	}
	if got := gotHeader.Get(constant.HTTPHeaderAuthorization); got != constant.HTTPAuthBearerPrefix+"codex-token-1" {
		t.Fatalf("authorization = %q", got)
	}

	// body：强制字段与清理
	if store, _ := gotBody["store"].(bool); store {
		t.Fatalf("store must be forced false, got %#v", gotBody["store"])
	}
	if stream_, _ := gotBody["stream"].(bool); !stream_ {
		t.Fatalf("stream must be forced true, got %#v", gotBody["stream"])
	}
	for _, field := range []string{"temperature", "top_p", "max_output_tokens", "metadata", "user", "truncation", "prompt_cache_retention", "safety_identifier", "stream_options"} {
		if _, ok := gotBody[field]; ok {
			t.Fatalf("field %q must be stripped for codex oauth", field)
		}
	}
	includes, _ := gotBody["include"].([]any)
	found := false
	for _, item := range includes {
		if item == constant.OAuthCodexReasoningEncryptedInclude {
			found = true
		}
	}
	if !found {
		t.Fatalf("include must contain %q, got %#v", constant.OAuthCodexReasoningEncryptedInclude, gotBody["include"])
	}
}

// TestCodexOAuthWithoutReasoningKeepsIncludeAbsent 无 reasoning 时不注入 include。
func TestCodexOAuthWithoutReasoningKeepsIncludeAbsent(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", constant.HTTPContentTypeEventStream)
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	}))
	defer mock.Close()

	guard := transport.NewEndpointGuard(nil)
	proxy := transport.NewOpenAIProxy(inflight.NewTracker(), guard)
	ep := vo.NewUpstreamEndpointFromCredential("gpt-5-codex", "codex-token-2", mock.URL, enum.EndpointAuthOAuthOpenAI, "acc-1", "oauth:10")

	stream, err := proxy.OpenCreateResponseStream(t.Context(), ep, []byte(`{"model":"gpt-5-codex","input":"hi"}`))
	if err != nil {
		t.Fatalf("open response stream: %v", err)
	}
	_, _ = io.Copy(io.Discard, stream) //nolint:errcheck // 读尽
	_ = stream.Close()                 //nolint:errcheck // 测试辅助关闭

	if _, ok := gotBody["include"]; ok {
		t.Fatalf("include must stay absent without reasoning, got %#v", gotBody["include"])
	}
}
