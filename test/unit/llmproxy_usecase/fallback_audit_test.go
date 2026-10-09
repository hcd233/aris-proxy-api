package llmproxy_usecase

import (
	"context"
	"sync"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// recordingTaskSubmitter 记录提交的审计任务，用于断言 fallback 的审计口径。
type recordingTaskSubmitter struct {
	mu     sync.Mutex
	audits []*dto.ModelCallAuditTask
}

func (s *recordingTaskSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, task)
	return nil
}

func (s *recordingTaskSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error { return nil }

var _ usecase.TaskSubmitter = (*recordingTaskSubmitter)(nil)

// multiCandidateResolver 按固定顺序返回多个候选（模拟 priority 已排好序）。
type multiCandidateResolver struct {
	cands []service.Candidate
}

func (r *multiCandidateResolver) ResolveCandidates(_ context.Context, _ uint, _ vo.EndpointAlias, _ func(*aggregate.Endpoint) bool) ([]service.Candidate, error) {
	return r.cands, nil
}

func (r *multiCandidateResolver) ResolveCandidatesWithAffinity(ctx context.Context, userID uint, alias vo.EndpointAlias, _ string, matcher func(*aggregate.Endpoint) bool) ([]service.Candidate, error) {
	return r.ResolveCandidates(ctx, userID, alias, matcher)
}

// sequencedDecisionProxy 第 i 次 Decision 转发返回 errs[i]（nil=成功）。
type sequencedDecisionProxy struct {
	mockOpenAIProxy
	errs  []error
	calls int
}

func (p *sequencedDecisionProxy) ForwardCreateDecision(_ context.Context, _ vo.UpstreamEndpoint, _ []byte) ([]byte, error) {
	i := p.calls
	p.calls++
	if i < len(p.errs) && p.errs[i] != nil {
		return nil, p.errs[i]
	}
	return []byte(`{"model":"m","answers":[],"usage":{"input_tokens":1,"output_tokens":1}}`), nil
}

func decisionCandidate(t *testing.T, id uint, name string) service.Candidate {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(id, name, "https://api.openai.com", "", "sk-test", false, false, false, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	m, err := aggregate.CreateModel(id, "decision-alias", "upstream-model", id, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	return service.Candidate{Endpoint: ep, Model: m}
}

// TestFallbackAuditsOnlyFinalOutcome spec §5：被切换掉的中间失败只记日志，审计只记终局结果。
func TestFallbackAuditsOnlyFinalOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		errs         []error
		wantEndpoint string
		wantSuccess  bool
	}{
		{"首端点 502 次端点成功", []error{&model.UpstreamError{StatusCode: 502}}, "ep-b", true},
		{"全部 502 只记最后一次", []error{&model.UpstreamError{StatusCode: 502}, &model.UpstreamError{StatusCode: 502}}, "ep-b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			submitter := &recordingTaskSubmitter{}
			proxy := &sequencedDecisionProxy{errs: tc.errs}
			resolver := &multiCandidateResolver{cands: []service.Candidate{decisionCandidate(t, 1, "ep-a"), decisionCandidate(t, 2, "ep-b")}}
			uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil, nil)

			_, err := uc.CreateDecision(context.Background(), newDecisionRequest())
			if (err == nil) != tc.wantSuccess {
				t.Fatalf("err = %v, wantSuccess %v", err, tc.wantSuccess)
			}
			if proxy.calls != 2 {
				t.Fatalf("上游调用次数 = %d, want 2（应切换到次端点）", proxy.calls)
			}
			if len(submitter.audits) != 1 {
				t.Fatalf("审计条数 = %d, want 1（中间失败不应入审计）", len(submitter.audits))
			}
			if got := submitter.audits[0].Endpoint; got != tc.wantEndpoint {
				t.Fatalf("审计端点 = %q, want %q", got, tc.wantEndpoint)
			}
		})
	}
}

// TestFallbackStopsWhenRequestCanceled 客户端断开/服务 drain 后不再逐个尝试剩余候选。
func TestFallbackStopsWhenRequestCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	submitter := &recordingTaskSubmitter{}
	proxy := &sequencedDecisionProxy{errs: []error{
		&model.UpstreamConnectionError{Cause: ierr.New(ierr.ErrProxyRequest, "context canceled")},
		nil,
	}}
	resolver := &multiCandidateResolver{cands: []service.Candidate{decisionCandidate(t, 1, "ep-a"), decisionCandidate(t, 2, "ep-b")}}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil, nil)

	if _, err := uc.CreateDecision(ctx, newDecisionRequest()); err == nil {
		t.Fatal("请求已取消时应返回错误")
	}
	if proxy.calls != 1 {
		t.Fatalf("上游调用次数 = %d, want 1（取消后不应切换端点）", proxy.calls)
	}
	if len(submitter.audits) != 1 {
		t.Fatalf("审计条数 = %d, want 1（终局失败仍需入审计）", len(submitter.audits))
	}
}
