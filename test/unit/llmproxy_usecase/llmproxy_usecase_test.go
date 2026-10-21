package llmproxy_usecase

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type mockReadRepo struct {
	listAliasesResult []*llmproxy.ModelAliasProjection
	listAliasesErr    error
	findResult        *llmproxy.EndpointProjection
	findModelResult   *llmproxy.ModelAliasProjection
	findErr           error
}

func newMockReadRepo(aliases []string) *mockReadRepo {
	projections := make([]*llmproxy.ModelAliasProjection, len(aliases))
	for i, alias := range aliases {
		projections[i] = &llmproxy.ModelAliasProjection{Alias: alias}
	}
	return &mockReadRepo{
		listAliasesResult: projections,
	}
}

func (r *mockReadRepo) ListAliases(_ context.Context, _ uint) ([]*llmproxy.ModelAliasProjection, error) {
	return r.listAliasesResult, r.listAliasesErr
}

func (r *mockReadRepo) ListEnabledModelDetails(_ context.Context, _ uint) ([]*llmproxy.ModelDetailProjection, error) {
	return nil, nil
}

func (r *mockReadRepo) FindEndpointByAlias(_ context.Context, _ uint, _ string, matcher func(*llmproxy.EndpointProjection) bool) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	if r.findErr != nil || r.findResult == nil {
		return r.findResult, r.findModelResult, r.findErr
	}
	if matcher != nil && !matcher(r.findResult) {
		return nil, nil, nil
	}
	return r.findResult, r.findModelResult, nil
}

type mockAnthropicProxy struct {
	forwardCountTokensCalled bool
	forwardCountTokensResult *dto.AnthropicTokensCount
	forwardCountTokensErr    error
	lastEndpoint             vo.UpstreamEndpoint
}

func (p *mockAnthropicProxy) ForwardCountTokens(_ context.Context, ep vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	p.forwardCountTokensCalled = true
	p.lastEndpoint = ep
	return p.forwardCountTokensResult, p.forwardCountTokensErr
}

func (p *mockAnthropicProxy) OpenCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockAnthropicProxy) ReadCreateMessageStream(_ context.Context, _ io.ReadCloser, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	return nil, nil
}

func (p *mockAnthropicProxy) ForwardCreateMessage(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	return nil, nil
}

var _ usecase.AnthropicProxyPort = (*mockAnthropicProxy)(nil)

func TestListOpenAIModels_Success(t *testing.T) {
	t.Parallel()
	repo := newMockReadRepo([]string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"})
	query := usecase.NewListOpenAIModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("Handle() returned nil response")
		return
	}
	if len(rsp.Data) != 3 {
		t.Errorf("expected 3 models, got %d", len(rsp.Data))
	}
	for i, model := range rsp.Data {
		if model.Object != "model" {
			t.Errorf("Data[%d].Object = %q, want %q", i, model.Object, "model")
		}
	}
}

func TestListOpenAIModels_Empty(t *testing.T) {
	t.Parallel()
	repo := newMockReadRepo([]string{})
	query := usecase.NewListOpenAIModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if len(rsp.Data) != 0 {
		t.Errorf("expected 0 models, got %d", len(rsp.Data))
	}
}

func TestListAnthropicModels_Success(t *testing.T) {
	t.Parallel()
	repo := newMockReadRepo([]string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022"})
	query := usecase.NewListAnthropicModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("Handle() returned nil response")
		return
	}
	if len(rsp.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(rsp.Data))
	}
	if rsp.HasMore != false {
		t.Errorf("HasMore = %v, want false", rsp.HasMore)
	}
}

func TestListAnthropicModels_Empty(t *testing.T) {
	t.Parallel()
	repo := newMockReadRepo([]string{})
	query := usecase.NewListAnthropicModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if len(rsp.Data) != 0 {
		t.Errorf("expected 0 models, got %d", len(rsp.Data))
	}
	if rsp.FirstID != "" {
		t.Errorf("FirstID = %q, want empty", rsp.FirstID)
	}
	if rsp.LastID != "" {
		t.Errorf("LastID = %q, want empty", rsp.LastID)
	}
}

func TestListModels_Pagination(t *testing.T) {
	t.Parallel()
	aliases := []string{"model-a", "model-b", "model-c"}
	repo := newMockReadRepo(aliases)
	query := usecase.NewListAnthropicModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp.FirstID != "model-a" {
		t.Errorf("FirstID = %q, want %q", rsp.FirstID, "model-a")
	}
	if rsp.LastID != "model-c" {
		t.Errorf("LastID = %q, want %q", rsp.LastID, "model-c")
	}
}

func TestCountTokens_ModelNotFound(t *testing.T) {
	t.Parallel()
	repo := &mockReadRepo{
		findResult:      nil,
		findModelResult: nil,
		findErr:         nil,
	}
	proxy := &mockAnthropicProxy{}
	query := usecase.NewCountTokens(repo, proxy, nil)

	req := &dto.AnthropicCountTokensRequest{
		Body: &dto.AnthropicCountTokensReq{
			Model: "nonexistent-model",
		},
	}

	rsp, err := query.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", rsp.InputTokens)
	}
	if proxy.forwardCountTokensCalled {
		t.Error("ForwardCountTokens should not be called when model not found")
	}
}

type stubTokenProvider struct {
	access service.OAuthAccess
	err    error
	gotID  uint
}

func (p *stubTokenProvider) GetAccessToken(_ context.Context, endpointID uint) (service.OAuthAccess, error) {
	p.gotID = endpointID
	return p.access, p.err
}

func (p *stubTokenProvider) RefreshIfNeeded(_ context.Context, _ uint) error { return nil }

// TestCountTokens_OAuthEndpointUsesAccessToken 锁定 OAuth 端点必须注入 access token。
// 回归：旧实现直接拿投影里的空 APIKey 拼 UpstreamEndpoint，请求上游必然 401，token 计数静默变 0。
func TestCountTokens_OAuthEndpointUsesAccessToken(t *testing.T) {
	t.Parallel()
	tokens := &stubTokenProvider{access: service.OAuthAccess{Token: "oauth-access-token", AccountRef: "account-1"}}
	repo := &mockReadRepo{
		findResult: &llmproxy.EndpointProjection{
			ID:                      7,
			AuthType:                enum.EndpointAuthOAuthAnthropic,
			AnthropicBaseURL:        "https://api.anthropic.com",
			SupportAnthropicMessage: true,
		},
		findModelResult: &llmproxy.ModelAliasProjection{Alias: "claude-upstream"},
	}
	proxy := &mockAnthropicProxy{forwardCountTokensResult: &dto.AnthropicTokensCount{InputTokens: 42}}
	query := usecase.NewCountTokens(repo, proxy, tokens)

	rsp, err := query.Handle(context.Background(), &dto.AnthropicCountTokensRequest{
		Body: &dto.AnthropicCountTokensReq{Model: "claude"},
	})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp.InputTokens != 42 {
		t.Errorf("InputTokens = %d, want 42", rsp.InputTokens)
	}
	if tokens.gotID != 7 {
		t.Errorf("GetAccessToken endpoint id = %d, want 7", tokens.gotID)
	}
	if proxy.lastEndpoint.APIKey != "oauth-access-token" {
		t.Errorf("upstream api key = %q, want oauth access token", proxy.lastEndpoint.APIKey)
	}
	if proxy.lastEndpoint.AuthType != enum.EndpointAuthOAuthAnthropic {
		t.Errorf("upstream auth type = %q, want %q", proxy.lastEndpoint.AuthType, enum.EndpointAuthOAuthAnthropic)
	}
	if proxy.lastEndpoint.OAuthAccountID != "account-1" {
		t.Errorf("upstream account id = %q, want account-1", proxy.lastEndpoint.OAuthAccountID)
	}
}
