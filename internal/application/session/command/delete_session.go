package command

import (
	"context"
	"slices"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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

func (h *deleteSessionHandler) Handle(ctx context.Context, cmd port.DeleteSessionCommand) (*port.DeleteSessionResult, error) {
	log := logger.WithCtx(ctx)

	isAdmin := cmd.RequesterPermission == enum.PermissionAdmin
	var ownerNames []string
	if !isAdmin {
		names, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.RequesterID)
		if lookupErr != nil {
			log.Error("[SessionCommand] Delete: lookup owner names failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.RequesterID))
			return nil, lookupErr
		}
		ownerNames = names
	}

	result := &port.DeleteSessionResult{}

	for _, id := range cmd.SessionIDs {
		sess, err := h.repo.FindByID(ctx, id)
		if err != nil {
			log.Error("[SessionCommand] Delete: FindByID failed", zap.Error(err), zap.Uint("sessionID", id))
			result.Failures = append(result.Failures, port.DeleteSessionFailedItem{ID: id, Error: "failed to find session"})
			continue
		}
		if sess == nil {
			result.Failures = append(result.Failures, port.DeleteSessionFailedItem{ID: id, Error: "session not found"})
			continue
		}

		if !isAdmin {
			owner := sess.Owner()
			allowed := slices.Contains(ownerNames, owner.String())
			if !allowed {
				result.Failures = append(result.Failures, port.DeleteSessionFailedItem{ID: id, Error: "no permission"})
				continue
			}
		}

		if err := h.repo.Delete(ctx, id); err != nil {
			log.Error("[SessionCommand] Delete: delete failed", zap.Error(err), zap.Uint("sessionID", id))
			result.Failures = append(result.Failures, port.DeleteSessionFailedItem{ID: id, Error: "failed to delete"})
			continue
		}

		result.DeletedCount++
		log.Info("[SessionCommand] Session deleted",
			zap.Uint("sessionID", id),
			zap.Uint("requesterID", cmd.RequesterID),
			zap.String("owner", sess.Owner().String()))
	}

	log.Info("[SessionCommand] Delete completed",
		zap.Int("total", len(cmd.SessionIDs)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.Failures)),
		zap.Uint("requesterID", cmd.RequesterID))

	return result, nil
}
