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
	endpointRepo llmproxy.EndpointRepository
	repo         llmproxy.ModelRepository
}

// NewUpdateModelHandler 构造更新命令处理器
func NewUpdateModelHandler(endpointRepo llmproxy.EndpointRepository, repo llmproxy.ModelRepository) port.UpdateModelHandler {
	return &updateModelHandler{endpointRepo: endpointRepo, repo: repo}
}

// Handle 执行更新命令
//
// 换绑 endpoint（cmd.EndpointID 非 nil）时必须校验目标 endpoint 归属：
// 用户 A 把 model 挂到用户 B 的 endpoint 上会让 model 出现在 B 的 upstream
// 分组视图里（ListByEndpointIDs 不做二次 scope 过滤），形成跨租户信息泄露。
// admin 全量视角下同样要求 endpoint 归属与 model 一致，防止误操作造出跨用户挂载。
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) error {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ID, cmd.ScopeUserID)
	if err != nil {
		log.Error("[ModelCommand] Find model for update failed", zap.Error(err))
		return err
	}
	if m == nil {
		return ierr.New(ierr.ErrDataNotExists, "model not found")
	}

	if cmd.EndpointID != nil {
		ep, eerr := h.endpointRepo.FindByID(ctx, *cmd.EndpointID, cmd.ScopeUserID)
		if eerr != nil {
			log.Error("[ModelCommand] Find endpoint for model update failed", zap.Error(eerr))
			return eerr
		}
		if ep == nil {
			return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
		}
		if ep.UserID() != m.UserID() {
			return ierr.New(ierr.ErrNoPermission, "endpoint owner does not match model owner")
		}
	}

	var aliasPtr *vo.EndpointAlias
	if cmd.Alias != nil {
		a := vo.EndpointAlias(*cmd.Alias)
		aliasPtr = &a
	}

	if uerr := m.Update(aliasPtr, cmd.UpstreamModel, cmd.EndpointID, cmd.Enabled, cmd.ContextLength, cmd.MaxOutputTokens, cmd.Capabilities, cmd.ModelID); uerr != nil {
		return uerr
	}

	if err := h.repo.Update(ctx, m); err != nil {
		log.Error("[ModelCommand] Update model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ID))
	return nil
}
