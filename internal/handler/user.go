// Package handler 用户处理器
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
	HandleListUsers(ctx context.Context, req *dto.ListUsersReq) (*dto.HTTPResponse[*dto.ListUsersRsp], error)
	HandleApproveUser(ctx context.Context, req *dto.ApproveUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDemoteUser(ctx context.Context, req *dto.DemoteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteUser(ctx context.Context, req *dto.DeleteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleSetDemoUser(ctx context.Context, req *dto.SetDemoUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleRestoreDemoUser(ctx context.Context, req *dto.RestoreDemoUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

// UserDependencies UserHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type UserDependencies struct {
	GetCurrentUser  port.GetCurrentUserHandler
	UpdateProfile   port.UpdateProfileHandler
	ListUsers       port.ListUsersHandler
	ApproveUser     port.ApproveUserHandler
	DemoteUser      port.DemoteUserHandler
	DeleteUser      port.DeleteUserHandler
	SetDemoUser     port.SetDemoUserHandler
	RestoreDemoUser port.RestoreDemoUserHandler
}

type userHandler struct {
	getCurrentUser  port.GetCurrentUserHandler
	updateProfile   port.UpdateProfileHandler
	listUsers       port.ListUsersHandler
	approveUser     port.ApproveUserHandler
	demoteUser      port.DemoteUserHandler
	deleteUser      port.DeleteUserHandler
	setDemoUser     port.SetDemoUserHandler
	restoreDemoUser port.RestoreDemoUserHandler
}

// NewUserHandler 创建用户处理器
//
//	@param deps UserDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return UserHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewUserHandler(deps UserDependencies) UserHandler {
	return &userHandler{
		getCurrentUser:  deps.GetCurrentUser,
		updateProfile:   deps.UpdateProfile,
		listUsers:       deps.ListUsers,
		approveUser:     deps.ApproveUser,
		demoteUser:      deps.DemoteUser,
		deleteUser:      deps.DeleteUser,
		setDemoUser:     deps.SetDemoUser,
		restoreDemoUser: deps.RestoreDemoUser,
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
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
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
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListUsers 用户列表（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.ListUsersReq
//	@return *dto.HTTPResponse[*dto.ListUsersRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func (h *userHandler) HandleListUsers(ctx context.Context, req *dto.ListUsersReq) (*dto.HTTPResponse[*dto.ListUsersRsp], error) {
	rsp := &dto.ListUsersRsp{}
	views, pageInfo, err := h.listUsers.Handle(ctx, port.ListUsersQuery{
		CommonParam: req.CommonParam,
		Permission:  enum.Permission(req.Permission),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] List users failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Items = lo.Map(views, func(v *port.UserView, _ int) *dto.UserItem {
		return &dto.UserItem{
			ID:         v.ID,
			Name:       v.Name,
			Email:      v.Email,
			Avatar:     v.Avatar,
			Permission: string(v.Permission),
			CreatedAt:  v.CreatedAt,
			LastLogin:  v.LastLogin,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleApproveUser 审核用户：pending → user（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.ApproveUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func (h *userHandler) HandleApproveUser(ctx context.Context, req *dto.ApproveUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.approveUser.Handle(ctx, port.ApproveUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Approve user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleDemoteUser 降级用户：user → pending（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.DemoteUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (h *userHandler) HandleDemoteUser(ctx context.Context, req *dto.DemoteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.demoteUser.Handle(ctx, port.DemoteUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Demote user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleDeleteUser 删除用户（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.DeleteUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (h *userHandler) HandleDeleteUser(ctx context.Context, req *dto.DeleteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.deleteUser.Handle(ctx, port.DeleteUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Delete user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleSetDemoUser 设置 Demo 账户：pending/user → demo（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.SetDemoUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *userHandler) HandleSetDemoUser(ctx context.Context, req *dto.SetDemoUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.setDemoUser.Handle(ctx, port.SetDemoUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Set demo user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleRestoreDemoUser 恢复 Demo 账户：demo → user（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.RestoreDemoUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *userHandler) HandleRestoreDemoUser(ctx context.Context, req *dto.RestoreDemoUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.restoreDemoUser.Handle(ctx, port.RestoreDemoUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Restore demo user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}
