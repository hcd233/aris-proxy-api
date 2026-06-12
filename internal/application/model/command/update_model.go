package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UpdateModelHandler 更新命令处理器
type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd port.UpdateModelCommand) error
}

type updateModelHandler struct {
	repo llmproxy.ModelRepository
}

// NewUpdateModelHandler 构造更新命令处理器
func NewUpdateModelHandler(repo llmproxy.ModelRepository) UpdateModelHandler {
	return &updateModelHandler{repo: repo}
}

// Handle 执行更新命令
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) error {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ModelID)
	if err != nil {
		log.Error("[ModelCommand] Find model for update failed", zap.Error(err))
		return err
	}
	if m == nil {
		return ierr.New(ierr.ErrDataNotExists, "model not found")
	}

	var aliasPtr *vo.EndpointAlias
	if cmd.Alias != nil {
		a := vo.EndpointAlias(*cmd.Alias)
		aliasPtr = &a
	}

	m.Update(aliasPtr, cmd.ModelName, cmd.EndpointID, cmd.Enabled)

	if err := h.repo.Update(ctx, m); err != nil {
		log.Error("[ModelCommand] Update model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ModelID))
	return nil
}
