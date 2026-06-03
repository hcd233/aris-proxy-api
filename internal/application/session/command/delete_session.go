package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// DeleteSessionCommand 删除 Session 命令
//
//	@author centonhuang
//	@update 2026-06-03 10:00:00
type DeleteSessionCommand struct {
	// SessionID 要删除的 Session ID
	SessionID uint
	// RequesterID 执行删除操作的用户 ID（来自 ctx）
	RequesterID uint
	// RequesterPermission 执行者权限
	RequesterPermission enum.Permission
}

// DeleteSessionHandler 删除命令处理器
//
//	@author centonhuang
//	@update 2026-06-03 10:00:00
type DeleteSessionHandler interface {
	Handle(ctx context.Context, cmd DeleteSessionCommand) error
}

type deleteSessionHandler struct {
	repo       session.SessionRepository
	apiKeyRepo apikey.APIKeyRepository
}

// NewDeleteSessionHandler 构造删除命令处理器
//
//	@param repo session.SessionRepository
//	@param apiKeyRepo apikey.APIKeyRepository
//	@return DeleteSessionHandler
//	@author centonhuang
//	@update 2026-06-03 10:00:00
func NewDeleteSessionHandler(repo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) DeleteSessionHandler {
	return &deleteSessionHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

// Handle 执行删除
//
//   - Session 不存在 → ErrDataNotExists
//
//   - 非 admin 且 session 不属于该用户 → ErrNoPermission
//
//   - 通过校验 → Delete
//
//     @receiver h *deleteSessionHandler
//     @param ctx context.Context
//     @param cmd DeleteSessionCommand
//     @return error
//     @author centonhuang
//     @update 2026-06-03 10:00:00
func (h *deleteSessionHandler) Handle(ctx context.Context, cmd DeleteSessionCommand) error {
	log := logger.WithCtx(ctx)

	sess, err := h.repo.FindByID(ctx, cmd.SessionID)
	if err != nil {
		log.Error("[SessionCommand] FindByID failed", zap.Error(err), zap.Uint("sessionID", cmd.SessionID))
		return err
	}
	if sess == nil {
		log.Warn("[SessionCommand] Session not found", zap.Uint("sessionID", cmd.SessionID))
		return ierr.New(ierr.ErrDataNotExists, "session not found")
	}

	if cmd.RequesterPermission != enum.PermissionAdmin {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.RequesterID)
		if lookupErr != nil {
			log.Error("[SessionCommand] LookupOwnerNamesByUserID failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.RequesterID))
			return lookupErr
		}
		owner := sess.Owner()
		allowed := false
		for _, name := range ownerNames {
			if owner.String() == name {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn("[SessionCommand] No permission to delete session",
				zap.Uint("sessionID", cmd.SessionID),
				zap.String("owner", owner.String()),
				zap.Uint("userID", cmd.RequesterID))
			return ierr.New(ierr.ErrNoPermission, "no permission to delete session")
		}
	}

	if err := h.repo.Delete(ctx, cmd.SessionID); err != nil {
		log.Error("[SessionCommand] Delete session failed", zap.Error(err), zap.Uint("sessionID", cmd.SessionID))
		return err
	}

	log.Info("[SessionCommand] Session deleted",
		zap.Uint("sessionID", cmd.SessionID),
		zap.Uint("requesterID", cmd.RequesterID),
		zap.String("owner", sess.Owner().String()))

	return nil
}
