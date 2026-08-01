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

type updateModelHandler struct {
	repo llmproxy.ModelRepository
}

// NewUpdateModelHandler 构造更新命令处理器
func NewUpdateModelHandler(repo llmproxy.ModelRepository) port.UpdateModelHandler {
	return &updateModelHandler{repo: repo}
}

// Handle 执行更新命令
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) error {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ID)
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

	if uerr := m.Update(aliasPtr, cmd.ModelName, cmd.EndpointID, cmd.Enabled, cmd.ContextLength, cmd.MaxOutputTokens, cmd.Capabilities, cmd.ModelID); uerr != nil {
		return uerr
	}

	if err := h.repo.Update(ctx, m); err != nil {
		log.Error("[ModelCommand] Update model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ID))
	return nil
}
