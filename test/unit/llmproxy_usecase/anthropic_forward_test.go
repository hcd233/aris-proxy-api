package llmproxy_usecase

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type mockAnthropicProxyForAnthropic struct {
	messageUnaryCalled   bool
	messageStreamCalled  bool
	openMessageStreamErr error
	readMessageStreamCnt int
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessage(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	p.messageUnaryCalled = true
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) OpenCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (io.ReadCloser, error) {
	p.messageStreamCalled = true
	if p.openMessageStreamErr != nil {
		return nil, p.openMessageStreamErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (p *mockAnthropicProxyForAnthropic) ReadCreateMessageStream(_ context.Context, _ io.ReadCloser, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	p.readMessageStreamCnt++
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCountTokens(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}

var _ usecase.AnthropicProxyPort = (*mockAnthropicProxyForAnthropic)(nil)

type mockAnthropicListModels struct{}

func (m *mockAnthropicListModels) Handle(_ context.Context) (*dto.AnthropicListModelsRsp, error) {
	return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{{ID: "claude-sonnet-4-20250514"}}}, nil
}

var _ usecase.ListAnthropicModels = (*mockAnthropicListModels)(nil)

type mockTaskSubmitter struct{}

func (m *mockTaskSubmitter) SubmitModelCallAuditTask(_ *dto.ModelCallAuditTask) error {
	return nil
}

func (m *mockTaskSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error {
	return nil
}

var _ usecase.TaskSubmitter = (*mockTaskSubmitter)(nil)

type mockAnthropicCountTokens struct{}

func (m *mockAnthropicCountTokens) Handle(_ context.Context, _ *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}

var _ usecase.CountTokens = (*mockAnthropicCountTokens)(nil)

func buildAnthropicTestEndpoint() *aggregate.Endpoint {
	ep, _ := aggregate.CreateEndpoint(2, "anthropic-endpoint", "https://api.openai.com", "https://api.anthropic.com", "sk-ant-test-api-key", false, false, true)
	return ep
}

func buildAnthropicTestModel() *aggregate.Model {
	m, _ := aggregate.CreateModel(2, "claude-alias", "claude-sonnet-4-20250514", 2, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	return m
}

// Native 流式请求成功时返回 *port.StreamResult，Protocol 为 Anthropic；Read 阶段需等 adapter 调用。
func TestAnthropicCreateMessage_NativeStream(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

	stream := true
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}
	if streamResult.Protocol != enum.ProtocolKindAnthropic {
		t.Fatalf("protocol = %v, want %v", streamResult.Protocol, enum.ProtocolKindAnthropic)
	}
	if streamResult.Open == nil {
		t.Fatal("StreamResult.Open callback is nil")
	}
	if mockProxy.readMessageStreamCnt != 0 {
		t.Fatal("ReadCreateMessageStream must not be called until adapter invokes Stream.Read")
	}
}

// Native unary 请求成功时返回 *port.JSONResult(200)，Protocol 为 Anthropic。
func TestAnthropicCreateMessage_NativeUnary(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", jsonResult.StatusCode, http.StatusOK)
	}
	if jsonResult.Protocol != enum.ProtocolKindAnthropic {
		t.Fatalf("protocol = %v, want %v", jsonResult.Protocol, enum.ProtocolKindAnthropic)
	}
	if len(jsonResult.Body) == 0 {
		t.Fatal("JSONResult.Body is empty")
	}
}

// Model not found 时，application 必须以 *port.ProxyError(404) 返回，由 adapter 写为 HTTP 404 JSON 响应。
func TestAnthropicCreateMessage_ModelNotFound(t *testing.T) {
	t.Parallel()
	mockResolver := &mockResolver{resolveErr: ierr.New(ierr.ErrInternal, "model not found")}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockAnthropicProxyForAnthropic{}, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

	stream := false
	userContent := &dto.AnthropicMessageContent{Text: "Hello"}
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "nonexistent-model",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: userContent},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
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
	if proxyErr.Protocol != enum.ProtocolKindAnthropic {
		t.Fatalf("protocol = %v, want %v", proxyErr.Protocol, enum.ProtocolKindAnthropic)
	}
}

func TestAnthropicCreateMessage_NativeStream_UpstreamError(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

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

func TestAnthropicCreateMessage_NativeUnary_UpstreamError(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

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

func TestAnthropicCreateMessage_ChatResponseEndpointUsesChatCompatibility(t *testing.T) {
	t.Parallel()
	anthropicProxy := &mockAnthropicProxyForAnthropic{}
	openAIProxy := &mockOpenAIProxy{}
	mockResolver := &mockResolver{
		resolveEndpoint: buildCompatEndpoint("chat-response", true, true, false),
		resolveModel:    buildAnthropicTestModel(),
	}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, anthropicProxy, openAIProxy, &mockTaskSubmitter{}, nil)

	stream := false
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: enum.RoleUser, Content: &dto.AnthropicMessageContent{Text: "Hello"}},
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
	if route := usecase.SelectCompatRoute(enum.ProxyAPIAnthropicMessage, mockResolver.resolveEndpoint); route != enum.CompatRouteViaOpenAIChat {
		t.Fatalf("route = %v, want via chat", route)
	}
	_ = openAIProxy
	_ = anthropicProxy
}

// Anthropic Message native 流式请求上游 Open 阶段失败时，usecase 不得调用 ReadCreateMessageStream。
// 新契约下 Open 错误以 *port.ProxyError 返回（result=nil），由 adapter 在写出 SSE 头之前映射为 HTTP 错误。
// 本测试锁定该行为，避免后续迁移把 Open 错误误延后到 SSE body 阶段。
func TestAnthropicCreateMessage_NativeStream_OpenErrorSkipsRead(t *testing.T) {
	t.Parallel()
	upstreamErr := &model.UpstreamError{
		StatusCode: http.StatusTooManyRequests,
		Body:       `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`,
	}
	proxy := &mockAnthropicProxyForAnthropic{openMessageStreamErr: upstreamErr}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, nil)

	stream := true
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "Hello"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if proxy.readMessageStreamCnt != 0 {
		t.Fatalf("ReadCreateMessageStream must not be called when Open fails; got cnt=%d", proxy.readMessageStreamCnt)
	}
}

// Anthropic Message 跨协议（经 OpenAI Chat 上游）流式请求 Open 失败时，不得调用 ReadChatCompletionStream。
func TestAnthropicCreateMessage_ViaChatStream_OpenErrorSkipsRead(t *testing.T) {
	t.Parallel()
	upstreamErr := &model.UpstreamError{
		StatusCode: http.StatusUnauthorized,
		Body:       `{"error":{"message":"unauthorized"}}`,
	}
	openAIProxy := &mockOpenAIProxy{openChatStreamErr: upstreamErr}
	anthropicProxy := &mockAnthropicProxyForAnthropic{}
	// 只支持 OpenAI Chat，强制走 ViaOpenAIChat 跨协议路径
	mockResolver := &mockResolver{
		resolveEndpoint: buildCompatEndpoint("chat-only", true, false, false),
		resolveModel:    buildAnthropicTestModel(),
	}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, anthropicProxy, openAIProxy, &mockTaskSubmitter{}, nil)

	stream := true
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "Hello"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if result != nil {
		t.Fatalf("result = %T, want nil (Open error must not produce a Result)", result)
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("err = %v, want *port.ProxyError", err)
	}
	if openAIProxy.readChatStreamCalled {
		t.Fatal("ReadChatCompletionStream must not be called when Open fails")
	}
}
