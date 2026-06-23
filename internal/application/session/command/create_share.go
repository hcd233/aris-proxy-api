// Package command Session 应用层命令处理器
package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// NewCreateShareHandler 构造创建分享命令处理器
func NewCreateShareHandler(getByUser port.GetSessionByUserHandler, shareCache port.ShareCreator) port.CreateShareHandler {
	return &createShareHandler{
		getByUser:  getByUser,
		shareCache: shareCache,
	}
}

type createShareHandler struct {
	getByUser  port.GetSessionByUserHandler
	shareCache port.ShareCreator
}

func (h *createShareHandler) Handle(ctx context.Context, cmd port.CreateShareCommand) (*port.CreateShareResult, error) {
	log := logger.WithCtx(ctx)

	view, err := h.getByUser.Handle(ctx, port.GetSessionByUserQuery{
		UserID:    cmd.RequesterID,
		IsAdmin:   cmd.RequesterPermission.Level() >= enum.PermissionAdmin.Level(),
		SessionID: cmd.SessionID,
	})
	if err != nil {
		log.Error("[SessionCommand] CreateShare: verify session failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return nil, err
	}
	if view == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
	}

	ttl, err := util.ParseExpiresIn(cmd.ExpiresIn, cmd.ExpiresAt)
	if err != nil {
		log.Warn("[SessionCommand] CreateShare: invalid expiration",
			zap.String("expiresIn", cmd.ExpiresIn), zap.Error(err))
		return nil, err
	}

	shareID, expiresAt, err := h.shareCache.CreateShare(ctx, cmd.RequesterID, cmd.SessionID, ttl)
	if err != nil {
		log.Error("[SessionCommand] CreateShare: create share failed",
			zap.Uint("sessionID", cmd.SessionID), zap.Error(err))
		return nil, err
	}

	log.Info("[SessionCommand] Share created",
		zap.String("shareID", shareID),
		zap.Uint("sessionID", cmd.SessionID),
		zap.Uint("userID", cmd.RequesterID))

	return &port.CreateShareResult{
		ShareID:   shareID,
		ExpiresAt: expiresAt,
	}, nil
}

// Compile-time interface check
var _ port.CreateShareHandler = (*createShareHandler)(nil)
