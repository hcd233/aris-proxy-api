package llmproxy_usecase

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	metricspkg "github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// TestRecordModelCall_ReportsTokenUsageToCounter 验证非流式 chat 调用收尾时
// recordModelCall 把 usage 的 input/output token 累加进 TokenUsageCounter（Request TPS 数据源）。
func TestRecordModelCall_ReportsTokenUsageToCounter(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	tokenMetrics := metricspkg.NewTokenUsageCounter(registry)

	proxy := &mockOpenAIProxy{} // ForwardChatCompletion 返回 Usage{PromptTokens: 1, CompletionTokens: 1}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil, tokenMetrics)

	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model: "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}},
		},
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if _, ok := result.(*port.JSONResult); !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if snap.TokenInput != 1 {
		t.Errorf("expected tokenInput 1, got %f", snap.TokenInput)
	}
	if snap.TokenOutput != 1 {
		t.Errorf("expected tokenOutput 1, got %f", snap.TokenOutput)
	}
}
