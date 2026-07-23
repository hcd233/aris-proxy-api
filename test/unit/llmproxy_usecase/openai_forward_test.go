package llmproxy_usecase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type mockOpenAIProxy struct {
	chatUnaryCalled      bool
	chatStreamCalled     bool
	responseUnaryCalled  bool
	responseStreamCalled bool
	lastChatBody         []byte
	openChatStreamErr    error
}

func (p *mockOpenAIProxy) ForwardChatCompletion(_ context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	p.chatUnaryCalled = true
	p.lastChatBody = append([]byte(nil), body...)
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

func (p *mockOpenAIProxy) OpenChatCompletionStream(_ context.Context, _ vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	p.chatStreamCalled = true
	p.lastChatBody = append([]byte(nil), body...)
	if p.openChatStreamErr != nil {
		return nil, p.openChatStreamErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockOpenAIProxy) ReadChatCompletionStream(_ context.Context, _ io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	chunk := &dto.OpenAIChatCompletionChunk{
		ID: "chatcmpl-test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{Content: lo.ToPtr("ok")},
		}},
	}
	if onChunk != nil {
		_ = onChunk(chunk)
	}
	return &dto.OpenAIChatCompletion{
		ID: "chatcmpl-test",
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

func (p *mockOpenAIProxy) OpenCreateResponseStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (io.ReadCloser, error) {
	p.responseStreamCalled = true
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockOpenAIProxy) ReadCreateResponseStream(_ context.Context, _ io.ReadCloser, onEvent func(string, []byte) error) error {
	if onEvent != nil {
		_ = onEvent("response.completed", []byte(`{"type":"response.completed","response":{"id":"resp_test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`))
	}
	return nil
}

var _ usecase.OpenAIProxyPort = (*mockOpenAIProxy)(nil)

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
		return nil, nil, ierr.New(ierr.ErrInternal, "endpoint unsupported")
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

func (p *mockAnthropicProxyForOpenAI) OpenCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (io.ReadCloser, error) {
	p.messageStreamCalled = true
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockAnthropicProxyForOpenAI) ReadCreateMessageStream(_ context.Context, _ io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	if onEvent != nil {
		_ = onEvent(dto.AnthropicSSEEvent{Event: enum.AnthropicSSEEventTypeContentBlockDelta, Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`)})
	}
	return &dto.AnthropicMessage{
		ID:      "msg-test",
		Type:    "message",
		Role:    enum.RoleAssistant,
		Content: []*dto.AnthropicContentBlock{{Type: enum.AnthropicContentBlockTypeText, Text: lo.ToPtr("ok")}},
		Usage:   &dto.AnthropicUsage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

func (p *mockAnthropicProxyForOpenAI) ForwardCountTokens(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 1}, nil
}

func buildTestModel() *aggregate.Model {
	m, _ := aggregate.CreateModel(1, "test-alias", "test-model", 1, true, 128000, 64000)
	return m
}

func TestOpenAICreateChatCompletion_NativeStream(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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

// 流式请求在上游建连即失败时，HTTP 状态码与错误体必须透传上游，而非 200 + SSE 错误帧。
func TestOpenAICreateChatCompletion_StreamOpenErrorPassthrough(t *testing.T) {
	t.Parallel()
	upstreamErr := &model.UpstreamError{
		StatusCode: http.StatusTooManyRequests,
		Body:       `{"error":{"message":"rate limited"}}`,
	}
	proxy := &mockOpenAIProxy{openChatStreamErr: upstreamErr}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

	stream := true
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model:    "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}}},
		Stream:   &stream,
	}}

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "chatStream",
		Method:      http.MethodPost,
		Path:        "/chat",
	}, func(ctx context.Context, _ *struct{}) (*huma.StreamResponse, error) {
		return uc.CreateChatCompletion(ctx, req)
	})

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/chat", http.NoBody)
	resp, err := app.Test(httpReq, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("chat stream request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != upstreamErr.Body {
		t.Fatalf("body = %q, want upstream body %q", string(body), upstreamErr.Body)
	}
}

func TestOpenAICreateChatCompletion_NativeUnary(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	resolver := &mockResolver{resolveErr: ierr.New(ierr.ErrInternal, "model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	resolver := &mockResolver{resolveErr: ierr.New(ierr.ErrInternal, "model not found")}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("anthropic-only", false, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-only", true, false, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("anthropic-only", false, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{}, nil)

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
	t.Parallel()
	openAIProxy := &mockOpenAIProxy{}
	anthropicProxy := &mockAnthropicProxyForOpenAI{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-and-anthropic", true, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{}, nil)

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
