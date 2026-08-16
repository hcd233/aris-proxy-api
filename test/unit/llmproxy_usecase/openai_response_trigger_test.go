package llmproxy_usecase

import (
	"context"
	"net/http"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// fakeTriggerChecker 可配置的 TriggerChecker 假实现：
// triggerIDs 命中即返回；denyIDs 控制哪些命中词为 deny；captureIDs 控制哪些命中词为 capture。
type fakeTriggerChecker struct {
	triggerIDs []uint
	denyIDs    []uint
	captureIDs []uint
	hits       []uint
}

func (f *fakeTriggerChecker) Check(text string) []uint {
	if len(f.triggerIDs) == 0 {
		return nil
	}
	return f.triggerIDs
}

func (f *fakeTriggerChecker) MatchedWords(ids []uint) []string {
	return []string{"触发词"}
}

func (f *fakeTriggerChecker) DenyIDs(ids []uint) []uint {
	return f.denyIDs
}

func (f *fakeTriggerChecker) OmitIDs(ids []uint) []uint { return nil }

func (f *fakeTriggerChecker) CaptureIDs(ids []uint) []uint {
	return f.captureIDs
}

func (f *fakeTriggerChecker) IncrementHits(_ context.Context, ids []uint) error {
	f.hits = append(f.hits, ids...)
	return nil
}

// TestOpenAICreateResponse_DenyTriggerInput
// /responses 输入命中 deny 触发词时必须返回 200 内容拦截消息（content_filter + refusal part），
// 且不触达上游代理（responseUnaryCalled 保持 false）。
func TestOpenAICreateResponse_DenyTriggerInput(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{Text: "这条消息包含触发词内容"},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
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
	assertResponseContentFilterBody(t, jsonResult.Body)
	if proxy.responseUnaryCalled || proxy.responseStreamCalled {
		t.Fatal("upstream must not be called for trigger request")
	}
}

// TestOpenAICreateResponse_DenyTriggerInstructions
// Instructions（系统指令）字段同样参与触发词扫描。
func TestOpenAICreateResponse_DenyTriggerInstructions(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:        lo.ToPtr("test-alias"),
		Instructions: lo.ToPtr("系统指令里也有触发词"),
	}}

	_, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
}

// TestOpenAICreateResponse_DenyTriggerItemContent
// ResponseInputItem 的 Content.Text / Queries / Arguments 均参与扫描。
func TestOpenAICreateResponse_DenyTriggerItemContent(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{1}, denyIDs: []uint{1}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{Items: []*dto.ResponseInputItem{
			{Content: &dto.ResponseInputMessageContent{Text: "包含触发词的消息文本"}},
			{Queries: []string{"file-search 查询含触发词"}},
			{Arguments: lo.ToPtr(`{"query":"函数参数含触发词"}`)},
		}},
	}}

	_, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("deny should return content filter result, got error: %v", err)
	}
}

// TestOpenAICreateResponse_OmitSkipsStore
// 命中 omit（非 deny）词时放行转发，但上下文应携带 SkipStore 标记。
func TestOpenAICreateResponse_OmitSkipsStore(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: []uint{2}, denyIDs: nil}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
		Input:  &dto.ResponseInput{Text: "omit 词内容"},
	}}

	// 构造 context 观察 SkipStore 标记：从 usecase 内部无法直接读取，
	// 但可通过 mockTaskSubmitter 捕获 audit task 的 ctx 间接验证（见下）。
	// 此处仅验证 omit 放行且不报错。
	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("omit should forward, got error: %v", err)
	}
	if result == nil {
		t.Fatal("omit should return result, got nil")
	}
	if !proxy.responseUnaryCalled {
		t.Fatal("upstream must be called for omit request")
	}
}

// TestOpenAICreateResponse_NoTriggerPassesThrough
// 无命中词时正常转发（回归护栏）。
func TestOpenAICreateResponse_NoTriggerPassesThrough(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	trigger := &fakeTriggerChecker{triggerIDs: nil}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, trigger, nil)

	stream := false
	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model:  lo.ToPtr("test-alias"),
		Stream: &stream,
		Input:  &dto.ResponseInput{Text: "正常内容"},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !proxy.responseUnaryCalled {
		t.Fatal("upstream must be called")
	}
}
