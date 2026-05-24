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
	repo llmproxy.EndpointRepository
}

// NewDeleteEndpointHandler 构造删除命令处理器
func NewDeleteEndpointHandler(repo llmproxy.EndpointRepository) DeleteEndpointHandler {
	return &deleteEndpointHandler{repo: repo}
}

// Handle 执行删除命令
func (h *deleteEndpointHandler) Handle(ctx context.Context, cmd DeleteEndpointCommand) error {
	log := logger.WithCtx(ctx)

	ep, err := h.repo.FindByID(ctx, cmd.EndpointID)
	if err != nil {
		return err
	}
	if ep == nil {
		return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}

	if err := h.repo.Delete(ctx, cmd.EndpointID); err != nil {
		log.Error("[EndpointCommand] Delete endpoint failed", zap.Error(err))
		return err
	}

	log.Info("[EndpointCommand] Delete endpoint success", zap.Uint("id", cmd.EndpointID))
	return nil
}
