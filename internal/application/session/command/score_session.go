package command

import (
	"context"
	"slices"
	"time"

	"github.com/samber/mo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	sessionvo "github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// NewScoreSessionHandler 构造评分命令处理器
func NewScoreSessionHandler(repo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) port.ScoreSessionHandler {
	return &scoreSessionHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

type scoreSessionHandler struct {
	repo       session.SessionRepository
	apiKeyRepo apikey.APIKeyRepository
}

func (h *scoreSessionHandler) Handle(ctx context.Context, cmd port.ScoreSessionCommand) (*time.Time, error) {
	log := logger.WithCtx(ctx)

	sessResult := h.repo.FindByID(ctx, cmd.SessionID)
	if sessResult.IsError() {
		log.Error("[SessionCommand] Score: FindByID failed", zap.Error(sessResult.Error()), zap.Uint("sessionID", cmd.SessionID))
		return nil, sessResult.Error()
	}
	sess := sessResult.MustGet()

	if cmd.RequesterPermission != enum.PermissionAdmin {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.RequesterID)
		if lookupErr != nil {
			log.Error("[SessionCommand] Score: LookupOwnerNamesByUserID failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.RequesterID))
			return nil, lookupErr
		}
		owner := sess.Owner()
		if !slices.Contains(ownerNames, owner.String()) {
			log.Warn("[SessionCommand] Score: No permission",
				zap.Uint("sessionID", cmd.SessionID),
				zap.String("owner", owner.String()),
				zap.Uint("userID", cmd.RequesterID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to score session")
		}
	}

	sv, err := sessionvo.NewSessionScore(cmd.Score, time.Now())
	if err != nil {
		return nil, err
	}

	if err := h.repo.UpdateScore(ctx, cmd.SessionID, mo.Some(sv)); err != nil {
		log.Error("[SessionCommand] UpdateScore failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return nil, err
	}

	at := sv.At()
	return &at, nil
}

// NewDeleteScoreSessionHandler 构造删除评分命令处理器
func NewDeleteScoreSessionHandler(repo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) port.DeleteScoreSessionHandler {
	return &deleteScoreSessionHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

type deleteScoreSessionHandler struct {
	repo       session.SessionRepository
	apiKeyRepo apikey.APIKeyRepository
}

func (h *deleteScoreSessionHandler) Handle(ctx context.Context, cmd port.DeleteScoreSessionCommand) error {
	log := logger.WithCtx(ctx)

	sessResult := h.repo.FindByID(ctx, cmd.SessionID)
	if sessResult.IsError() {
		log.Error("[SessionCommand] DeleteScore: FindByID failed", zap.Error(sessResult.Error()), zap.Uint("sessionID", cmd.SessionID))
		return sessResult.Error()
	}
	sess := sessResult.MustGet()

	if cmd.RequesterPermission != enum.PermissionAdmin {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.RequesterID)
		if lookupErr != nil {
			log.Error("[SessionCommand] DeleteScore: LookupOwnerNamesByUserID failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.RequesterID))
			return lookupErr
		}
		owner := sess.Owner()
		if !slices.Contains(ownerNames, owner.String()) {
			log.Warn("[SessionCommand] DeleteScore: No permission",
				zap.Uint("sessionID", cmd.SessionID),
				zap.String("owner", owner.String()),
				zap.Uint("userID", cmd.RequesterID))
			return ierr.New(ierr.ErrNoPermission, "no permission to delete session score")
		}
	}

	if err := h.repo.DeleteScore(ctx, cmd.SessionID); err != nil {
		log.Error("[SessionCommand] DeleteScore failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return err
	}

	return nil
}
