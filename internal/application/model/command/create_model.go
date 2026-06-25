package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type createModelHandler struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
}

// NewCreateModelHandler 构造创建命令处理器
func NewCreateModelHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) port.CreateModelHandler {
	return &createModelHandler{endpointRepo: endpointRepo, modelRepo: modelRepo}
}

// Handle 执行创建命令
func (h *createModelHandler) Handle(ctx context.Context, cmd port.CreateModelCommand) (*port.CreateModelResult, error) {
	log := logger.WithCtx(ctx)

	// Verify endpoint exists
	ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID)
	if err != nil {
		log.Error("[ModelCommand] Find endpoint for model creation failed", zap.Error(err))
		return nil, err
	}
	if ep == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}

	m, err := aggregate.CreateModel(0, vo.EndpointAlias(cmd.Alias), cmd.ModelName, cmd.EndpointID, true)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "validate model")
	}

	id, err := h.modelRepo.Create(ctx, m)
	if err != nil {
		log.Error("[ModelCommand] Create model failed", zap.Error(err))
		return nil, err
	}

	log.Info("[ModelCommand] Create model success", zap.Uint("id", id))
	return &port.CreateModelResult{ModelID: id}, nil
}
