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
	repo         llmproxy.ModelRepository
	endpointRepo llmproxy.EndpointRepository
}

// NewDeleteModelHandler 构造删除命令处理器
func NewDeleteModelHandler(repo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository) port.DeleteModelHandler {
	return &deleteModelHandler{repo: repo, endpointRepo: endpointRepo}
}

// Handle 执行删除命令
func (h *deleteModelHandler) Handle(ctx context.Context, cmd port.DeleteModelCommand) error {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ModelID, cmd.ScopeUserID)
	if err != nil {
		return err
	}
	if m == nil {
		return ierr.New(ierr.ErrDataNotExists, "model not found")
	}

	// 降级保护：公开端点下的模型只允许 admin 删除。
	if !cmd.IsAdmin {
		ep, eerr := h.endpointRepo.FindByID(ctx, m.EndpointID(), cmd.ScopeUserID)
		if eerr != nil {
			return eerr
		}
		if ep != nil && ep.Visibility().IsPublic() {
			return ierr.New(ierr.ErrUnauthorized, "only admin can modify models of a public endpoint")
		}
	}

	if err := h.repo.Delete(ctx, cmd.ModelID, cmd.ScopeUserID); err != nil {
		log.Error("[ModelCommand] Delete model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Delete model success", zap.Uint("id", cmd.ModelID))
	return nil
}
