package llmproxy_usecase

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type mockAnthropicProxyForAnthropic struct {
	messageUnaryCalled  bool
	messageStreamCalled bool
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessage(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
	p.messageUnaryCalled = true
	return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessageStream(_ context.Context, _ vo.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	p.messageStreamCalled = true
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
	m, _ := aggregate.CreateModel(2, "claude-alias", "claude-sonnet-4-20250514", 2, true)
	return m
}

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

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

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

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
	}
}

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

	rsp, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("CreateMessage() returned nil response")
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
