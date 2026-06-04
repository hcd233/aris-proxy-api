package usecase

import (
	"context"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func auditFailure(ctx context.Context, submitter TaskSubmitter, m *aggregate.Model, exposedModel, endpoint string, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	auditFailureWithProviders(ctx, submitter, m, exposedModel, endpoint, apiProtocol, apiProtocol, totalMs, err)
}

func auditFailureWithProviders(ctx context.Context, submitter TaskSubmitter, m *aggregate.Model, exposedModel, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             m.AggregateID(),
		Model:               exposedModel,
		Endpoint:            endpoint,
		UpstreamProtocol:    upstreamProtocol,
		APIProtocol:         apiProtocol,
		FirstTokenLatencyMs: totalMs,
	}
	task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
	_ = submitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit
}

func newAuditTask(ctx context.Context, m *aggregate.Model, exposedModel, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, firstTokenLatencyMs int64) *dto.ModelCallAuditTask {
	return &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             m.AggregateID(),
		Model:               exposedModel,
		Endpoint:            endpoint,
		UpstreamProtocol:    upstreamProtocol,
		APIProtocol:         apiProtocol,
		FirstTokenLatencyMs: firstTokenLatencyMs,
	}
}
