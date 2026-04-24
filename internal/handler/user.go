// Package handler 用户处理器
package handler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
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

type userHandler struct {
	getCurrentUser query.GetCurrentUserHandler
	updateProfile  command.UpdateProfileHandler
}

// NewUserHandler 创建用户处理器
//
//	@return UserHandler
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func NewUserHandler() UserHandler {
	userRepo := repository.NewUserRepository()
	return &userHandler{
		getCurrentUser: query.NewGetCurrentUserHandler(userRepo),
		updateProfile:  command.NewUpdateProfileHandler(userRepo),
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

	view, err := h.getCurrentUser.Handle(ctx, query.GetCurrentUserQuery{UserID: userID})
	if err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.User = &dto.DetailedUser{
		ID:         view.ID,
		CreatedAt:  view.CreatedAt.Format(time.DateTime),
		LastLogin:  view.LastLogin.Format(time.DateTime),
		Permission: string(view.Permission),
		User: dto.User{
			Name:   view.Name,
			Email:  view.Email,
			Avatar: view.Avatar,
		},
	}

	logger.WithCtx(ctx).Info("[UserHandler] get cur user info",
		zap.String("email", view.Email),
		zap.String("avatar", view.Avatar),
		zap.Time("createdAt", view.CreatedAt),
		zap.Time("lastLogin", view.LastLogin),
		zap.String("permission", string(view.Permission)))

	return util.WrapHTTPResponse(rsp, nil)
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

	err := h.updateProfile.Handle(ctx, command.UpdateProfileCommand{
		UserID: userID,
		Name:   req.Body.User.Name,
		Email:  req.Body.User.Email,
		Avatar: req.Body.User.Avatar,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Update user failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrDBUpdate.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}
	return util.WrapHTTPResponse(rsp, nil)
}
