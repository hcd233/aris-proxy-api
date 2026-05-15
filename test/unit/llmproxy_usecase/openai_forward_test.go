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

type mockOpenAIProxy struct {
	chatUnaryCalled      bool
	chatStreamCalled     bool
	responseUnaryCalled  bool
	responseStreamCalled bool
}

func (p *mockOpenAIProxy) ForwardChatCompletion(_ context.Context, ep vo.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
	p.chatUnaryCalled = true
	return &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-test",
		Model: ep.Model,
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role:    enum.RoleAssistant,
				Content: &dto.OpenAIMessageContent{Text: "ok"},
			},
			FinishReason: enum.FinishReasonStop,
		}},
		Usage: &dto.OpenAICompletionUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *mockOpenAIProxy) ForwardChatCompletionStream(_ context.Context, ep vo.UpstreamEndpoint, _ []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	p.chatStreamCalled = true
	chunk := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-test",
		Model: ep.Model,
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{Content: "ok"},
		}},
	}
	if onChunk != nil {
		_ = onChunk(chunk)
	}
	return &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-test",
		Model: ep.Model,
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Message:      &dto.OpenAIChatCompletionMessageParam{Role: enum.RoleAssistant, Content: &dto.OpenAIMessageContent{Text: "ok"}},
			FinishReason: enum.FinishReasonStop,
		}},
		Usage: &dto.OpenAICompletionUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *mockOpenAIProxy) ForwardCreateResponse(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) ([]byte, error) {
	p.responseUnaryCalled = true
	return []byte(`{"id":"resp_test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`), nil
}

func (p *mockOpenAIProxy) ForwardCreateResponseStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, onEvent func(string, []byte) error) error {
	p.responseStreamCalled = true
	if onEvent != nil {
		_ = onEvent("response.completed", []byte(`{"type":"response.completed","response":{"id":"resp_test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`))
	}
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
	return buildCompatEndpoint("test-endpoint", true, true, false)
}

func buildCompatEndpoint(name string, supportChat, supportResponse, supportMessage bool) *aggregate.Endpoint {
	openaiBaseURL := ""
	if supportChat || supportResponse {
		openaiBaseURL = "https://api.openai.com"
	}
	anthropicBaseURL := ""
	if supportMessage {
		anthropicBaseURL = "https://api.anthropic.com"
	}
	ep, _ := aggregate.CreateEndpoint(1, name, openaiBaseURL, anthropicBaseURL, "test-api-key", supportChat, supportResponse, supportMessage)
	return ep
}

type mockAnthropicProxyForOpenAI struct {
	messageUnaryCalled  bool
	messageStreamCalled bool
}

func (p *mockAnthropicProxyForOpenAI) ForwardCreateMessage(_ context.Context, ep vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	p.messageUnaryCalled = true
	return &dto.AnthropicMessage{
		ID:      "msg-test",
		Type:    "message",
		Role:    enum.RoleAssistant,
		Model:   ep.Model,
		Content: []*dto.AnthropicContentBlock{{Type: enum.AnthropicContentBlockTypeText, Text: lo.ToPtr("ok")}},
		Usage:   &dto.AnthropicUsage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

func (p *mockAnthropicProxyForOpenAI) ForwardCreateMessageStream(_ context.Context, ep vo.UpstreamEndpoint, _ []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	p.messageStreamCalled = true
	if onEvent != nil {
		_ = onEvent(dto.AnthropicSSEEvent{Event: enum.AnthropicSSEEventTypeContentBlockDelta, Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`)})
	}
	return &dto.AnthropicMessage{
		ID:      "msg-test",
		Type:    "message",
		Role:    enum.RoleAssistant,
		Model:   ep.Model,
		Content: []*dto.AnthropicContentBlock{{Type: enum.AnthropicContentBlockTypeText, Text: lo.ToPtr("ok")}},
		Usage:   &dto.AnthropicUsage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

func (p *mockAnthropicProxyForOpenAI) ForwardCountTokens(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 1}, nil
}

func buildTestModel() *aggregate.Model {
	m, _ := aggregate.CreateModel(1, "test-alias", "test-model", 1)
	return m
}

func TestOpenAICreateChatCompletion_NativeStream(t *testing.T) {
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{})

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

func TestOpenAICreateChatCompletion_AnthropicOnlyUsesAnthropicCompatibility(t *testing.T) {
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("anthropic-only", false, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model:    "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}}},
		Stream:   &stream,
	}}

	rsp, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateChatCompletion() returned nil response")
	}
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIChat, resolver.resolveEndpoint); route != enum.CompatRouteViaAnthropicMessage {
		t.Fatalf("route = %v, want via anthropic", route)
	}
	_ = openAIProxy
	_ = anthropicProxy
}

func TestOpenAICreateResponse_ChatOnlyUsesChatCompatibility(t *testing.T) {
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-only", true, false, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Input:  &dto.ResponseInput{Text: "Hello"},
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIResponse, resolver.resolveEndpoint); route != enum.CompatRouteViaOpenAIChat {
		t.Fatalf("route = %v, want via chat", route)
	}
	_ = openAIProxy
	_ = anthropicProxy
}

func TestOpenAICreateResponse_AnthropicOnlyUsesAnthropicCompatibility(t *testing.T) {
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("anthropic-only", false, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Input:  &dto.ResponseInput{Text: "Hello"},
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIResponse, resolver.resolveEndpoint); route != enum.CompatRouteViaAnthropicMessage {
		t.Fatalf("route = %v, want via anthropic", route)
	}
	_ = openAIProxy
	_ = anthropicProxy
}

func TestOpenAICreateResponse_ChatAndAnthropicPrefersChatCompatibility(t *testing.T) {
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-and-anthropic", true, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{})

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Input:  &dto.ResponseInput{Text: "Hello"},
		Stream: &stream,
	}}

	rsp, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateResponse() returned nil response")
	}
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIResponse, resolver.resolveEndpoint); route != enum.CompatRouteViaOpenAIChat {
		t.Fatalf("route = %v, want via chat", route)
	}
	_ = openAIProxy
	_ = anthropicProxy
}
