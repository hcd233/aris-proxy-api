package usecase

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ratelimit"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// auditFailure 记录非流式失败调用的审计（上下游协议一致的简化入口）。
func auditFailure(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, exposedModel, endpoint string, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	auditFailureWithProviders(ctx, m, submitter, exposedModel, endpoint, apiProtocol, apiProtocol, totalMs, err)
}

// auditFailureWithProviders 记录非流式失败调用的审计（上下游协议可不同）。
func auditFailureWithProviders(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, exposedModel, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	recordModelCall(ctx, submitter, callOutcome{
		model:               m,
		exposedModel:        exposedModel,
		endpoint:            endpoint,
		upstreamProtocol:    upstreamProtocol,
		apiProtocol:         apiProtocol,
		firstTokenLatencyMs: totalMs,
		err:                 err,
	})
}

// reportTokenUsage 从 context 取出 TokenUsageReporter 并上报实际 token 用量。
func reportTokenUsage(ctx context.Context, tokens int64) {
	if tokens <= 0 {
		return
	}
	reporter, ok := ctx.Value(constant.CtxKeyTokenUsageReporter).(ratelimit.TokenUsageReporter)
	if !ok || reporter == nil {
		return
	}
	reporter.Report(ctx, tokens)
}
