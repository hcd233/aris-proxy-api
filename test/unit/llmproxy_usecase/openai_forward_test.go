package llmproxy_usecase

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type mockOpenAIProxy struct {
	chatUnaryCalled          bool
	chatStreamCalled         bool
	responseUnaryCalled      bool
	responseStreamCalled     bool
	readChatStreamCalled     bool
	readResponseStreamCalled bool
	lastChatBody             []byte
	openChatStreamErr        error
	openResponseStreamErr    error
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
	p.readChatStreamCalled = true
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
	if p.openResponseStreamErr != nil {
		return nil, p.openResponseStreamErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockOpenAIProxy) ReadCreateResponseStream(_ context.Context, _ io.ReadCloser, onEvent func(string, []byte) error) error {
	p.readResponseStreamCalled = true
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
	messageUnaryCalled   bool
	messageStreamCalled  bool
	readMessageStreamErr error
	readMessageStreamCnt int
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
	if p.readMessageStreamErr != nil {
		return nil, p.readMessageStreamErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockAnthropicProxyForOpenAI) ReadCreateMessageStream(_ context.Context, _ io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	p.readMessageStreamCnt++
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

// Native 流式请求成功时返回 *port.StreamResult，Protocol 为 OpenAI；Open callback 在 adapter 调用前不执行。
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

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}
	if streamResult.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", streamResult.Protocol, enum.ProtocolKindOpenAI)
	}
	if streamResult.Open == nil {
		t.Fatal("StreamResult.Open callback is nil")
	}
	// 上游 Open 已在 usecase 内建立连接；Read 阶段需等 adapter 调用 Stream.Read
	if !proxy.chatStreamCalled {
		t.Fatal("OpenChatCompletionStream must be called by usecase to establish upstream")
	}
	if proxy.readChatStreamCalled {
		t.Fatal("ReadChatCompletionStream must not be called until adapter invokes Stream.Read")
	}
}

// 流式请求在上游建连即失败时，application 必须以 *port.ProxyError 透传上游状态码与错误体，
// 而非返回 200 + SSE 错误帧（Open 错误发生在 SSE 头写出之前，不应进入流）。
// HTTP 状态码与错误体透传由 adapter 根据 ProxyError 写出，本测试在 application 边界锁定该契约。
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

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxyErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", proxyErr.StatusCode, http.StatusTooManyRequests)
	}
	if string(proxyErr.Body) != upstreamErr.Body {
		t.Fatalf("body = %q, want upstream body %q", string(proxyErr.Body), upstreamErr.Body)
	}
	if proxyErr.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", proxyErr.Protocol, enum.ProtocolKindOpenAI)
	}
}

// Native unary 请求成功时返回 *port.JSONResult(200)，Protocol 为 OpenAI，body 为非空 JSON。
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

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", jsonResult.StatusCode, http.StatusOK)
	}
	if jsonResult.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", jsonResult.Protocol, enum.ProtocolKindOpenAI)
	}
	if len(jsonResult.Body) == 0 {
		t.Fatal("JSONResult.Body is empty")
	}
}

// Model not found 时，application 必须以 *port.ProxyError(404) 返回，由 adapter 写为 HTTP 404 JSON 响应。
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

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (model not found must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxyErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", proxyErr.StatusCode, http.StatusNotFound)
	}
	if proxyErr.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", proxyErr.Protocol, enum.ProtocolKindOpenAI)
	}
}

// Responses API native 流式请求成功时返回 *port.StreamResult，Protocol 为 OpenAI。
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

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}
	if streamResult.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", streamResult.Protocol, enum.ProtocolKindOpenAI)
	}
	if streamResult.Open == nil {
		t.Fatal("StreamResult.Open callback is nil")
	}
	if proxy.readResponseStreamCalled {
		t.Fatal("ReadCreateResponseStream must not be called until adapter invokes Stream.Read")
	}
}

// Responses API native unary 请求成功时返回 *port.JSONResult(200)，Protocol 为 OpenAI。
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

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateResponse() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", jsonResult.StatusCode, http.StatusOK)
	}
	if jsonResult.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", jsonResult.Protocol, enum.ProtocolKindOpenAI)
	}
	if len(jsonResult.Body) == 0 {
		t.Fatal("JSONResult.Body is empty")
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

	result, err := uc.CreateResponse(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (model not found must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxyErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", proxyErr.StatusCode, http.StatusNotFound)
	}
	if proxyErr.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", proxyErr.Protocol, enum.ProtocolKindOpenAI)
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

// 流式请求上游 Open 阶段失败时，usecase 不得调用 ReadChatCompletionStream。
// 新契约下 Open 错误以 *port.ProxyError 返回（result=nil），由 adapter 在写出 SSE 头之前映射为 HTTP 错误。
// 本测试锁定该行为，避免后续迁移把 Open 错误误延后到 SSE body 阶段。
func TestOpenAICreateChatCompletion_NativeStream_OpenErrorSkipsRead(t *testing.T) {
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

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxy.readChatStreamCalled {
		t.Fatal("ReadChatCompletionStream must not be called when Open fails")
	}
}

// Response API 流式请求上游 Open 阶段失败时，usecase 不得调用 ReadCreateResponseStream。
func TestOpenAICreateResponse_NativeStream_OpenErrorSkipsRead(t *testing.T) {
	t.Parallel()
	upstreamErr := &model.UpstreamError{
		StatusCode: http.StatusTooManyRequests,
		Body:       `{"error":{"message":"rate limited"}}`,
	}
	proxy := &mockOpenAIProxy{openResponseStreamErr: upstreamErr}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil)

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxy.readResponseStreamCalled {
		t.Fatal("ReadCreateResponseStream must not be called when Open fails")
	}
}

// OpenAI Chat 跨协议（经 Anthropic 上游）流式请求 Open 失败时，不得调用 Anthropic ReadCreateMessageStream。
func TestOpenAICreateChatCompletion_ViaAnthropicStream_OpenErrorSkipsRead(t *testing.T) {
	t.Parallel()
	upstreamErr := &model.UpstreamError{
		StatusCode: http.StatusUnauthorized,
		Body:       `{"error":{"message":"unauthorized"}}`,
	}
	anthropicProxy := &mockAnthropicProxyForOpenAI{readMessageStreamErr: upstreamErr}
	openAIProxy := &mockOpenAIProxy{}
	// 只支持 Anthropic，强制走 ViaAnthropicMessage 跨协议路径
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("anthropic-only", false, false, true), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, openAIProxy, anthropicProxy, &mockTaskSubmitter{}, nil)

	stream := true
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model:    "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}}},
		Stream:   &stream,
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if anthropicProxy.readMessageStreamCnt != 0 {
		t.Fatalf("ReadCreateMessageStream must not be called when Open fails; got cnt=%d", anthropicProxy.readMessageStreamCnt)
	}
}
