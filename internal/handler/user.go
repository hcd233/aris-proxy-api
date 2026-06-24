// Package handler 用户处理器
package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// UserHandler 用户处理器
//
//	@author centonhuang
//	@update 2025-01-04 15:56:20
type UserHandler interface {
	HandleGetCurUser(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.GetCurUserRsp], error)
	HandleUpdateUser(ctx context.Context, req *dto.UpdateUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

// UserDependencies UserHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type UserDependencies struct {
	GetCurrentUser port.GetCurrentUserHandler
	UpdateProfile  port.UpdateProfileHandler
}

type userHandler struct {
	getCurrentUser port.GetCurrentUserHandler
	updateProfile  port.UpdateProfileHandler
}

// NewUserHandler 创建用户处理器
//
//	@param deps UserDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return UserHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewUserHandler(deps UserDependencies) UserHandler {
	return &userHandler{
		getCurrentUser: deps.GetCurrentUser,
		updateProfile:  deps.UpdateProfile,
	}
}

// HandleGetCurUser 获取当前用户信息
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.GetCurUserRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *userHandler) HandleGetCurUser(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.GetCurUserRsp], error) {
	rsp := &dto.GetCurUserRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	view, err := h.getCurrentUser.Handle(ctx, port.GetCurrentUserQuery{UserID: userID})
	if err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Get current user failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.User = &dto.DetailedUser{
		ID:         view.ID,
		CreatedAt:  view.CreatedAt,
		LastLogin:  view.LastLogin,
		Permission: string(view.Permission),
		User: dto.User{
			Name:   view.Name,
			Email:  view.Email,
			Avatar: view.Avatar,
		},
	}

	logger.WithCtx(ctx).Info("[UserHandler] Get cur user info",
		zap.String("email", view.Email),
		zap.String("avatar", view.Avatar),
		zap.Time("createdAt", view.CreatedAt),
		zap.Time("lastLogin", view.LastLogin),
		zap.String("permission", string(view.Permission)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleUpdateUser 更新当前用户资料
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.UpdateUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *userHandler) HandleUpdateUser(ctx context.Context, req *dto.UpdateUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	err := h.updateProfile.Handle(ctx, port.UpdateProfileCommand{
		UserID: userID,
		Name:   req.Body.User.Name,
		Email:  req.Body.User.Email,
		Avatar: req.Body.User.Avatar,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Update user failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
