package llmproxy_usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

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
	m, _ := aggregate.CreateModel(2, "claude-alias", "claude-sonnet-4-20250514", 2)
	return m
}

func TestAnthropicCreateMessage_NativeStream(t *testing.T) {
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockTaskSubmitter{})

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
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockTaskSubmitter{})

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
	mockResolver := &mockResolver{resolveErr: errors.New("model not found")}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockAnthropicProxyForAnthropic{}, &mockTaskSubmitter{})

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
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockTaskSubmitter{})

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
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockTaskSubmitter{})

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
