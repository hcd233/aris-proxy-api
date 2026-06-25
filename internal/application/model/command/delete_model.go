package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type deleteModelHandler struct {
	repo llmproxy.ModelRepository
}

// NewDeleteModelHandler 构造删除命令处理器
func NewDeleteModelHandler(repo llmproxy.ModelRepository) port.DeleteModelHandler {
	return &deleteModelHandler{repo: repo}
}

// Handle 执行删除命令
func (h *deleteModelHandler) Handle(ctx context.Context, cmd port.DeleteModelCommand) error {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ModelID)
	if err != nil {
		return err
	}
	if m == nil {
		return ierr.New(ierr.ErrDataNotExists, "model not found")
	}

	if err := h.repo.Delete(ctx, cmd.ModelID); err != nil {
		log.Error("[ModelCommand] Delete model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Delete model success", zap.Uint("id", cmd.ModelID))
	return nil
}
