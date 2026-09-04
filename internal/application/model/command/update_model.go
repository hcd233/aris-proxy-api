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
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) (port.UpdateModelResult, error) {
	log := logger.WithCtx(ctx)

	m, err := h.repo.FindByID(ctx, cmd.ID, cmd.ScopeUserID)
	if err != nil {
		log.Error("[ModelCommand] Find model for update failed", zap.Error(err))
		return port.UpdateModelResult{}, err
	}
	if m == nil {
		return port.UpdateModelResult{}, ierr.New(ierr.ErrDataNotExists, "model not found")
	}

	if cmd.EndpointID != nil {
		ep, eerr := h.endpointRepo.FindByID(ctx, *cmd.EndpointID, cmd.ScopeUserID)
		if eerr != nil {
			log.Error("[ModelCommand] Find endpoint for model update failed", zap.Error(eerr))
			return port.UpdateModelResult{}, eerr
		}
		if ep == nil {
			return port.UpdateModelResult{}, ierr.New(ierr.ErrDataNotExists, "endpoint not found")
		}
		if ep.UserID() != m.UserID() {
			return port.UpdateModelResult{}, ierr.New(ierr.ErrNoPermission, "endpoint owner does not match model owner")
		}
	}

	var aliasPtr *vo.EndpointAlias
	if cmd.Alias != nil {
		a := vo.EndpointAlias(*cmd.Alias)
		aliasPtr = &a
	}

	oldModelID := m.ModelID() // 在领域更新前记录，供历史同步判定与替换

	if uerr := m.Update(aliasPtr, cmd.UpstreamModel, cmd.EndpointID, cmd.Enabled, cmd.ContextLength, cmd.MaxOutputTokens, cmd.Capabilities, cmd.ModelID); uerr != nil {
		return port.UpdateModelResult{}, uerr
	}
	if err := h.repo.Update(ctx, m); err != nil {
		log.Error("[ModelCommand] Update model failed", zap.Error(err))
		return port.UpdateModelResult{}, err
	}

	result := port.UpdateModelResult{}
	if cmd.SyncHistory != nil && *cmd.SyncHistory && cmd.ModelID != nil && *cmd.ModelID != oldModelID {
		// scope 取模型归属 user（m.UserID()，非操作者）：admin 代管时只作用于模型 owner 的数据。
		// 替换失败时模型本体已改名、历史未同步（spec §7）：返回错误，前端提示可重试。
		counts, serr := h.repo.ReplaceHistoricalModelID(ctx, m.UserID(), oldModelID, *cmd.ModelID)
		if serr != nil {
			log.Error("[ModelCommand] Replace historical model id failed",
				zap.Uint("id", cmd.ID), zap.Error(serr))
			return port.UpdateModelResult{}, serr
		}
		result = port.UpdateModelResult{
			AuditCount:   counts.AuditCount,
			SessionCount: counts.SessionCount,
			MessageCount: counts.MessageCount,
		}
		log.Info("[ModelCommand] Historical model id synced",
			zap.Uint("id", cmd.ID),
			zap.String("old", oldModelID), zap.String("new", *cmd.ModelID),
			zap.Int64("auditCount", result.AuditCount),
			zap.Int64("sessionCount", result.SessionCount),
			zap.Int64("messageCount", result.MessageCount))
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ID))
	return result, nil
}
