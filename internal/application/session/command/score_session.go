package command

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	sessionvo "github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// NewScoreSessionHandler 构造评分命令处理器
func NewScoreSessionHandler(repo session.SessionRepository) port.ScoreSessionHandler {
	return &scoreSessionHandler{repo: repo}
}

type scoreSessionHandler struct {
	repo session.SessionRepository
}

func (h *scoreSessionHandler) Handle(ctx context.Context, cmd port.ScoreSessionCommand) (*time.Time, error) {
	log := logger.WithCtx(ctx)

	sv, err := sessionvo.NewSessionScore(cmd.Score, time.Now())
	if err != nil {
		return nil, err
	}

	if err := h.repo.UpdateScore(ctx, cmd.SessionID, sv); err != nil {
		log.Error("[SessionCommand] UpdateScore failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return nil, err
	}

	return sv.At(), nil
}

// NewDeleteScoreSessionHandler 构造删除评分命令处理器
func NewDeleteScoreSessionHandler(repo session.SessionRepository) port.DeleteScoreSessionHandler {
	return &deleteScoreSessionHandler{repo: repo}
}

type deleteScoreSessionHandler struct {
	repo session.SessionRepository
}

func (h *deleteScoreSessionHandler) Handle(ctx context.Context, cmd port.DeleteScoreSessionCommand) error {
	log := logger.WithCtx(ctx)

	if err := h.repo.DeleteScore(ctx, cmd.SessionID); err != nil {
		log.Error("[SessionCommand] DeleteScore failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return err
	}

	return nil
}
