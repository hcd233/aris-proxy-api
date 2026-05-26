// Package handler Session处理器
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// SessionHandler Session处理器
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type SessionHandler interface {
	HandleListSessionsByUser(ctx context.Context, req *dto.ListSessionsByUserReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error)
	HandleGetSessionByUser(ctx context.Context, req *dto.GetSessionByUserReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error)
}

// SessionDependencies SessionHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type SessionDependencies struct {
	ListByUser sessionquery.ListSessionsByUserHandler
	GetByUser  sessionquery.GetSessionByUserHandler
}

type sessionHandler struct {
	listByUser sessionquery.ListSessionsByUserHandler
	getByUser  sessionquery.GetSessionByUserHandler
}

// NewSessionHandler 创建Session处理器
//
//	@param deps SessionDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return SessionHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewSessionHandler(deps SessionDependencies) SessionHandler {
	return &sessionHandler{
		listByUser: deps.ListByUser,
		getByUser:  deps.GetByUser,
	}
}

// HandleListSessionsByUser 分页获取当前用户的Session列表（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.ListSessionsByUserReq
//	@return *dto.HTTPResponse[*dto.ListSessionsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-24 10:00:00
func (h *sessionHandler) HandleListSessionsByUser(ctx context.Context, req *dto.ListSessionsByUserReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error) {
	rsp := &dto.ListSessionsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	views, pageInfo, err := h.listByUser.Handle(ctx, sessionquery.ListSessionsByUserQuery{
		UserID:   userID,
		IsAdmin:  isAdmin,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List sessions by user failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Sessions = lo.Map(views, func(v *sessionquery.SessionSummaryView, _ int) *dto.SessionSummary {
		return &dto.SessionSummary{
			ID:         v.ID,
			CreatedAt:  v.CreatedAt,
			UpdatedAt:  v.UpdatedAt,
			Summary:    v.Summary,
			MessageIDs: v.MessageIDs,
			ToolIDs:    v.ToolIDs,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetSessionByUser 获取当前用户的Session详情（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.GetSessionByUserReq
//	@return *dto.HTTPResponse[*dto.GetSessionRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-24 10:00:00
func (h *sessionHandler) HandleGetSessionByUser(ctx context.Context, req *dto.GetSessionByUserReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error) {
	rsp := &dto.GetSessionRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.getByUser.Handle(ctx, sessionquery.GetSessionByUserQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Get session by user failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	messageItems := lo.Map(view.Messages, func(m *sessionquery.MessageView, _ int) *dto.MessageItem {
		return &dto.MessageItem{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})
	toolItems := lo.Map(view.Tools, func(t *sessionquery.ToolView, _ int) *dto.ToolItem {
		return &dto.ToolItem{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})

	rsp.Session = &dto.SessionDetail{
		ID:         view.ID,
		APIKeyName: view.APIKeyName,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
		Metadata:   view.Metadata,
		Messages:   messageItems,
		Tools:      toolItems,
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Get session detail by user",
		zap.Uint("sessionID", req.SessionID),
		zap.Uint("userID", userID),
		zap.Int("messageCount", len(messageItems)),
		zap.Int("toolCount", len(toolItems)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
