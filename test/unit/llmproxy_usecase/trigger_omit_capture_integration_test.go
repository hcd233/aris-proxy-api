package llmproxy_usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/application/trigger"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// realTriggerRepo 供真实 TriggerService 使用的词条仓库 fake。
type realTriggerRepo struct {
	items []*aggregate.Trigger
}

func (f *realTriggerRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}

func (f *realTriggerRepo) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
	return 0, nil
}

func (f *realTriggerRepo) Delete(ctx context.Context, id uint) error { return nil }

func (f *realTriggerRepo) DeleteBatch(ctx context.Context, ids []uint) error { return nil }

func (f *realTriggerRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *realTriggerRepo) Paginate(ctx context.Context, p model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *realTriggerRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	return f.items, nil
}

func (f *realTriggerRepo) BatchIncrementHitCount(ctx context.Context, m map[uint]uint) error {
	return nil
}

var _ triggerdomain.TriggerRepository = (*realTriggerRepo)(nil)

// newRealTriggerService 用真实 AC matcher 构建 TriggerService（omit 词 + capture 词）。
func newRealTriggerService(t *testing.T) *trigger.TriggerService {
	t.Helper()
	omit, _ := aggregate.CreateTrigger(5, "ignore", enum.TriggerActionOmit)
	capture, _ := aggregate.CreateTrigger(7, "/save", enum.TriggerActionCapture)
	svc := trigger.NewTriggerService(&realTriggerRepo{items: []*aggregate.Trigger{omit, capture}}, nil, nil)
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return svc
}

// TestRealService_OmitAndCapture_LastMessage 历史含 omit 词、最后一条 user 消息含 capture 词：
// capture 必须生效（短路 + 保存历史），不能只走 omit。
func TestRealService_OmitAndCapture_LastMessage(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please ignore this"}},
			{Role: "assistant", Content: &dto.OpenAIMessageContent{Text: "ok"}},
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please /save this conversation"}},
		},
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.chatUnaryCalled {
		t.Fatal("upstream proxy must not be called: capture should short-circuit over omit")
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatalf("reply body should contain fixed reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
}

// TestRealService_OmitAndCapture_SameMessage omit 词与 capture 词同在最后一条 user 消息：
// capture 生效。
func TestRealService_OmitAndCapture_SameMessage(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "first question"}},
			{Role: "assistant", Content: &dto.OpenAIMessageContent{Text: "first answer"}},
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please ignore and /save this"}},
		},
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.chatUnaryCalled {
		t.Fatal("upstream proxy must not be called: capture should short-circuit over omit")
	}
	json, _ := result.(*port.JSONResult)
	if !strings.Contains(string(json.Body), constant.TriggerCaptureSavedReply) {
		t.Fatalf("reply body should contain fixed reply: %s", json.Body)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
}

// TestRealService_OmitAndCapture_CaptureInHistory 回归：capture 词在历史、omit 词在最后一条
// user 消息（capture 未短路）时，capture 的上下文保存必须照常执行——保存最后一条 user 消息
// 之前的历史，同时 omit 的跳过存储照常生效、请求照常转发（两个逻辑都跑，不短路）。
func TestRealService_OmitAndCapture_CaptureInHistory(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please /save the context"}},
			{Role: "assistant", Content: &dto.OpenAIMessageContent{Text: "saved"}},
			{Role: "user", Content: &dto.OpenAIMessageContent{Text: "please ignore that"}},
		},
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxy.chatUnaryCalled {
		t.Fatal("capture word in history should not short-circuit; request forwards")
	}
	if _, isJSON := result.(*port.JSONResult); !isJSON {
		t.Fatalf("expect forwarded JSONResult, got %T", result)
	}
	// capture 保存：最后一条 user 消息（omit 词所在）之前的历史
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
	if msgs := submitter.storeTasks[0].Messages; msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected history roles: %+v", msgs)
	}
	// capture 审计照常提交
	if !lo.ContainsBy(submitter.auditTasks, func(t *dto.ModelCallAuditTask) bool {
		return strings.Contains(t.ErrorMessage, "capture")
	}) {
		t.Fatalf("expect capture audit task among %+v", submitter.auditTasks)
	}
}

// TestRealService_OmitAndCapture_Anthropic anthropic 入口同场景：capture 词在历史 + omit 词
// 在最后一条 user 消息 → 保存历史 + 转发（不短路）。
func TestRealService_OmitAndCapture_Anthropic(t *testing.T) {
	t.Parallel()
	proxy := &mockAnthropicProxyForAnthropic{}
	submitter := &captureTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildAnthropicTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewAnthropicUseCase(resolver, &mockAnthropicListModels{}, &mockAnthropicCountTokens{}, proxy, &mockOpenAIProxy{}, submitter, newRealTriggerService(t), nil)

	req := &dto.AnthropicCreateMessageRequest{Body: &dto.AnthropicCreateMessageReq{
		Model: "test-alias",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "please /save the context"}},
			{Role: "assistant", Content: &dto.AnthropicMessageContent{Text: "saved"}},
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "please ignore that"}},
		},
	}}

	result, err := uc.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxy.messageUnaryCalled {
		t.Fatal("capture word in history should not short-circuit; request forwards")
	}
	if _, isJSON := result.(*port.JSONResult); !isJSON {
		t.Fatalf("expect forwarded JSONResult, got %T", result)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
	if !lo.ContainsBy(submitter.auditTasks, func(t *dto.ModelCallAuditTask) bool {
		return strings.Contains(t.ErrorMessage, "capture")
	}) {
		t.Fatalf("expect capture audit task among %+v", submitter.auditTasks)
	}
}

// TestRealService_OmitAndCapture_Response Response 入口同场景：capture 词在历史 + omit 词
// 在最后一条 user item → 保存历史 + 转发（不短路）。
func TestRealService_OmitAndCapture_Response(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	submitter := &captureTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("test-endpoint", true, true, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, newRealTriggerService(t), nil)

	req := &dto.OpenAICreateResponseRequest{Body: &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-alias"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "please /save the context"}},
				{Role: lo.ToPtr("assistant"), Content: &dto.ResponseInputMessageContent{Text: "saved"}},
				{Role: lo.ToPtr("user"), Content: &dto.ResponseInputMessageContent{Text: "please ignore that"}},
			},
		},
	}}

	result, err := uc.CreateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxy.responseUnaryCalled {
		t.Fatal("capture word in history should not short-circuit; request forwards")
	}
	if _, isJSON := result.(*port.JSONResult); !isJSON {
		t.Fatalf("expect forwarded JSONResult, got %T", result)
	}
	if len(submitter.storeTasks) != 1 || len(submitter.storeTasks[0].Messages) != 2 {
		t.Fatalf("expect 1 store task with 2 history messages, got %+v", submitter.storeTasks)
	}
	if !lo.ContainsBy(submitter.auditTasks, func(t *dto.ModelCallAuditTask) bool {
		return strings.Contains(t.ErrorMessage, "capture")
	}) {
		t.Fatalf("expect capture audit task among %+v", submitter.auditTasks)
	}
}
