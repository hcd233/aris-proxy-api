package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// DeleteEndpointCommand 删除 Endpoint 命令
type DeleteEndpointCommand struct {
	EndpointID uint
}

// DeleteEndpointHandler 删除命令处理器
type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd DeleteEndpointCommand) error
}

type deleteEndpointHandler struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
}

func NewDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) DeleteEndpointHandler {
	return &deleteEndpointHandler{endpointRepo: endpointRepo, modelRepo: modelRepo}
}

func (h *deleteEndpointHandler) Handle(ctx context.Context, cmd DeleteEndpointCommand) error {
	log := logger.WithCtx(ctx)

	ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID)
	if err != nil {
		return err
	}
	if ep == nil {
		return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}

	models, err := h.modelRepo.List(ctx)
	if err != nil {
		log.Error("[EndpointCommand] Check model references failed", zap.Error(err))
		return err
	}
	for _, m := range models {
		if m.EndpointID() == cmd.EndpointID {
			return ierr.New(ierr.ErrValidation, "endpoint is still referenced by models, delete models first")
		}
	}

	if err := h.endpointRepo.Delete(ctx, cmd.EndpointID); err != nil {
		log.Error("[EndpointCommand] Delete endpoint failed", zap.Error(err))
		return err
	}

	log.Info("[EndpointCommand] Delete endpoint success", zap.Uint("id", cmd.EndpointID))
	return nil
}
