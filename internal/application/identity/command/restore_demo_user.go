package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type restoreDemoUserHandler struct {
	repo                identity.UserRepository
	invalidateUserCache func(ctx context.Context, userID uint)
}

// NewRestoreDemoUserHandler 构造
//
//	@param repo identity.UserRepository
//	@param invalidateUserCache 用户权限变更后清理 Redis JWT 用户缓存（可为 nil）
//	@return port.RestoreDemoUserHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewRestoreDemoUserHandler(repo identity.UserRepository, invalidateUserCache func(ctx context.Context, userID uint)) port.RestoreDemoUserHandler {
	return &restoreDemoUserHandler{repo: repo, invalidateUserCache: invalidateUserCache}
}

// Handle 恢复 Demo 账户为普通用户（仅允许 demo → user）
//
// 规则：
//
//   - 用户不存在 → ErrDataNotExists
//
//   - 目标权限非 demo → ErrValidation
//
//     @receiver h *restoreDemoUserHandler
//     @param ctx context.Context
//     @param cmd port.RestoreDemoUserCommand
//     @return error
//     @author centonhuang
//     @update 2026-08-16 10:00:00
func (h *restoreDemoUserHandler) Handle(ctx context.Context, cmd port.RestoreDemoUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for restore demo", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if user.Permission() != enum.PermissionDemo {
		log.Warn("[IdentityCommand] Restore demo rejected, target not demo",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "user %d is not demo", cmd.UserID)
	}

	user.ChangePermission(enum.PermissionUser)
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if h.invalidateUserCache != nil {
		h.invalidateUserCache(ctx, cmd.UserID)
	}
	log.Info("[IdentityCommand] Restore demo user to user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
