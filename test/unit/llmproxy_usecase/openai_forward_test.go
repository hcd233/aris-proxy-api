package llmproxy_usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

type mockOpenAIProxy struct{}

func (p *mockOpenAIProxy) ForwardChatCompletion(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardChatCompletionStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardCreateResponse(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) ([]byte, error) {
	return []byte(`{"status":"completed"}`), nil
}

func (p *mockOpenAIProxy) ForwardCreateResponseStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
	return nil
}

var _ transport.OpenAIProxy = (*mockOpenAIProxy)(nil)

type mockResolver struct {
	resolveEndpoint *aggregate.Endpoint
	resolveModel    *aggregate.Model
	resolveErr      error
}

func (r *mockResolver) Resolve(_ context.Context, _ vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) (*aggregate.Endpoint, *aggregate.Model, error) {
	if r.resolveErr != nil || r.resolveEndpoint == nil {
		return r.resolveEndpoint, r.resolveModel, r.resolveErr
	}
	if matcher != nil && !matcher(r.resolveEndpoint) {
		return nil, nil, errors.New("endpoint unsupported")
	}
	return r.resolveEndpoint, r.resolveModel, nil
}

type mockListModels struct{}

func (m *mockListModels) Handle(_ context.Context) (*dto.OpenAIListModelsRsp, error) {
	return &dto.OpenAIListModelsRsp{Object: "list", Data: []*dto.OpenAIModel{{ID: "test"}}}, nil
}

var _ usecase.ListOpenAIModels = (*mockListModels)(nil)

func buildTestEndpoint() *aggregate.Endpoint {
	ep, _ := aggregate.CreateEndpoint(1, "test-endpoint", "https://api.openai.com", "https://api.anthropic.com", "test-api-key", true, true, false)
	return ep
}

func buildTestModel() *aggregate.Model {
	m, _ := aggregate.CreateModel(1, "test-alias", "test-model", 1)
	return m
}

func TestOpenAICreateChatCompletion_NativeStream(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockTaskSubmitter{})

	stream := true
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateChatCompletion() returned nil response")
	}
}

func TestOpenAICreateChatCompletion_NativeUnary(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateChatCompletion() returned nil response")
	}
}

func TestOpenAICreateChatCompletion_ModelNotFound(t *testing.T) {
	resolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "nonexistent-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateChatCompletion() returned nil response")
	}
}

func TestOpenAICreateResponse_NativeStream(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockTaskSubmitter{})

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
}

func TestOpenAICreateResponse_NativeUnary(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
}

func TestOpenAICreateResponse_ModelNotFound(t *testing.T) {
	resolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("nonexistent-model"),
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
}
