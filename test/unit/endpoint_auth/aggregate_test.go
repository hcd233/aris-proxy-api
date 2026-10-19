// Package endpoint_auth 验证 Endpoint 聚合的 auth_type 不变量：
// OAuth 类型端点豁免 API Key 非空、必须开启对应协议能力，且连接字段在 Update 路径不可变。
package endpoint_auth

import (
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// newAPIKeyEndpoint 构造一个合法的 api_key 类型端点（回归基线）。
func newAPIKeyEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(1, "ep", "https://o.example.com", "", "sk-x", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatalf("create api_key endpoint: %v", err)
	}
	return ep
}

// newClaudeOAuthEndpoint 构造一个合法的 oauth_anthropic 端点（凭据在 credentials 表，apiKey 为空）。
func newClaudeOAuthEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(2, "claude-sub", "", "https://api.anthropic.com/v1", "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	if err != nil {
		t.Fatalf("create oauth endpoint: %v", err)
	}
	return ep
}

// TestEndpointEmptyAuthTypeDefaultsToAPIKey 空 authType 归一为 api_key（调用方漏传不产生脏数据）。
func TestEndpointEmptyAuthTypeDefaultsToAPIKey(t *testing.T) {
	t.Parallel()
	ep, err := aggregate.CreateEndpoint(1, "ep", "https://o.example.com", "", "sk-x", true, false, false, false, "")
	if err != nil {
		t.Fatalf("create with empty auth type: %v", err)
	}
	if got := ep.AuthType(); got != enum.EndpointAuthAPIKey {
		t.Fatalf("empty auth type should default to api_key, got %q", got)
	}
}

// TestEndpointOAuthAllowsEmptyAPIKey OAuth 类型豁免 API Key 非空。
func TestEndpointOAuthAllowsEmptyAPIKey(t *testing.T) {
	t.Parallel()
	ep := newClaudeOAuthEndpoint(t)
	if !ep.AuthType().IsOAuth() {
		t.Fatalf("auth type should be OAuth, got %q", ep.AuthType())
	}
	if ep.APIKey() != "" {
		t.Fatalf("oauth endpoint api key should stay empty, got %q", ep.APIKey())
	}
}

// TestEndpointOAuthRequiresProviderCapability OAuth 类型必须开启对应协议能力（能力不变量，防 2026-10-12 类脏行）。
func TestEndpointOAuthRequiresProviderCapability(t *testing.T) {
	t.Parallel()

	if _, err := aggregate.CreateEndpoint(1, "c", "", "https://api.anthropic.com/v1", "", false, false, false, false, enum.EndpointAuthOAuthAnthropic); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("oauth_anthropic without anthropic message capability should fail, got %v", err)
	}
	if _, err := aggregate.CreateEndpoint(1, "o", "https://chatgpt.com/backend-api/codex", "", "", false, false, false, false, enum.EndpointAuthOAuthOpenAI); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("oauth_openai without responses capability should fail, got %v", err)
	}
}

// TestEndpointOAuthUpdateRejectsConnectionFields OAuth 类型 Update 禁止改连接字段与能力标记，仅 name 可改。
func TestEndpointOAuthUpdateRejectsConnectionFields(t *testing.T) {
	t.Parallel()
	ep := newClaudeOAuthEndpoint(t)

	baseURL := "https://evil.example.com"
	if err := ep.Update(nil, nil, &baseURL, nil, nil, nil, nil, nil); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("oauth endpoint must reject anthropic baseURL update, got %v", err)
	}

	apiKey := "sk-injected"
	if err := ep.Update(nil, nil, nil, &apiKey, nil, nil, nil, nil); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("oauth endpoint must reject api key update, got %v", err)
	}

	enable := true
	if err := ep.Update(nil, nil, nil, nil, &enable, nil, nil, nil); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("oauth endpoint must reject capability update, got %v", err)
	}

	name := "renamed"
	if err := ep.Update(&name, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("oauth endpoint should allow name update, got %v", err)
	}
	if ep.Name() != name {
		t.Fatalf("name update not applied, got %q", ep.Name())
	}
}

// TestEndpointAPIKeyUpdateKeepsLegacyBehavior api_key 类型行为与改动前一致（回归）。
func TestEndpointAPIKeyUpdateKeepsLegacyBehavior(t *testing.T) {
	t.Parallel()
	ep := newAPIKeyEndpoint(t)

	baseURL := "https://api.anthropic.com/v1"
	enable := true
	if err := ep.Update(nil, nil, &baseURL, nil, nil, nil, &enable, nil); err != nil {
		t.Fatalf("api_key endpoint update must keep working, got %v", err)
	}
	if ep.AnthropicBaseURL() != baseURL || !ep.SupportAnthropicMessage() {
		t.Fatal("api_key endpoint update did not apply")
	}
}
