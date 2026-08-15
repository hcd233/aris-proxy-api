package llmproxy_usecase

import (
	"context"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// assertChatContentFilterBody 断言 OpenAI Chat 拦截 JSON：200 + finish_reason=content_filter + 固定文案。
func assertChatContentFilterBody(t *testing.T, body []byte) {
	t.Helper()
	var completion dto.OpenAIChatCompletion
	if err := sonic.Unmarshal(body, &completion); err != nil {
		t.Fatalf("failed to unmarshal chat body: %v", err)
	}
	if len(completion.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(completion.Choices))
	}
	choice := completion.Choices[0]
	if choice.FinishReason != enum.FinishReasonContentFilter {
		t.Fatalf("finish_reason = %q, want %q", choice.FinishReason, enum.FinishReasonContentFilter)
	}
	if choice.Message == nil || choice.Message.Content == nil || choice.Message.Content.Text != constant.TriggerContentFilterMessage {
		t.Fatalf("message.content = %+v, want %q", choice.Message, constant.TriggerContentFilterMessage)
	}
	if choice.Message.Role != enum.RoleAssistant {
		t.Fatalf("message.role = %q, want %q", choice.Message.Role, enum.RoleAssistant)
	}
}

// assertResponseContentFilterBody 断言 OpenAI Response 拦截 JSON：
// incomplete_details.reason=content_filter + output 含 refusal part。
func assertResponseContentFilterBody(t *testing.T, body []byte) {
	t.Helper()
	var rsp dto.OpenAICreateResponseRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if rsp.IncompleteDetails == nil || rsp.IncompleteDetails.Reason != "content_filter" {
		t.Fatalf("incomplete_details = %+v, want reason=content_filter", rsp.IncompleteDetails)
	}
	if len(rsp.Output) != 1 {
		t.Fatalf("output = %d items, want 1", len(rsp.Output))
	}
	item := rsp.Output[0]
	if item.Content == nil || len(item.Content.Parts) != 1 {
		t.Fatalf("output[0].content = %+v, want 1 refusal part", item.Content)
	}
	part := item.Content.Parts[0]
	if part.Type != enum.ResponseContentTypeRefusal {
		t.Fatalf("content part type = %q, want %q", part.Type, enum.ResponseContentTypeRefusal)
	}
	if part.Refusal == nil || *part.Refusal != constant.TriggerContentFilterMessage {
		t.Fatalf("content part refusal = %v, want %q", part.Refusal, constant.TriggerContentFilterMessage)
	}
}

// assertAnthropicContentFilterBody 断言 Anthropic 拦截 JSON：stop_reason=refusal + stop_details + 固定文案。
func assertAnthropicContentFilterBody(t *testing.T, body []byte) {
	t.Helper()
	var msg dto.AnthropicMessage
	if err := sonic.Unmarshal(body, &msg); err != nil {
		t.Fatalf("failed to unmarshal anthropic body: %v", err)
	}
	if msg.StopReason == nil || *msg.StopReason != enum.AnthropicStopReasonRefusal {
		t.Fatalf("stop_reason = %v, want %q", msg.StopReason, enum.AnthropicStopReasonRefusal)
	}
	if msg.StopDetails == nil || msg.StopDetails.Type != "refusal" {
		t.Fatalf("stop_details = %+v, want type=refusal", msg.StopDetails)
	}
	if msg.StopDetails.Explanation == nil || *msg.StopDetails.Explanation != constant.TriggerContentFilterMessage {
		t.Fatalf("stop_details.explanation = %v, want %q", msg.StopDetails.Explanation, constant.TriggerContentFilterMessage)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text == nil || *msg.Content[0].Text != constant.TriggerContentFilterMessage {
		t.Fatalf("content = %+v, want text block %q", msg.Content, constant.TriggerContentFilterMessage)
	}
}

// readStreamEvents 消费 StreamResult 的事件序列并断言事件类型列表。
func readStreamEvents(t *testing.T, result port.Result, wantEvents []string) []capturedEvent {
	t.Helper()
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}
	stream, err := streamResult.Open(context.Background())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	sink := &captureSink{}
	if err := stream.Read(context.Background(), sink); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if len(sink.events) != len(wantEvents) {
		t.Fatalf("events = %d, want %d (got: %+v)", len(sink.events), len(wantEvents), sink.events)
	}
	for i, want := range wantEvents {
		if sink.events[i].event != want {
			t.Fatalf("events[%d].event = %q, want %q", i, sink.events[i].event, want)
		}
	}
	return sink.events
}

// TestOpenAICreateChatCompletion_DenyTriggerUnary
// OpenAI Chat 非流式命中 deny 词时返回 200 content_filter JSON，不触达上游。
func TestOpenAICreateChatCompletion_DenyTriggerUnary(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	stream := false
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "这条消息包含敏感词内容"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", jsonResult.StatusCode)
	}
	if jsonResult.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", jsonResult.Protocol, enum.ProtocolKindOpenAI)
	}
	assertChatContentFilterBody(t, jsonResult.Body)
	if proxy.chatUnaryCalled || proxy.chatStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}

// TestOpenAICreateChatCompletion_DenyTriggerStream
// OpenAI Chat 流式命中 deny 词时返回 200 SSE：role → content → finish_reason=content_filter → [DONE]。
func TestOpenAICreateChatCompletion_DenyTriggerStream(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	stream := true
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "敏感词"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	events := readStreamEvents(t, result, []string{"", "", "", ""})

	var first dto.OpenAIChatCompletionChunk
	if err := sonic.Unmarshal(events[0].data, &first); err != nil {
		t.Fatalf("failed to unmarshal role chunk: %v", err)
	}
	if len(first.Choices) != 1 || first.Choices[0].Delta == nil || first.Choices[0].Delta.Role != enum.RoleAssistant {
		t.Fatalf("role chunk delta = %+v, want role=assistant", first.Choices[0])
	}
	var second dto.OpenAIChatCompletionChunk
	if err := sonic.Unmarshal(events[1].data, &second); err != nil {
		t.Fatalf("failed to unmarshal content chunk: %v", err)
	}
	if second.Choices[0].Delta.Content == nil || *second.Choices[0].Delta.Content != constant.TriggerContentFilterMessage {
		t.Fatalf("content chunk delta = %+v, want %q", second.Choices[0].Delta, constant.TriggerContentFilterMessage)
	}
	var third dto.OpenAIChatCompletionChunk
	if err := sonic.Unmarshal(events[2].data, &third); err != nil {
		t.Fatalf("failed to unmarshal finish chunk: %v", err)
	}
	if third.Choices[0].FinishReason == nil || *third.Choices[0].FinishReason != enum.FinishReasonContentFilter {
		t.Fatalf("finish chunk finish_reason = %v, want %q", third.Choices[0].FinishReason, enum.FinishReasonContentFilter)
	}
	if string(events[3].data) != constant.SSEDoneSignal {
		t.Fatalf("last frame = %q, want %q", string(events[3].data), constant.SSEDoneSignal)
	}
	if proxy.chatUnaryCalled || proxy.chatStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}

// TestOpenAICreateResponse_DenyTriggerStream
// /responses 流式命中 deny 词时返回 200 SSE 事件序列，completed 事件带 incomplete_details=content_filter。
func TestOpenAICreateResponse_DenyTriggerStream(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
		Input:  &dto.ResponseInput{Text: "敏感词"},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	events := readStreamEvents(t, result, []string{
		enum.ResponseStreamEventCreated,
		enum.ResponseStreamEventOutputItemAdded,
		enum.ResponseStreamEventContentPartAdded,
		enum.ResponseStreamEventContentPartDone,
		enum.ResponseStreamEventOutputItemDone,
		enum.ResponseStreamEventCompleted,
	})

	var created dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(events[0].data, &created); err != nil {
		t.Fatalf("failed to unmarshal created event: %v", err)
	}
	if created.Response == nil || created.Response.Status != constant.ResponseStreamFieldStatusCompleted {
		t.Fatalf("created response = %+v, want status=completed", created.Response)
	}
	if len(created.Response.Output) != 0 {
		t.Fatalf("created event output must be empty, got %d items", len(created.Response.Output))
	}

	var completed dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(events[5].data, &completed); err != nil {
		t.Fatalf("failed to unmarshal completed event: %v", err)
	}
	if completed.Response == nil || completed.Response.IncompleteDetails == nil || completed.Response.IncompleteDetails.Reason != "content_filter" {
		t.Fatalf("completed response = %+v, want incomplete_details.reason=content_filter", completed.Response)
	}
	if proxy.responseUnaryCalled || proxy.responseStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}

// TestAnthropicCreateMessage_DenyTriggerUnary
// Anthropic 非流式命中 deny 词时返回 200 refusal JSON（stop_reason=refusal），不触达上游。
func TestAnthropicCreateMessage_DenyTriggerUnary(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, trigger, nil)

	stream := false
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "这条消息包含敏感词内容"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", jsonResult.StatusCode)
	}
	if jsonResult.Protocol != enum.ProtocolKindAnthropic {
		t.Fatalf("protocol = %v, want %v", jsonResult.Protocol, enum.ProtocolKindAnthropic)
	}
	assertAnthropicContentFilterBody(t, jsonResult.Body)
	if mockProxy.messageUnaryCalled || mockProxy.messageStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}

// TestAnthropicCreateMessage_DenyTriggerStream
// Anthropic 流式命中 deny 词时返回 200 SSE 事件序列，message_delta 带 stop_reason=refusal。
func TestAnthropicCreateMessage_DenyTriggerStream(t *testing.T) {
	t.Parallel()
	mockProxy := &mockAnthropicProxyForAnthropic{}
	mockResolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildAnthropicTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewAnthropicUseCase(mockResolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, mockProxy, &mockOpenAIProxy{}, &mockTaskSubmitter{}, trigger, nil)

	stream := true
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "claude-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "敏感词"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	events := readStreamEvents(t, result, []string{
		enum.AnthropicSSEEventTypeMessageStart,
		enum.AnthropicSSEEventTypeContentBlockStart,
		enum.AnthropicSSEEventTypeContentBlockDelta,
		enum.AnthropicSSEEventTypeContentBlockStop,
		enum.AnthropicSSEEventTypeMessageDelta,
		enum.AnthropicSSEEventTypeMessageStop,
	})

	var start dto.AnthropicSSEMessageStart
	if err := sonic.Unmarshal(events[0].data, &start); err != nil {
		t.Fatalf("failed to unmarshal message_start: %v", err)
	}
	if start.Message == nil || start.Message.Role != enum.RoleAssistant {
		t.Fatalf("message_start message = %+v, want role=assistant", start.Message)
	}
	if len(start.Message.Content) != 0 {
		t.Fatalf("message_start content = %+v, want empty (text delivered via delta only)", start.Message.Content)
	}

	var blockStart dto.AnthropicSSEContentBlockStart
	if err := sonic.Unmarshal(events[1].data, &blockStart); err != nil {
		t.Fatalf("failed to unmarshal content_block_start: %v", err)
	}
	if blockStart.ContentBlock == nil || blockStart.ContentBlock.Text == nil || *blockStart.ContentBlock.Text != "" {
		t.Fatalf("content_block_start text = %+v, want empty (text delivered via delta only)", blockStart.ContentBlock)
	}

	var delta dto.AnthropicSSEContentBlockDelta
	if err := sonic.Unmarshal(events[2].data, &delta); err != nil {
		t.Fatalf("failed to unmarshal content_block_delta: %v", err)
	}
	if delta.Delta.Text != constant.TriggerContentFilterMessage {
		t.Fatalf("content_block_delta text = %q, want %q", delta.Delta.Text, constant.TriggerContentFilterMessage)
	}

	var msgDelta dto.AnthropicSSEMessageDelta
	if err := sonic.Unmarshal(events[4].data, &msgDelta); err != nil {
		t.Fatalf("failed to unmarshal message_delta: %v", err)
	}
	if msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != enum.AnthropicStopReasonRefusal {
		t.Fatalf("message_delta stop_reason = %v, want %q", msgDelta.Delta.StopReason, enum.AnthropicStopReasonRefusal)
	}
	if msgDelta.Delta.StopDetails == nil || msgDelta.Delta.StopDetails.Type != "refusal" {
		t.Fatalf("message_delta stop_details = %+v, want type=refusal", msgDelta.Delta.StopDetails)
	}
	if mockProxy.messageUnaryCalled || mockProxy.messageStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}
