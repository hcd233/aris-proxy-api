// Package llmproxy_usecase 测试 internal/application/llmproxy/usecase
// 的查询用例：ListModels 和 CountTokens
package llmproxy_usecase

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// mockReadRepo 实现 llmproxy.EndpointReadRepository 接口
type mockReadRepo struct {
	listAliasesResult []*llmproxy.EndpointAliasProjection
	listAliasesErr    error
	findCredsResult   *llmproxy.EndpointCredentialProjection
	findCredsErr      error
}

func newMockReadRepo(aliases []string) *mockReadRepo {
	projections := make([]*llmproxy.EndpointAliasProjection, len(aliases))
	for i, alias := range aliases {
		projections[i] = &llmproxy.EndpointAliasProjection{Alias: alias}
	}
	return &mockReadRepo{
		listAliasesResult: projections,
	}
}

func (r *mockReadRepo) ListAliasesByProvider(_ context.Context, _ enum.ProviderType) ([]*llmproxy.EndpointAliasProjection, error) {
	return r.listAliasesResult, r.listAliasesErr
}

func (r *mockReadRepo) FindCredentialByAliasAndProvider(_ context.Context, _ string, _ enum.ProviderType) (*llmproxy.EndpointCredentialProjection, error) {
	return r.findCredsResult, r.findCredsErr
}

// mockAnthropicProxy 模拟 AnthropicProxy
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

// Ensure mockAnthropicProxy implements transport.AnthropicProxy
var _ transport.AnthropicProxy = (*mockAnthropicProxy)(nil)

// TestListOpenAIModels_Success 测试 OpenAI 模型列表查询成功
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

// TestListOpenAIModels_Empty 测试空模型列表
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

// TestListAnthropicModels_Success 测试 Anthropic 模型列表查询成功
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

// TestListAnthropicModels_Empty 测试空模型列表
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

// TestListModels_Pagination 测试分页字段正确设置
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

// TestCountTokens_ModelNotFound 测试模型不存在时返回空结果
func TestCountTokens_ModelNotFound(t *testing.T) {
	repo := &mockReadRepo{
		findCredsResult: nil,
		findCredsErr:    nil,
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
	// Should return empty result, not error (matches old behavior)
	if rsp.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", rsp.InputTokens)
	}
	if proxy.forwardCountTokensCalled {
		t.Error("ForwardCountTokens should not be called when model not found")
	}
}
