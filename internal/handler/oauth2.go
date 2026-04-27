// Package handler OAuth2处理器
package handler

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
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

// Oauth2Platforms OAuth2 平台映射（github/google 等）
type Oauth2Platforms map[string]oauth2service.Platform

// Oauth2Dependencies OAuth2Handler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type Oauth2Dependencies struct {
	Platforms     Oauth2Platforms
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner
	RefreshSigner identityservice.TokenSigner
	DirCreator    command.ObjectStorageDirCreator // 可选；nil 时跳过存储目录创建
}

type oauth2Handler struct {
	initiate command.InitiateLoginHandler
	callback command.HandleCallbackHandler
}

// NewOauth2Handler 创建OAuth2处理器
//
//	@param deps Oauth2Dependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return Oauth2Handler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewOauth2Handler(deps Oauth2Dependencies) Oauth2Handler {
	return &oauth2Handler{
		initiate: command.NewInitiateLoginHandler(deps.Platforms),
		callback: command.NewHandleCallbackHandler(
			deps.Platforms,
			deps.UserRepo,
			deps.AccessSigner,
			deps.RefreshSigner,
			deps.DirCreator,
		),
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
	rsp.AccessToken = result.TokenPair.AccessToken()
	rsp.RefreshToken = result.TokenPair.RefreshToken()
	return util.WrapHTTPResponse(rsp, nil)
}
