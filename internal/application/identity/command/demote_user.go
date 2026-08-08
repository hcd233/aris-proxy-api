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

type demoteUserHandler struct {
	repo identity.UserRepository
}

// NewDemoteUserHandler 构造
//
//	@param repo identity.UserRepository
//	@return DemoteUserHandler
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func NewDemoteUserHandler(repo identity.UserRepository) port.DemoteUserHandler {
	return &demoteUserHandler{repo: repo}
}

// Handle 执行用户降级：仅允许 user → pending
//
// 规则：
//   - 用户不存在 → ErrDataNotExists
//   - 操作者即目标（禁止自降级）→ ErrValidation
//   - 目标权限非 user（admin/pending 均拒绝）→ ErrValidation
//   - 变更通过领域方法 ChangePermission + Save 持久化
//
// @receiver h *demoteUserHandler
// @param ctx context.Context
// @param cmd DemoteUserCommand
// @return error
// @author centonhuang
// @update 2026-08-08 10:00:00
func (h *demoteUserHandler) Handle(ctx context.Context, cmd port.DemoteUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for demote", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if cmd.OperatorID == cmd.UserID {
		log.Warn("[IdentityCommand] Demote rejected, cannot demote self",
			zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrValidation, "cannot demote self")
	}
	if user.Permission() != enum.PermissionUser {
		log.Warn("[IdentityCommand] Demote rejected, target not user",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "user %d is not regular user", cmd.UserID)
	}

	user.ChangePermission(enum.PermissionPending)
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	log.Info("[IdentityCommand] Demote user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
