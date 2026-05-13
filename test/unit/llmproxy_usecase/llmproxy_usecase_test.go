package llmproxy_usecase

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
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

func (r *mockReadRepo) ListAliases(_ context.Context) ([]*llmproxy.ModelAliasProjection, error) {
	return r.listAliasesResult, r.listAliasesErr
}

func (r *mockReadRepo) FindEndpointByAlias(_ context.Context, _ string) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	return r.findResult, r.findModelResult, r.findErr
}

type mockAnthropicProxy struct {
	forwardCountTokensCalled bool
	forwardCountTokensResult *dto.AnthropicTokensCount
	forwardCountTokensErr    error
}

func (p *mockAnthropicProxy) ForwardCountTokens(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	p.forwardCountTokensCalled = true
	return p.forwardCountTokensResult, p.forwardCountTokensErr
}

func (p *mockAnthropicProxy) ForwardCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	return nil, nil
}

func (p *mockAnthropicProxy) ForwardCreateMessage(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	return nil, nil
}

var _ transport.AnthropicProxy = (*mockAnthropicProxy)(nil)

func TestListOpenAIModels_Success(t *testing.T) {
	repo := newMockReadRepo([]string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"})
	query := usecase.NewListOpenAIModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("Handle() returned nil response")
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
	repo := newMockReadRepo([]string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022"})
	query := usecase.NewListAnthropicModels(repo)

	rsp, err := query.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("Handle() returned nil response")
	}
	if len(rsp.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(rsp.Data))
	}
	if rsp.HasMore != false {
		t.Errorf("HasMore = %v, want false", rsp.HasMore)
	}
}

func TestListAnthropicModels_Empty(t *testing.T) {
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
	repo := &mockReadRepo{
		findResult:      nil,
		findModelResult: nil,
		findErr:         nil,
	}
	proxy := &mockAnthropicProxy{}
	query := usecase.NewCountTokens(repo, proxy)

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
