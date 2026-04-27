// Package handler 令牌处理器
package handler

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TokenHandler 令牌处理器
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type TokenHandler interface {
	HandleRefreshToken(ctx context.Context, req *dto.RefreshTokenReq) (*dto.HTTPResponse[*dto.RefreshTokenRsp], error)
}

// TokenDependencies TokenHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type TokenDependencies struct {
	Refresh command.RefreshTokensHandler
}

type tokenHandler struct {
	refresh command.RefreshTokensHandler
}

// NewTokenHandler 创建令牌处理器
//
//	@param deps TokenDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return TokenHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewTokenHandler(deps TokenDependencies) TokenHandler {
	return &tokenHandler{
		refresh: deps.Refresh,
	}
}

// HandleRefreshToken 刷新令牌
//
//	@receiver h *tokenHandler
//	@param ctx context.Context
//	@param req *dto.RefreshTokenReq
//	@return *dto.HTTPResponse[*dto.RefreshTokenRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *tokenHandler) HandleRefreshToken(ctx context.Context, req *dto.RefreshTokenReq) (*dto.HTTPResponse[*dto.RefreshTokenRsp], error) {
	rsp := &dto.RefreshTokenRsp{}

	if strings.TrimSpace(req.Body.RefreshToken) == "" {
		rsp.Error = ierr.ErrValidation.BizError()
		return util.WrapHTTPResponse(rsp, nil)
	}

	pair, err := h.refresh.Handle(ctx, command.RefreshTokensCommand{
		RefreshToken: req.Body.RefreshToken,
	})
	if err != nil {
		logger.WithCtx(ctx).Warn("[TokenHandler] Refresh token failed",
			zap.String("refreshToken", util.MaskSecret(req.Body.RefreshToken)),
			zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.AccessToken = pair.AccessToken()
	rsp.RefreshToken = pair.RefreshToken()
	return util.WrapHTTPResponse(rsp, nil)
}
