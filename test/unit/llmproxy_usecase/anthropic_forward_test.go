// Package llmproxy_usecase 测试 internal/application/llmproxy/usecase
// 的 Anthropic Messages 转发路径
package llmproxy_usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// mockAnthropicProxyForAnthropic 模拟 Anthropic 代理
type mockAnthropicProxyForAnthropic struct{}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessage(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCountTokens(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}

var _ transport.AnthropicProxy = (*mockAnthropicProxyForAnthropic)(nil)

// mockOpenAIProxyForAnthropic 模拟 OpenAI 代理
type mockOpenAIProxyForAnthropic struct{}

func (p *mockOpenAIProxyForAnthropic) ForwardChatCompletion(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardChatCompletionStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardCreateResponse(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) ([]byte, error) {
	return nil, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardCreateResponseStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
	return nil
}

var _ transport.OpenAIProxy = (*mockOpenAIProxyForAnthropic)(nil)

// mockAnthropicListModels 模拟 ListAnthropicModels
type mockAnthropicListModels struct{}

func (m *mockAnthropicListModels) Handle(_ context.Context) (*dto.AnthropicListModelsRsp, error) {
	return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{{ID: "claude-sonnet-4-20250514"}}}, nil
}

var _ usecase.ListAnthropicModels = (*mockAnthropicListModels)(nil)

// mockTaskSubmitter 模拟 TaskSubmitter
type mockTaskSubmitter struct{}

func (m *mockTaskSubmitter) SubmitModelCallAuditTask(_ *dto.ModelCallAuditTask) error {
	return nil
}

func (m *mockTaskSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error {
	return nil
}

var _ usecase.TaskSubmitter = (*mockTaskSubmitter)(nil)

// mockAnthropicCountTokens 模拟 CountTokens
type mockAnthropicCountTokens struct{}

func (m *mockAnthropicCountTokens) Handle(_ context.Context, _ *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}

var _ usecase.CountTokens = (*mockAnthropicCountTokens)(nil)

// buildAnthropicTestEndpoint 创建测试用 Anthropic Endpoint 聚合
func buildAnthropicTestEndpoint(provider enum.ProviderType) *aggregate.Endpoint {
	creds, _ := vo.NewUpstreamCreds("sk-ant-test-api-key", "https://api.anthropic.com", "claude-sonnet-4-20250514")
	ep, _ := aggregate.CreateEndpoint(2, "claude-alias", provider, creds)
	return ep
}

// TestAnthropicCreateMessage_NativeStream 测试 Anthropic Messages Native 流式转发
func TestAnthropicCreateMessage_NativeStream(t *testing.T) {
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockOpenAIProxyForAnthropic{}, mockProxy, &mockTaskSubmitter{})

	stream := true
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_NativeUnary 测试 Anthropic Messages Native 非流式转发
func TestAnthropicCreateMessage_NativeUnary(t *testing.T) {
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockOpenAIProxyForAnthropic{}, mockProxy, &mockTaskSubmitter{})

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_ViaOpenAIStream 测试 Anthropic Messages via OpenAI 流式转发
func TestAnthropicCreateMessage_ViaOpenAIStream(t *testing.T) {
	mockOpenAI := &mockOpenAIProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockOpenAI, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

	stream := true
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_ViaOpenAIUnary 测试 Anthropic Messages via OpenAI 非流式转发
func TestAnthropicCreateMessage_ViaOpenAIUnary(t *testing.T) {
	mockOpenAI := &mockOpenAIProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockOpenAI, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_ModelNotFound 测试 Anthropic Messages 模型未找到
func TestAnthropicCreateMessage_ModelNotFound(t *testing.T) {
	mockResolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockOpenAIProxyForAnthropic{}, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "nonexistent-model",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_NativeStream_UpstreamError 测试 Native 流式转发上游错误
func TestAnthropicCreateMessage_NativeStream_UpstreamError(t *testing.T) {
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockOpenAIProxyForAnthropic{}, mockProxy, &mockTaskSubmitter{})

	stream := true
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_NativeUnary_UpstreamError 测试 Native 非流式转发上游错误
func TestAnthropicCreateMessage_NativeUnary_UpstreamError(t *testing.T) {
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderAnthropic)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockOpenAIProxyForAnthropic{}, mockProxy, &mockTaskSubmitter{})

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_ViaOpenAIStream_UpstreamError 测试 via OpenAI 流式转发上游错误
func TestAnthropicCreateMessage_ViaOpenAIStream_UpstreamError(t *testing.T) {
	mockOpenAI := &mockOpenAIProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockOpenAI, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

	stream := true
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

// TestAnthropicCreateMessage_ViaOpenAIUnary_UpstreamError 测试 via OpenAI 非流式转发上游错误
func TestAnthropicCreateMessage_ViaOpenAIUnary_UpstreamError(t *testing.T) {
	mockOpenAI := &mockOpenAIProxyForAnthropic{}
	mockResolver := &mockResolver{resolveResult: buildAnthropicTestEndpoint(enum.ProviderOpenAI)}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockOpenAI, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}
