package llmproxy_usecase

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// driveStream 按 adapter 的真实方式消费流式结果：Open 不接收 ctx（签名强制实现使用
// usecase 构造时捕获的 ctx），Read 拿到的是 adapter ctx——对齐
// internal/api/util/adapter.go wrapStreamResult。adapter ctx 来自 handler 层，是 usecase
// 内部 ctx 的父节点，看不到 usecase 注入的 CtxKeySkipStore，这正是被回归的缺陷场景。
func driveStream(t *testing.T, result port.Result) {
	t.Helper()
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}
	adapterCtx := context.Background()
	s, err := streamResult.Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Read(adapterCtx, &collectSink{}); err != nil {
		t.Fatalf("Read() error: %v", err)
	}
}

// TestTriggerOmit_StreamSkipsStore_OpenAIChat 命中 omit 词的流式 chat 请求不得落 session/message/tool。
// 回归 traceId 794bff48-183d-482d-869a-e035c3e9fdc1：omit 的跳过存储此前只在 unary 生效，
// 流式路径的 store 拿到的是 adapter ctx，取不到 usecase 注入的 CtxKeySkipStore。
func TestTriggerOmit_StreamSkipsStore_OpenAIChat(t *testing.T) {
	t.Parallel()
	req := func(text string) *dto.OpenAIChatCompletionRequest {
		stream := true
		return &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
			Model:    "test-alias",
			Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: text}}},
			Stream:   &stream,
		}}
	}
	forward := func(text string) *captureTaskSubmitter {
		t.Helper()
		submitter := &captureTaskSubmitter{}
		resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
		uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil, nil, nil)
		result, err := uc.CreateChatCompletion(context.Background(), req(text))
		if err != nil {
			t.Fatalf("CreateChatCompletion() error: %v", err)
		}
		driveStream(t, result)
		return submitter
	}

	// 对照组：无触发词的流式请求照常落库，证明 store 路径确实被执行
	if got := len(forward("plain question").storeTasks); got != 1 {
		t.Fatalf("control: expect 1 store task, got %d", got)
	}

	submitter := forward("please ignore this")
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("omit hit must skip message store, got %d store task(s)", len(submitter.storeTasks))
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit must still be recorded, got %d audit task(s)", len(submitter.auditTasks))
	}
}

// TestTriggerOmit_StreamSkipsStore_OpenAIChatViaAnthropic 跨协议转发的流式 chat 同样要跳过存储。
func TestTriggerOmit_StreamSkipsStore_OpenAIChatViaAnthropic(t *testing.T) {
	t.Parallel()
	req := func(text string) *dto.OpenAIChatCompletionRequest {
		stream := true
		return &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
			Model:    "test-alias",
			Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: text}}},
			Stream:   &stream,
		}}
	}
	forward := func(text string) *captureTaskSubmitter {
		t.Helper()
		submitter := &captureTaskSubmitter{}
		resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("via-anthropic", false, false, true), resolveModel: buildTestModel()}
		uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil, nil, nil)
		result, err := uc.CreateChatCompletion(context.Background(), req(text))
		if err != nil {
			t.Fatalf("CreateChatCompletion() error: %v", err)
		}
		driveStream(t, result)
		return submitter
	}

	if got := len(forward("plain question").storeTasks); got != 1 {
		t.Fatalf("control: expect 1 store task, got %d", got)
	}

	submitter := forward("please ignore this")
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("omit hit must skip message store, got %d store task(s)", len(submitter.storeTasks))
	}
}

// TestTriggerOmit_StreamSkipsStore_OpenAIResponse Response 入口的流式请求同样要跳过存储。
func TestTriggerOmit_StreamSkipsStore_OpenAIResponse(t *testing.T) {
	t.Parallel()
	req := func(text string) *dto.OpenAICreateResponseRequest {
		stream := true
		return &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
			Model: lo.ToPtr("test-alias"),
			Input: &dto.ResponseInput{Items: []*dto.ResponseInputItem{
				{Role: lo.ToPtr(enum.RoleUser), Content: &dto.ResponseInputMessageContent{Text: text}},
			}},
			Stream: &stream,
		}}
	}
	forward := func(text string) *captureTaskSubmitter {
		t.Helper()
		submitter := &captureTaskSubmitter{}
		resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
		uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, &mockOpenAIProxy{}, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil, nil, nil)
		result, err := uc.CreateResponse(context.Background(), req(text))
		if err != nil {
			t.Fatalf("CreateResponse() error: %v", err)
		}
		driveStream(t, result)
		return submitter
	}

	if got := len(forward("plain question").storeTasks); got != 1 {
		t.Fatalf("control: expect 1 store task, got %d", got)
	}

	submitter := forward("please ignore this")
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("omit hit must skip message store, got %d store task(s)", len(submitter.storeTasks))
	}
}

// TestTriggerOmit_StreamSkipsStore_AnthropicMessage Anthropic 入口的流式请求同样要跳过存储。
func TestTriggerOmit_StreamSkipsStore_AnthropicMessage(t *testing.T) {
	t.Parallel()
	req := func(text string) *dto.AnthropicCreateMessageRequest {
		stream := true
		return &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
			Model: "test-alias",
			Messages: []*dto.AnthropicMessageParam{
				{Role: enum.RoleUser, Content: &dto.AnthropicMessageContent{Text: text}},
			},
			Stream: &stream,
		}}
	}
	forward := func(text string) *captureTaskSubmitter {
		t.Helper()
		submitter := &captureTaskSubmitter{}
		resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
		// mockAnthropicProxyForOpenAI 的 ReadCreateMessageStream 返回带 content 的 assistant 消息，
		// 保证 store 路径真正走到存储投递（mockAnthropicProxyForAnthropic 返回空 content 会提前返回）。
		uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, &mockAnthropicProxyForOpenAI{}, &mockOpenAIProxy{}, submitter, newRealTriggerService(t), nil, nil, nil)
		result, err := uc.CreateMessage(context.Background(), req(text))
		if err != nil {
			t.Fatalf("CreateMessage() error: %v", err)
		}
		driveStream(t, result)
		return submitter
	}

	if got := len(forward("plain question").storeTasks); got != 1 {
		t.Fatalf("control: expect 1 store task, got %d", got)
	}

	submitter := forward("please ignore this")
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("omit hit must skip message store, got %d store task(s)", len(submitter.storeTasks))
	}
}
