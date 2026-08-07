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

type approveUserHandler struct {
	repo identity.UserRepository
}

// NewApproveUserHandler 构造
//
//	@param repo identity.UserRepository
//	@return ApproveUserHandler
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func NewApproveUserHandler(repo identity.UserRepository) port.ApproveUserHandler {
	return &approveUserHandler{repo: repo}
}

// Handle 执行用户审核：仅允许 pending → user
//
// 规则：
//
//   - 用户不存在 → ErrDataNotExists
//   - 目标权限非 pending（重复批准/操作 user/admin）→ ErrValidation
//   - 变更通过领域方法 ChangePermission + Save 持久化
//
// @receiver h *approveUserHandler
// @param ctx context.Context
// @param cmd ApproveUserCommand
// @return error
// @author centonhuang
// @update 2026-08-07 10:00:00
func (h *approveUserHandler) Handle(ctx context.Context, cmd port.ApproveUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for approve", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if user.Permission() != enum.PermissionPending {
		log.Warn("[IdentityCommand] Approve rejected, target not pending",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "user %d is not pending", cmd.UserID)
	}

	user.ChangePermission(enum.PermissionUser)
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	log.Info("[IdentityCommand] Approve user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
