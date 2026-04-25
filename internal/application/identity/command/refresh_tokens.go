package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// RefreshTokensCommand 刷新 token 对命令
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type RefreshTokensCommand struct {
	RefreshToken string
}

// RefreshTokensHandler 刷新命令处理器
type RefreshTokensHandler interface {
	Handle(ctx context.Context, cmd RefreshTokensCommand) (*vo.TokenPair, error)
}

type refreshTokensHandler struct {
	repo     identity.UserRepository
	access   service.TokenSigner
	refresh  service.TokenSigner
	logLabel string
}

// NewRefreshTokensHandler 构造刷新处理器
//
//	@param repo identity.UserRepository
//	@param access service.TokenSigner
//	@param refresh service.TokenSigner
//	@return RefreshTokensHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewRefreshTokensHandler(repo identity.UserRepository, access, refresh service.TokenSigner) RefreshTokensHandler {
	return &refreshTokensHandler{repo: repo, access: access, refresh: refresh, logLabel: "[IdentityCommand]"}
}

// Handle 执行刷新：解析 refresh token → 校验用户存在 → 重新签发一对新 token
//
//	@receiver h *refreshTokensHandler
//	@param ctx context.Context
//	@param cmd RefreshTokensCommand
//	@return *vo.TokenPair
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (h *refreshTokensHandler) Handle(ctx context.Context, cmd RefreshTokensCommand) (*vo.TokenPair, error) {
	log := logger.WithCtx(ctx)

	userID, err := h.refresh.DecodeToken(cmd.RefreshToken)
	if err != nil {
		log.Error(h.logLabel+" Decode refresh token failed",
			zap.String("refreshToken", util.MaskSecret(cmd.RefreshToken)), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTDecode, err, "decode refresh token")
	}

	user, err := h.repo.FindByID(ctx, userID)
	if err != nil {
		log.Error(h.logLabel+" FindByID failed", zap.Error(err), zap.Uint("userID", userID))
		return nil, err
	}
	if user == nil {
		log.Warn(h.logLabel+" User not found during refresh", zap.Uint("userID", userID))
		return nil, ierr.New(ierr.ErrDataNotExists, "user not found")
	}

	access, err := h.access.EncodeToken(userID)
	if err != nil {
		log.Error(h.logLabel+" Encode access token failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode access token")
	}
	refresh, err := h.refresh.EncodeToken(userID)
	if err != nil {
		log.Error(h.logLabel+" Encode refresh token failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode refresh token")
	}

	log.Info(h.logLabel+" Refresh token success", zap.Uint("userID", userID))

	return &vo.TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}
