package llmproxy_usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// captureTaskSubmitter 记录提交任务的 mockTaskSubmitter，用于断言存储/审计行为。
type captureTaskSubmitter struct {
	auditTasks []*dto.ModelCallAuditTask
	storeTasks []*dto.MessageStoreTask
}

func (m *captureTaskSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	m.auditTasks = append(m.auditTasks, task)
	return nil
}

func (m *captureTaskSubmitter) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	m.storeTasks = append(m.storeTasks, task)
	return nil
}

// lastUserTextCaptureChecker 按文本真实匹配的 TriggerChecker fake：
// Check 对包含触发词的文本返回 capture 词命中。
type lastUserTextCaptureChecker struct {
	captureWord string
}

func (f *lastUserTextCaptureChecker) Check(text string) []uint {
	if strings.Contains(text, f.captureWord) {
		return []uint{7}
	}
	return nil
}

func (*lastUserTextCaptureChecker) MatchedWords(ids []uint) []string {
	return []string{"capture-word"}
}

func (*lastUserTextCaptureChecker) DenyIDs(ids []uint) []uint { return nil }

func (*lastUserTextCaptureChecker) CaptureIDs(ids []uint) []uint { return []uint{7} }

func (*lastUserTextCaptureChecker) IncrementHits(_ context.Context, _ []uint) error { return nil }

// denyAndCaptureChecker 同时命中 deny 词与 capture 词。
type denyAndCaptureChecker struct {
	captureWord string
	denyWord    string
}

func (f *denyAndCaptureChecker) Check(text string) []uint {
	if strings.Contains(text, f.captureWord) || strings.Contains(text, f.denyWord) {
		return []uint{1, 7}
	}
	return nil
}

func (*denyAndCaptureChecker) MatchedWords(ids []uint) []string {
	return []string{"deny-word", "capture-word"}
}

func (*denyAndCaptureChecker) DenyIDs(ids []uint) []uint { return []uint{1} }

func (*denyAndCaptureChecker) CaptureIDs(ids []uint) []uint { return []uint{7} }

func (*denyAndCaptureChecker) IncrementHits(_ context.Context, _ []uint) error { return nil }

// collectSink 收集 SSE 事件名与 text_delta 文本的 EventSink。
type collectSink struct {
	names    []string
	fullText string
}

func (s *collectSink) WriteEvent(event string, data []byte) error {
	s.names = append(s.names, event)
	if event == "content_block_delta" {
		var parsed struct {
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := sonic.Unmarshal(data, &parsed); err == nil {
			s.fullText += parsed.Delta.Text
		}
	}
	return nil
}

func (s *collectSink) hasEvent(name string) bool {
	return lo.Contains(s.names, name)
}

// ==================== Anthropic ====================

// capture 命中最后一条 user 消息且有历史时：保存历史（无 assistant 回复、无触发消息）、不触达上游、返回固定回复。
func TestAnthropicCreateMessage_CaptureWithHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, checker, nil)

	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "test-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "first question"}},
			{Role: "assistant", Content: &dto.AnthropicMessageContent{Text: "first answer"}},
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "please /save"}},
		},
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.messageUnaryCalled || proxy.messageStreamCalled {
		t.Fatal("upstream proxy must not be called on capture")
	}
	json, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("expect JSONResult, got %T", result)
	}
	if json.StatusCode != 200 {
		t.Fatalf("expect 200, got %d", json.StatusCode)
	}
	if !strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatalf("reply body should contain fixed reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 1 {
		t.Fatalf("expect 1 store task, got %d", len(submitter.storeTasks))
	}
	msgs := submitter.storeTasks[0].Messages
	if len(msgs) != 2 {
		t.Fatalf("expect 2 history messages (no trigger msg, no assistant reply), got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected roles: %s, %s", msgs[0].Role, msgs[1].Role)
	}
	if len(submitter.auditTasks) != 1 || !strings.Contains(submitter.auditTasks[0].ErrorMessage, "capture") {
		t.Fatalf("expect capture audit task, got %+v", submitter.auditTasks)
	}
}

// 触发消息为第一条（无历史）时：不提交存储，返回特殊回复。
func TestAnthropicCreateMessage_CaptureWithoutHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, checker, nil)

	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model:    "test-alias",
		Messages: []*dto.AnthropicMessageParam{{Role: "user", Content: &dto.AnthropicMessageContent{Text: "/save"}}},
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.messageUnaryCalled || proxy.messageStreamCalled {
		t.Fatal("upstream proxy must not be called on capture")
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureEmptyReply) {
		t.Fatalf("reply body should contain empty-history reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("expect no store task without history, got %d", len(submitter.storeTasks))
	}
}

// 触发词只出现在历史消息（非最后一条 user 提问）时：capture 不生效，正常转发。
func TestAnthropicCreateMessage_CaptureWordOnlyInHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, checker, nil)

	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "test-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "history mentions /save"}},
			{Role: "assistant", Content: &dto.AnthropicMessageContent{Text: "ok"}},
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "normal question"}},
		},
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxy.messageUnaryCalled {
		t.Fatal("capture word only in history should forward normally")
	}
	if _, isJSON := result.(*port.JSONResult); !isJSON {
		t.Fatalf("expect forwarded JSONResult, got %T", result)
	}
}

// deny 与 capture 同时命中时 deny 优先：返回 403 ProxyError，不保存。
func TestAnthropicCreateMessage_DenyWinsOverCapture(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	checker := &denyAndCaptureChecker{captureWord: "/save", denyWord: "badword"}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, checker, nil)

	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "test-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "first"}},
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "please /save badword"}},
		},
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
	json, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("expect JSONResult (deny content filter), got %T", result)
	}
	if json.StatusCode != 200 {
		t.Fatalf("deny content filter: expect 200, got %d", json.StatusCode)
	}
	if strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatal("deny must not return capture reply")
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatal("deny must not store context")
	}
}

// stream=true 时 capture 返回 StreamResult，事件序列完整且携带固定回复。
func TestAnthropicCreateMessage_CaptureStream(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, checker, nil)

	stream := true
	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "test-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "first"}},
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "please /save"}},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sr, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("expect StreamResult, got %T", result)
	}
	if proxy.messageStreamCalled {
		t.Fatal("upstream must not be called")
	}
	streamReader, err := sr.Open(context.Background())
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	events := &collectSink{}
	if err := streamReader.Read(context.Background(), events); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !events.hasEvent("message_start") || !events.hasEvent("message_stop") {
		t.Fatalf("incomplete anthropic SSE sequence: %v", events.names)
	}
	if !strings.Contains(events.fullText, constant.TriggerCaptureSavedReply) {
		t.Fatalf("stream should carry fixed reply: %q", events.fullText)
	}
}

// ==================== OpenAI Chat ====================

func TestOpenAICreateChatCompletion_CaptureWithHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "first question"}},
			{Role: "assistant", Content: &dto.OpenAIMessageContent{Text: "first answer"}},
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please /save"}},
		},
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.chatUnaryCalled {
		t.Fatal("upstream proxy must not be called on capture")
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatalf("reply body should contain fixed reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
}

// tool_call_id 非空的 user 消息（tool 结果回传）不是用户提问，不作为触发位置。
func TestOpenAICreateChatCompletion_CaptureSkipsToolResultUser(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please run the tool"}},
			{Role: "user", ToolCallID: lo.ToPtr("call_1"), Content: &dto.OpenAIMessageContent{Text: "/save"}},
		},
	}}

	_, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxy.chatUnaryCalled {
		t.Fatal("trigger word in tool_result user message should not capture; request forwards")
	}
}

// ==================== OpenAI Response ====================

func TestOpenAICreateResponse_CaptureWithHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("test-endpoint", true, true, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "first question"}},
				{Role: lo.ToPtr("assistant"), Content: &dto.ResponseInputMessageContent{Text: "first answer"}},
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "please /save"}},
			},
		},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.responseUnaryCalled {
		t.Fatal("upstream proxy must not be called on capture")
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatalf("reply body should contain fixed reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
}

// 字符串 input 即触发消息且无 Instructions：无历史，返回特殊回复。
func TestOpenAICreateResponse_CaptureStringInputNoHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("test-endpoint", true, true, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{Text: "/save"},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureEmptyReply) {
		t.Fatalf("reply body should contain empty-history reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatal("no store task expected")
	}
}

// stream=true 时 Response capture 返回完整 SSE 事件序列。
func TestOpenAICreateResponse_CaptureStream(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	checker := &lastUserTextCaptureChecker{captureWord: "/save"}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("test-endpoint", true, true, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)

	stream := true
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "first"}},
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "please /save"}},
			},
		},
		Stream: &stream,
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sr, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("expect StreamResult, got %T", result)
	}
	if proxy.responseStreamCalled {
		t.Fatal("upstream must not be called")
	}
	streamReader, err := sr.Open(context.Background())
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	events := &collectSink{}
	if err := streamReader.Read(context.Background(), events); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !events.hasEvent("response.created") || !events.hasEvent("response.completed") {
		t.Fatalf("incomplete response SSE sequence: %v", events.names)
	}
}
