package endpoint_resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// oauthFilterEndpointRepo 按 ID 返回预置端点集合（嵌入 stubEndpointRepo 复用其余接口方法）。
type oauthFilterEndpointRepo struct {
	*stubEndpointRepo
	endpoints map[uint]*aggregate.Endpoint
}

func (s *oauthFilterEndpointRepo) FindByID(_ context.Context, id uint, _ *uint) (*aggregate.Endpoint, error) {
	return s.endpoints[id], nil
}

// newOAuthEndpointFixture 构造带凭据状态投影的 oauth_anthropic 端点。
func newOAuthEndpointFixture(t *testing.T, id uint, name string, status enum.OAuthCredentialStatus) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(id, name, "", "https://api.anthropic.com/v1", "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	if err != nil {
		t.Fatalf("create oauth endpoint: %v", err)
	}
	ep.SetOAuthStatus(status)
	return ep
}

// newAPIKeyEndpointFixture 构造 api_key 端点（回归基线）。
func newAPIKeyEndpointFixture(t *testing.T, id uint, name string) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(id, name, "https://api.openai.com/v1", "", "sk-test", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatalf("create api_key endpoint: %v", err)
	}
	return ep
}

// modelForEndpoint 构造绑定到指定端点的启用模型。
func modelForEndpoint(t *testing.T, id, endpointID uint, alias string) *aggregate.Model {
	t.Helper()
	m, err := aggregate.CreateModel(id, vo.EndpointAlias(alias), "upstream-model", endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	return m
}

// newOAuthFilterResolver 组装 resolver（用户 ID 7，静态模型集合）。
func newOAuthFilterResolver(endpoints map[uint]*aggregate.Endpoint, models []*aggregate.Model) service.EndpointResolver {
	repo := &oauthFilterEndpointRepo{stubEndpointRepo: &stubEndpointRepo{}, endpoints: endpoints}
	return service.NewEndpointResolver(repo, &staticModelRepo{models: models}, false, nil)
}

// TestResolveCandidatesSkipsReauthRequiredOAuthEndpoint reauth_required 的 OAuth 端点必须从候选中剔除（Review Focus 1）。
func TestResolveCandidatesSkipsReauthRequiredOAuthEndpoint(t *testing.T) {
	t.Parallel()

	endpoints := map[uint]*aggregate.Endpoint{
		1: newOAuthEndpointFixture(t, 1, "claude-sub", enum.OAuthCredentialStatusReauthRequired),
		2: newAPIKeyEndpointFixture(t, 2, "apikey"),
	}
	models := []*aggregate.Model{
		modelForEndpoint(t, 11, 1, "claude-sonnet"),
		modelForEndpoint(t, 12, 2, "claude-sonnet"),
	}
	resolver := newOAuthFilterResolver(endpoints, models)

	cands, err := resolver.ResolveCandidates(t.Context(), 7, vo.EndpointAlias("claude-sonnet"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cands) != 1 || cands[0].Endpoint.AggregateID() != 2 {
		t.Fatalf("reauth endpoint must be skipped, got %d candidate(s)", len(cands))
	}
}

// TestResolveCandidatesAllOAuthReauthReturnsNotFound 仅剩 reauth 端点时返回 ErrDataNotExists（fallback 无路可走）。
func TestResolveCandidatesAllOAuthReauthReturnsNotFound(t *testing.T) {
	t.Parallel()

	endpoints := map[uint]*aggregate.Endpoint{
		1: newOAuthEndpointFixture(t, 1, "claude-sub", enum.OAuthCredentialStatusReauthRequired),
	}
	models := []*aggregate.Model{modelForEndpoint(t, 11, 1, "claude-sonnet")}
	resolver := newOAuthFilterResolver(endpoints, models)

	_, err := resolver.ResolveCandidates(t.Context(), 7, vo.EndpointAlias("claude-sonnet"), nil)
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("want ErrDataNotExists, got %v", err)
	}
}

// TestResolveCandidatesKeepsActiveOAuthEndpoint active 的 OAuth 端点正常参与调度。
func TestResolveCandidatesKeepsActiveOAuthEndpoint(t *testing.T) {
	t.Parallel()

	endpoints := map[uint]*aggregate.Endpoint{
		1: newOAuthEndpointFixture(t, 1, "claude-sub", enum.OAuthCredentialStatusActive),
		2: newAPIKeyEndpointFixture(t, 2, "apikey"),
	}
	models := []*aggregate.Model{
		modelForEndpoint(t, 11, 1, "claude-sonnet"),
		modelForEndpoint(t, 12, 2, "claude-sonnet"),
	}
	resolver := newOAuthFilterResolver(endpoints, models)

	cands, err := resolver.ResolveCandidates(t.Context(), 7, vo.EndpointAlias("claude-sonnet"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cands) != 2 {
		t.Fatalf("active oauth endpoint must stay schedulable, got %d candidate(s)", len(cands))
	}
}

// TestResolveCandidatesKeepsAPIKeyWithoutOAuthStatus api_key 端点不受凭据状态字段影响（回归）。
func TestResolveCandidatesKeepsAPIKeyWithoutOAuthStatus(t *testing.T) {
	t.Parallel()

	endpoints := map[uint]*aggregate.Endpoint{
		2: newAPIKeyEndpointFixture(t, 2, "apikey"),
	}
	models := []*aggregate.Model{modelForEndpoint(t, 12, 2, "gpt-4o")}
	resolver := newOAuthFilterResolver(endpoints, models)

	cands, err := resolver.ResolveCandidates(t.Context(), 7, vo.EndpointAlias("gpt-4o"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("api_key endpoint must resolve, got %d candidate(s)", len(cands))
	}
}
