package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type setDemoUserHandler struct {
	repo                identity.UserRepository
	invalidateUserCache func(ctx context.Context, userID uint)
}

// NewSetDemoUserHandler 构造
//
//	@param repo identity.UserRepository
//	@param invalidateUserCache 用户权限变更后清理 Redis JWT 用户缓存（可为 nil）
//	@return port.SetDemoUserHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewSetDemoUserHandler(repo identity.UserRepository, invalidateUserCache func(ctx context.Context, userID uint)) port.SetDemoUserHandler {
	return &setDemoUserHandler{repo: repo, invalidateUserCache: invalidateUserCache}
}

// Handle 设置全局单例 Demo 账户（替换语义）
//
// 规则：
//
//   - 用户不存在 → ErrDataNotExists
//
//   - 操作者即目标（禁止对自己操作）→ ErrValidation
//
//   - 目标权限非 pending/user（admin/已 demo 均拒绝）→ ErrValidation
//
//   - 已存在其他 Demo 用户时将其回退为 pending（替换），保证全局单例
//
//   - 两个受影响用户均失效 Redis JWT 用户缓存
//
//     @receiver h *setDemoUserHandler
//     @param ctx context.Context
//     @param cmd port.SetDemoUserCommand
//     @return error
//     @author centonhuang
//     @update 2026-08-16 10:00:00
func (h *setDemoUserHandler) Handle(ctx context.Context, cmd port.SetDemoUserCommand) error {
	log := logger.WithCtx(ctx)

	if cmd.OperatorID == cmd.UserID {
		log.Warn("[IdentityCommand] Set demo rejected, cannot operate self",
			zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrValidation, "cannot set demo on self")
	}

	previousDemoID, err := h.repo.ReplaceDemoUser(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] Replace demo user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if h.invalidateUserCache != nil {
		if previousDemoID != 0 {
			h.invalidateUserCache(ctx, previousDemoID)
		}
		h.invalidateUserCache(ctx, cmd.UserID)
	}
	log.Info("[IdentityCommand] Set demo user",
		zap.Uint("operatorID", cmd.OperatorID),
		zap.Uint("targetID", cmd.UserID),
		zap.Uint("previousDemoID", previousDemoID))
	return nil
}
