package handler

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// Oauth2Handler OAuth2处理器接口
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type Oauth2Handler interface {
	HandleLogin(ctx context.Context, req *dto.LoginReq) (*dto.HTTPResponse[*dto.LoginResp], error)
	HandleCallback(ctx context.Context, req *dto.CallbackReq) (*dto.HTTPResponse[*dto.CallbackRsp], error)
}

type oauth2Handler struct{}

// NewOauth2Handler 创建OAuth2处理器
//
//	return Oauth2Handler
//	author centonhuang
//	update 2025-01-05 21:00:00
func NewOauth2Handler() Oauth2Handler {
	return &oauth2Handler{}
}

// HandleLogin OAuth2登录
//
//	@receiver h *oauth2Handler
//	@param ctx context.Context
//	@param req *dto.LoginReq
//	@return *dto.HTTPResponse[*dto.LoginResp]
//	@return error
//	@author centonhuang
//	@update 2025-11-11 04:57:58
func (h *oauth2Handler) HandleLogin(ctx context.Context, req *dto.LoginReq) (*dto.HTTPResponse[*dto.LoginResp], error) {
	return util.WrapHTTPResponse(h.getService(req.Platform).Login(ctx, req))
}

// HandleCallback OAuth2回调
//
//	@receiver h *oauth2Handler
//	@param ctx context.Context
//	@param req *dto.CallbackReq
//	@return *dto.HTTPResponse[*dto.CallbackRsp]
//	@return error
//	@author centonhuang
//	@update 2025-11-11 04:58:11
func (h *oauth2Handler) HandleCallback(ctx context.Context, req *dto.CallbackReq) (*dto.HTTPResponse[*dto.CallbackRsp], error) {
	return util.WrapHTTPResponse(h.getService(req.Body.Platform).Callback(ctx, req))
}

// getService 根据platform获取对应的service
//
//	receiver h *oauth2Handler
//	param platform string
//	return service.Oauth2Service
//	author centonhuang
//	update 2025-01-05 21:00:00
func (h *oauth2Handler) getService(platform string) service.Oauth2Service {
	switch platform {
	case enum.Oauth2PlatformGithub:
		return service.NewGithubOauth2Service()
	case enum.Oauth2PlatformGoogle:
		return service.NewGoogleOauth2Service()
	default:
		return service.NewGithubOauth2Service() // 默认返回 github
	}
}
