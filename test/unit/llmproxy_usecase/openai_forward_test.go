// Package llmproxy_usecase 测试 internal/application/llmproxy/usecase
// 的 OpenAI ChatCompletion 转发路径
package llmproxy_usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// mockOpenAIProxy 模拟 OpenAI 代理
type mockOpenAIProxy struct {
	chatUnaryCalled      bool
	chatStreamCalled     bool
	responseUnaryCalled  bool
	responseStreamCalled bool
}

func (p *mockOpenAIProxy) ForwardChatCompletion(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
	p.chatUnaryCalled = true
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardChatCompletionStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	p.chatStreamCalled = true
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardCreateResponse(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) ([]byte, error) {
	p.responseUnaryCalled = true
	return []byte(`{"status":"completed"}`), nil
}

func (p *mockOpenAIProxy) ForwardCreateResponseStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
	p.responseStreamCalled = true
	return nil
}

var _ transport.OpenAIProxy = (*mockOpenAIProxy)(nil)

// mockOpenAIAnthropicProxy 模拟 Anthropic 代理
type mockOpenAIAnthropicProxy struct {
	messageUnaryCalled  bool
	messageStreamCalled bool
	countTokensCalled   bool
}

func (p *mockOpenAIAnthropicProxy) ForwardCreateMessage(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	p.messageUnaryCalled = true
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockOpenAIAnthropicProxy) ForwardCreateMessageStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	p.messageStreamCalled = true
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockOpenAIAnthropicProxy) ForwardCountTokens(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	p.countTokensCalled = true
	return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}

var _ transport.AnthropicProxy = (*mockOpenAIAnthropicProxy)(nil)

// mockResolver 模拟 EndpointResolver
type mockResolver struct {
	resolveResult *aggregate.Endpoint
	resolveErr    error
}

func (r *mockResolver) Resolve(_ context.Context, _ vo.EndpointAlias, _, _ enum.ProviderType) (*aggregate.Endpoint, error) {
	return r.resolveResult, r.resolveErr
}

var _ service.EndpointResolver = (*mockResolver)(nil)

// mockListModels 模拟 ListOpenAIModels
type mockListModels struct{}

func (m *mockListModels) Handle(_ context.Context) (*dto.OpenAIListModelsRsp, error) {
	return &dto.OpenAIListModelsRsp{Object: "list", Data: []*dto.OpenAIModel{{ID: "test"}}}, nil
}

var _ usecase.ListOpenAIModels = (*mockListModels)(nil)

// buildTestEndpoint 创建测试用 Endpoint 聚合
func buildTestEndpoint(provider enum.ProviderType) *aggregate.Endpoint {
	creds, _ := vo.NewUpstreamCreds("test-api-key", "https://api.test.com", "test-model")
	ep, _ := aggregate.CreateEndpoint(1, "test-alias", provider, creds)
	return ep
}

// TestOpenAICreateChatCompletion_NativeStream 测试 OpenAI ChatCompletion Native 流式转发
func TestOpenAICreateChatCompletion_NativeStream(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockOpenAIAnthropicProxy{})

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

// TestOpenAICreateChatCompletion_NativeUnary 测试 OpenAI ChatCompletion Native 非流式转发
func TestOpenAICreateChatCompletion_NativeUnary(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockOpenAIAnthropicProxy{})

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

// TestOpenAICreateChatCompletion_ViaAnthropicStream 测试 OpenAI ChatCompletion via Anthropic 流式转发
func TestOpenAICreateChatCompletion_ViaAnthropicStream(t *testing.T) {
	anthropicProxy := &mockOpenAIAnthropicProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, anthropicProxy)

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

// TestOpenAICreateChatCompletion_ViaAnthropicUnary 测试 OpenAI ChatCompletion via Anthropic 非流式转发
func TestOpenAICreateChatCompletion_ViaAnthropicUnary(t *testing.T) {
	anthropicProxy := &mockOpenAIAnthropicProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, anthropicProxy)

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

// TestOpenAICreateChatCompletion_ModelNotFound 测试模型未找到时返回错误响应
func TestOpenAICreateChatCompletion_ModelNotFound(t *testing.T) {
	resolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockOpenAIAnthropicProxy{})

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

// TestOpenAICreateResponse_NativeStream 测试 OpenAI Response API Native 流式转发
func TestOpenAICreateResponse_NativeStream(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockOpenAIAnthropicProxy{})

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  "test-alias",
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

// TestOpenAICreateResponse_NativeUnary 测试 OpenAI Response API Native 非流式转发
func TestOpenAICreateResponse_NativeUnary(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockOpenAIAnthropicProxy{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  "test-alias",
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

// TestOpenAICreateResponse_ViaAnthropicStream 测试 OpenAI Response API via Anthropic 流式转发
func TestOpenAICreateResponse_ViaAnthropicStream(t *testing.T) {
	anthropicProxy := &mockOpenAIAnthropicProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, anthropicProxy)

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  "test-alias",
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

// TestOpenAICreateResponse_ViaAnthropicUnary 测试 OpenAI Response API via Anthropic 非流式转发
func TestOpenAICreateResponse_ViaAnthropicUnary(t *testing.T) {
	anthropicProxy := &mockOpenAIAnthropicProxy{}
	resolver := &mockResolver{resolveResult: buildTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, anthropicProxy)

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  "test-alias",
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

// TestOpenAICreateResponse_ModelNotFound 测试 Response API 模型未找到
func TestOpenAICreateResponse_ModelNotFound(t *testing.T) {
	resolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockOpenAIAnthropicProxy{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  "nonexistent-model",
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
