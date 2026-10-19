package endpoint_auth

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestEndpointKeyStableAcrossTokenRotation OAuth 端点熔断/隔离键不随 access token 轮换漂移（Review Focus 2）。
func TestEndpointKeyStableAcrossTokenRotation(t *testing.T) {
	t.Parallel()

	const base = "https://api.anthropic.com/v1"
	first := vo.NewUpstreamEndpointFromCredential("claude", "token-1", base, enum.EndpointAuthOAuthAnthropic, "org-1", "oauth:7")
	second := vo.NewUpstreamEndpointFromCredential("claude", "token-2", base, enum.EndpointAuthOAuthAnthropic, "org-1", "oauth:7")

	if transport.EndpointKey(first) != transport.EndpointKey(second) {
		t.Fatalf("endpoint key must be stable across token rotation: %q vs %q",
			transport.EndpointKey(first), transport.EndpointKey(second))
	}
}

// TestEndpointKeyAPIKeyBehaviorUnchanged api_key 端点维持原哈希逻辑（不同 key 不同键、同输入确定性）。
func TestEndpointKeyAPIKeyBehaviorUnchanged(t *testing.T) {
	t.Parallel()

	const base = "https://api.openai.com/v1"
	a := vo.NewUpstreamEndpointFromCredential("m", "sk-a", base, enum.EndpointAuthAPIKey, "", "")
	b := vo.NewUpstreamEndpointFromCredential("m", "sk-b", base, enum.EndpointAuthAPIKey, "", "")

	if transport.EndpointKey(a) == transport.EndpointKey(b) {
		t.Fatal("different api keys must yield different endpoint keys")
	}
	a2 := vo.NewUpstreamEndpointFromCredential("m", "sk-a", base, enum.EndpointAuthAPIKey, "", "")
	if transport.EndpointKey(a) != transport.EndpointKey(a2) {
		t.Fatal("endpoint key must be deterministic for the same input")
	}
}
