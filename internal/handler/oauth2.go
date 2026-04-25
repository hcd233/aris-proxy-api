// Package handler OAuth2处理器
package handler

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// Oauth2Handler OAuth2处理器接口
//
//	@author centonhuang
//	@update 2025-01-05 21:00:00
type Oauth2Handler interface {
	HandleLogin(ctx context.Context, req *dto.LoginReq) (*dto.HTTPResponse[*dto.LoginResp], error)
	HandleCallback(ctx context.Context, req *dto.CallbackReq) (*dto.HTTPResponse[*dto.CallbackRsp], error)
}

type oauth2Handler struct {
	initiate command.InitiateLoginHandler
	callback command.HandleCallbackHandler
}

// NewOauth2Handler 创建OAuth2处理器
//
//	@return Oauth2Handler
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func NewOauth2Handler() Oauth2Handler {
	platforms := map[string]service.Platform{
		constant.OAuthProviderGithub: oauth2.NewGithubPlatform(),
		constant.OAuthProviderGoogle: oauth2.NewGooglePlatform(),
	}

	userRepo := repository.NewUserRepository()
	accessSigner := jwt.GetAccessTokenSigner()
	refreshSigner := jwt.GetRefreshTokenSigner()
	dirCreator := repository.NewAudioDirCreator()

	return &oauth2Handler{
		initiate: command.NewInitiateLoginHandler(platforms),
		callback: command.NewHandleCallbackHandler(platforms, userRepo, accessSigner, refreshSigner, dirCreator),
	}
}

// HandleLogin OAuth2登录
//
//	@receiver h *oauth2Handler
//	@param ctx context.Context
//	@param req *dto.LoginReq
//	@return *dto.HTTPResponse[*dto.LoginResp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *oauth2Handler) HandleLogin(ctx context.Context, req *dto.LoginReq) (*dto.HTTPResponse[*dto.LoginResp], error) {
	rsp := &dto.LoginResp{}
	result, err := h.initiate.Handle(ctx, command.InitiateLoginCommand{Platform: req.Platform})
	if err != nil {
		logger.WithCtx(ctx).Error("[OAuth2Handler] Initiate login failed",
			zap.String("platform", req.Platform), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}
	rsp.RedirectURL = result.RedirectURL
	return util.WrapHTTPResponse(rsp, nil)
}

// HandleCallback OAuth2回调
//
//	@receiver h *oauth2Handler
//	@param ctx context.Context
//	@param req *dto.CallbackReq
//	@return *dto.HTTPResponse[*dto.CallbackRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *oauth2Handler) HandleCallback(ctx context.Context, req *dto.CallbackReq) (*dto.HTTPResponse[*dto.CallbackRsp], error) {
	rsp := &dto.CallbackRsp{}
	result, err := h.callback.Handle(ctx, command.HandleCallbackCommand{
		Platform: req.Body.Platform,
		Code:     req.Body.Code,
		State:    req.Body.State,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[OAuth2Handler] Callback failed",
			zap.String("platform", req.Body.Platform), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}
	rsp.AccessToken = result.TokenPair.AccessToken
	rsp.RefreshToken = result.TokenPair.RefreshToken
	return util.WrapHTTPResponse(rsp, nil)
}
