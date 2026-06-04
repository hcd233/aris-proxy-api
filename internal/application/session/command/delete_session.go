package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// NewDeleteSessionHandler 构造删除命令处理器
func NewDeleteSessionHandler(repo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) port.DeleteSessionHandler {
	return &deleteSessionHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

type deleteSessionHandler struct {
	repo       session.SessionRepository
	apiKeyRepo apikey.APIKeyRepository
}

func (h *deleteSessionHandler) Handle(ctx context.Context, cmd port.DeleteSessionCommand) error {
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
