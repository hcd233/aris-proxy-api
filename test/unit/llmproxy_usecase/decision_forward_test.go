package llmproxy_usecase

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestMarshalOpenAIDecisionBodyForModel_RewritesModelOnly(t *testing.T) {
	t.Parallel()

	name := "damaged"
	req := &dto.OpenAICreateDecisionReq{
		Model: "exposed-alias",
		Input: dto.DecisionInput{Text: lo.ToPtr("The package arrived with a broken screen.")},
		Questions: []*dto.DecisionQuestion{{
			Type:         "predicate",
			Name:         &name,
			Instructions: "Does the customer report a damaged item?",
		}},
	}

	body := proxyutil.MarshalOpenAIDecisionBodyForModel(req, "gpt-upstream-real")

	var got map[string]any
	if err := sonic.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["model"] != "gpt-upstream-real" {
		t.Fatalf("model = %v, want gpt-upstream-real", got["model"])
	}
	if got["input"] != "The package arrived with a broken screen." {
		t.Fatalf("input = %v, want the original text", got["input"])
	}
	questions, ok := got["questions"].([]any)
	if !ok || len(questions) != 1 {
		t.Fatalf("questions = %v, want 1 item", got["questions"])
	}
	if req.Model != "exposed-alias" {
		t.Fatalf("original request mutated: model = %s", req.Model)
	}
}

// decisionTaskSubmitter 捕获 Decision 路径提交的审计/存储任务。
type decisionTaskSubmitter struct {
	auditTasks []*dto.ModelCallAuditTask
	storeTasks []*dto.MessageStoreTask
}

func (s *decisionTaskSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	s.auditTasks = append(s.auditTasks, task)
	return nil
}

func (s *decisionTaskSubmitter) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	s.storeTasks = append(s.storeTasks, task)
	return nil
}

var _ usecase.TaskSubmitter = (*decisionTaskSubmitter)(nil)

func buildDecisionEndpoint() *aggregate.Endpoint {
	ep, _ := aggregate.CreateEndpoint(5, "decision-endpoint", "https://api.openai.com", "", "sk-test", false, false, false, true)
	return ep
}

func decisionResponse(answers string) []byte {
	return []byte(`{"model":"gpt-upstream-real","answers":` + answers + `,"usage":{"input_tokens":42,"output_tokens":7,"total_tokens":49,"input_tokens_details":{"cached_tokens":12,"cache_write_tokens":5},"output_tokens_details":{"reasoning_tokens":3}}}`)
}

func newDecisionRequest() *dto.OpenAICreateDecisionRequest {
	name := "damaged"
	return &dto.OpenAICreateDecisionRequest{Body: &dto.OpenAICreateDecisionReq{
		Model: "test-alias",
		Input: dto.DecisionInput{Text: lo.ToPtr("The package arrived with a broken screen.")},
		Questions: []*dto.DecisionQuestion{{
			Type:         "predicate",
			Name:         &name,
			Instructions: "Does the customer report a damaged item?",
		}},
	}}
}

func newDecisionUseCase(proxy *mockOpenAIProxy, submitter *decisionTaskSubmitter) port.OpenAIUseCase {
	resolver := &mockResolver{resolveEndpoint: buildDecisionEndpoint(), resolveModel: buildTestModel()}
	return usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil, nil)
}

func TestCreateDecision_NativeForwardRewritesModelAndStoresSession(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.95}]`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	result, err := uc.CreateDecision(t.Context(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", jsonResult.StatusCode)
	}

	var upstreamBody map[string]any
	if err := sonic.Unmarshal(proxy.lastDecisionBody, &upstreamBody); err != nil {
		t.Fatalf("unmarshal upstream body: %v", err)
	}
	if upstreamBody["model"] != "test-model" {
		t.Fatalf("upstream model = %v, want test-model (upstream_model)", upstreamBody["model"])
	}

	var exposed map[string]any
	if err := sonic.Unmarshal(jsonResult.Body, &exposed); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	if exposed["model"] != "test-alias" {
		t.Fatalf("exposed model = %v, want test-alias", exposed["model"])
	}
	answers, ok := exposed["answers"].([]any)
	if !ok || len(answers) != 1 {
		t.Fatalf("answers = %v, want 1 item", exposed["answers"])
	}

	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
	audit := submitter.auditTasks[0]
	if audit.APIProtocol != enum.ProtocolOpenAIDecision || audit.UpstreamProtocol != enum.ProtocolOpenAIDecision {
		t.Fatalf("audit protocols = %s/%s, want openai-decision", audit.APIProtocol, audit.UpstreamProtocol)
	}
	if audit.UpstreamStatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", audit.UpstreamStatusCode)
	}
	if audit.InputTokens != 25 || audit.OutputTokens != 7 || audit.CacheReadInputTokens != 12 || audit.CacheCreationInputTokens != 5 {
		t.Fatalf("audit tokens = %+v, want net 25/7/12/5", audit)
	}

	if len(submitter.storeTasks) != 1 {
		t.Fatalf("store tasks = %d, want 1", len(submitter.storeTasks))
	}
	store := submitter.storeTasks[0]
	if len(store.Messages) != 2 {
		t.Fatalf("stored messages = %d, want 2 (user + assistant)", len(store.Messages))
	}
	if store.Messages[0].Content.Text != "The package arrived with a broken screen." {
		t.Fatalf("user message = %q", store.Messages[0].Content.Text)
	}
	var storedAnswers []map[string]any
	if err := sonic.UnmarshalString(store.Messages[1].Content.Text, &storedAnswers); err != nil {
		t.Fatalf("stored answers must be JSON: %v (raw=%q)", err, store.Messages[1].Content.Text)
	}
	if len(storedAnswers) != 1 || storedAnswers[0]["type"] != "predicate" || storedAnswers[0]["name"] != "damaged" {
		t.Fatalf("stored answers = %+v", storedAnswers)
	}
	if store.InputTokens != 42 || store.OutputTokens != 7 {
		t.Fatalf("store tokens = %d/%d, want 42/7", store.InputTokens, store.OutputTokens)
	}
}

func TestCreateDecision_ImageInputStoredAsParts(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.9}]`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	detail := "high"
	imageURL := "https://example.com/x.png"
	req := &dto.OpenAICreateDecisionRequest{Body: &dto.OpenAICreateDecisionReq{
		Model: "test-alias",
		Input: dto.DecisionInput{Messages: []*dto.DecisionInputMessage{{
			Role: "user",
			Content: dto.ResponseInputMessageContent{Parts: []*dto.ResponseInputContent{
				{Type: "input_text", Text: lo.ToPtr("damaged?")},
				{Type: "input_image", ImageURL: &imageURL, Detail: &detail},
			}},
		}}},
		Questions: []*dto.DecisionQuestion{{Type: "predicate", Instructions: "damaged?"}},
	}}

	if _, err := uc.CreateDecision(t.Context(), req); err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	if len(submitter.storeTasks) != 1 {
		t.Fatalf("store tasks = %d, want 1", len(submitter.storeTasks))
	}
	parts := submitter.storeTasks[0].Messages[0].Content.Parts
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if parts[1].Type != enum.ContentPartTypeImageURL || parts[1].ImageURL != imageURL || parts[1].ImageDetail != detail {
		t.Fatalf("image part = %+v", parts[1])
	}
}

func TestCreateDecision_ModelNotSupported(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{}
	submitter := &decisionTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-only", true, false, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil, nil)

	_, err := uc.CreateDecision(t.Context(), newDecisionRequest())
	if err == nil {
		t.Fatal("CreateDecision() must fail when endpoint does not support decisions")
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("error = %T, want *port.ProxyError", err)
	}
	if proxy.decisionUnaryCalled {
		t.Fatal("upstream must not be called when model is unsupported")
	}
}

func TestCreateDecision_UpstreamErrorPassthrough(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionErr: &model.UpstreamError{
		StatusCode: http.StatusTooManyRequests,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       `{"error":{"message":"rate limited"}}`,
	}}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	_, err := uc.CreateDecision(t.Context(), newDecisionRequest())
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("error = %T, want *port.ProxyError", err)
	}
	if proxyErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", proxyErr.StatusCode)
	}
	if string(proxyErr.Body) != `{"error":{"message":"rate limited"}}` {
		t.Fatalf("body = %s, want upstream body", proxyErr.Body)
	}
	if len(submitter.auditTasks) != 1 || submitter.auditTasks[0].UpstreamStatusCode != http.StatusTooManyRequests {
		t.Fatalf("audit = %+v, want 429 failure audit", submitter.auditTasks)
	}
}

func TestCreateDecision_UnparsableBodyStillPassesThrough(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: []byte(`not-json`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	result, err := uc.CreateDecision(t.Context(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if string(jsonResult.Body) != "not-json" {
		t.Fatalf("body = %s, want passthrough", jsonResult.Body)
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0", len(submitter.storeTasks))
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
}

// stubTriggerChecker 以固定命中结果驱动 Decision 拦截分支。
type stubTriggerChecker struct {
	matched []uint
	words   []string
	denyIDs []uint
	omitIDs []uint
}

func (s *stubTriggerChecker) Check(string) []uint { return s.matched }

func (s *stubTriggerChecker) MatchedWords([]uint) []string { return s.words }

func (s *stubTriggerChecker) DenyIDs([]uint) []uint { return s.denyIDs }

func (s *stubTriggerChecker) OmitIDs([]uint) []uint { return s.omitIDs }

func (s *stubTriggerChecker) CaptureIDs([]uint) []uint { return nil }

func (s *stubTriggerChecker) IncrementHits(context.Context, []uint) error { return nil }

var _ usecase.TriggerChecker = (*stubTriggerChecker)(nil)

func newInterceptDecisionUseCase(proxy *mockOpenAIProxy, submitter *decisionTaskSubmitter, checker *stubTriggerChecker) port.OpenAIUseCase {
	resolver := &mockResolver{resolveEndpoint: buildDecisionEndpoint(), resolveModel: buildTestModel()}
	return usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil, nil)
}

func TestCreateDecision_DenyReturnsRefusalPerQuestion(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{}
	submitter := &decisionTaskSubmitter{}
	checker := &stubTriggerChecker{matched: []uint{1}, words: []string{"blocked"}, denyIDs: []uint{1}}
	uc := newInterceptDecisionUseCase(proxy, submitter, checker)

	result, err := uc.CreateDecision(t.Context(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if proxy.decisionUnaryCalled {
		t.Fatal("upstream must not be called on deny")
	}

	var body struct {
		Model   string `json:"model"`
		Answers []struct {
			Type string  `json:"type"`
			Name *string `json:"name"`
		} `json:"answers"`
		Usage map[string]any `json:"usage"`
	}
	if err := sonic.Unmarshal(jsonResult.Body, &body); err != nil {
		t.Fatalf("unmarshal refusal body: %v", err)
	}
	if body.Model != "test-alias" {
		t.Fatalf("model = %s, want test-alias", body.Model)
	}
	if len(body.Answers) != 1 || body.Answers[0].Type != "refusal" {
		t.Fatalf("answers = %+v, want 1 refusal", body.Answers)
	}
	if body.Answers[0].Name == nil || *body.Answers[0].Name != "damaged" {
		t.Fatalf("refusal name = %v, want damaged", body.Answers[0].Name)
	}
	if body.Usage == nil {
		t.Fatal("usage must be present")
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
	if submitter.auditTasks[0].ErrorMessage == "" {
		t.Fatal("deny audit must carry trigger word remark")
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0", len(submitter.storeTasks))
	}
}

func TestCreateDecision_OmitSkipsSessionStore(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.9}]`)}
	submitter := &decisionTaskSubmitter{}
	checker := &stubTriggerChecker{matched: []uint{2}, words: []string{"private"}, omitIDs: []uint{2}}
	uc := newInterceptDecisionUseCase(proxy, submitter, checker)

	if _, err := uc.CreateDecision(t.Context(), newDecisionRequest()); err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	if !proxy.decisionUnaryCalled {
		t.Fatal("upstream must be called when only omit matches")
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0 (omit)", len(submitter.storeTasks))
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
}
