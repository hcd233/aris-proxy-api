// Package command Identity 域命令处理器
package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UpdateProfileCommand 更新用户档案命令
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type UpdateProfileCommand struct {
	UserID uint
	Name   string
	Email  string
	Avatar string
}

// UpdateProfileHandler 更新档案命令处理器
type UpdateProfileHandler interface {
	Handle(ctx context.Context, cmd UpdateProfileCommand) error
}

type updateProfileHandler struct {
	repo identity.UserRepository
}

// NewUpdateProfileHandler 构造
//
//	@param repo identity.UserRepository
//	@return UpdateProfileHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewUpdateProfileHandler(repo identity.UserRepository) UpdateProfileHandler {
	return &updateProfileHandler{repo: repo}
}

// Handle 执行档案更新
//
// 规则：
//
//   - 用户不存在 → ErrDataNotExists（原行为是"静默成功"，现改为显式报错，
//     避免前端误认为更新成功）
//
//   - Name 为空 → ErrValidation（与 RegisterUser 对齐）
//
//     @receiver h *updateProfileHandler
//     @param ctx context.Context
//     @param cmd UpdateProfileCommand
//     @return error
//     @author centonhuang
//     @update 2026-04-23 10:50:00
func (h *updateProfileHandler) Handle(ctx context.Context, cmd UpdateProfileCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("userID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for profile update", zap.Uint("userID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if err := user.UpdateProfile(vo.UserName(cmd.Name), vo.Email(cmd.Email), vo.Avatar(cmd.Avatar)); err != nil {
		log.Warn("[IdentityCommand] UpdateProfile validation failed", zap.Error(err), zap.Uint("userID", cmd.UserID))
		return err
	}
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err))
		return err
	}
	return nil
}
