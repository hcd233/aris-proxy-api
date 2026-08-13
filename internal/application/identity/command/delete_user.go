// Package command Identity 域命令处理器
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

type deleteUserHandler struct {
	repo                identity.UserRepository
	invalidateUserCache func(ctx context.Context, userID uint)
}

// NewDeleteUserHandler 构造
//
//	@param repo identity.UserRepository
//	@param invalidateUserCache 用户删除后清理 Redis JWT 用户缓存（可为 nil）
//	@return DeleteUserHandler
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func NewDeleteUserHandler(repo identity.UserRepository, invalidateUserCache func(ctx context.Context, userID uint)) port.DeleteUserHandler {
	return &deleteUserHandler{repo: repo, invalidateUserCache: invalidateUserCache}
}

// Handle 执行用户删除（软删除 + 级联撤销 API Keys）
//
// 规则：
//   - 用户不存在 → ErrDataNotExists
//   - 操作者即目标（禁止自删）→ ErrValidation
//   - 目标为 admin → ErrValidation（admin 只读）
//   - 通过校验 → DeleteCascade（事务内软删用户 + 批量软删 API Keys）
//
// @receiver h *deleteUserHandler
// @param ctx context.Context
// @param cmd DeleteUserCommand
// @return error
// @author centonhuang
// @update 2026-08-08 10:00:00
func (h *deleteUserHandler) Handle(ctx context.Context, cmd port.DeleteUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for delete", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if cmd.OperatorID == cmd.UserID {
		log.Warn("[IdentityCommand] Delete rejected, cannot delete self", zap.Uint("operatorID", cmd.OperatorID))
		return ierr.New(ierr.ErrValidation, "cannot delete self")
	}
	if user.Permission() == enum.PermissionAdmin {
		log.Warn("[IdentityCommand] Delete rejected, target is admin",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "cannot delete admin user %d", cmd.UserID)
	}

	if err := h.repo.DeleteCascade(ctx, cmd.UserID); err != nil {
		log.Error("[IdentityCommand] DeleteCascade failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	// 用户已删除：失效 Redis JWT 用户缓存，避免 TTL 内继续以已删用户身份访问
	if h.invalidateUserCache != nil {
		h.invalidateUserCache(ctx, cmd.UserID)
	}
	log.Info("[IdentityCommand] Delete user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
