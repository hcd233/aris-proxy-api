package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// DeleteEndpointHandler 删除命令处理器
type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd port.DeleteEndpointCommand) error
}

type deleteEndpointHandler struct {
	endpointRepo llmproxy.EndpointRepository
}

func NewDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository) DeleteEndpointHandler {
	return &deleteEndpointHandler{endpointRepo: endpointRepo}
}

func (h *deleteEndpointHandler) Handle(ctx context.Context, cmd port.DeleteEndpointCommand) error {
	log := logger.WithCtx(ctx)

	ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID)
	if err != nil {
		return err
	}
	if ep == nil {
		return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}

	if err := h.endpointRepo.DeleteCascade(ctx, cmd.EndpointID); err != nil {
		log.Error("[EndpointCommand] Cascade delete endpoint failed", zap.Error(err))
		return err
	}

	log.Info("[EndpointCommand] Delete endpoint success", zap.Uint("id", cmd.EndpointID))
	return nil
}
