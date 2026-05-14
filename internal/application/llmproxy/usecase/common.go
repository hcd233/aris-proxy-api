package usecase

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func auditFailure(submitter TaskSubmitter, ctx context.Context, m *aggregate.Model, exposedModel string, apiProvider enum.ProviderType, totalMs int64, err error) {
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             m.AggregateID(),
		Model:               exposedModel,
		UpstreamProvider:    apiProvider,
		APIProvider:         apiProvider,
		FirstTokenLatencyMs: totalMs,
	}
	task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
	_ = submitter.SubmitModelCallAuditTask(task)
}
