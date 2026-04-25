package usecase

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// auditFailure 提交失败态审计任务（非流式错误分支共享）
//
//	@param ctx context.Context
//	@param ep *aggregate.Endpoint
//	@param exposedModel string
//	@param apiProvider enum.ProviderType
//	@param totalMs int64
//	@param err error
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func auditFailure(ctx context.Context, ep *aggregate.Endpoint, exposedModel string, apiProvider enum.ProviderType, totalMs int64, err error) {
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             ep.AggregateID(),
		Model:               exposedModel,
		UpstreamProvider:    ep.Provider(),
		APIProvider:         apiProvider,
		FirstTokenLatencyMs: totalMs,
	}
	task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
	_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
}
