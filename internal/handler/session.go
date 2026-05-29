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
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
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
	HandleCreateShare(ctx context.Context, req *dto.CreateShareReq) (*dto.HTTPResponse[*dto.CreateShareRsp], error)
	HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error)
	HandleListShares(ctx context.Context, req *dto.ListSharesReq) (*dto.HTTPResponse[*dto.ListSharesRsp], error)
	HandleDeleteShare(ctx context.Context, req *dto.DeleteShareReq) (*dto.HTTPResponse[*dto.CommonRsp], error)
	// 新增（详情接口性能优化）
	HandleGetSessionMetadata(ctx context.Context, req *dto.GetSessionMetadataReq) (*dto.HTTPResponse[*dto.GetSessionMetadataRsp], error)
	HandleListSessionMessages(ctx context.Context, req *dto.ListSessionMessagesReq) (*dto.HTTPResponse[*dto.ListSessionMessagesRsp], error)
	HandleListSessionTools(ctx context.Context, req *dto.ListSessionToolsReq) (*dto.HTTPResponse[*dto.ListSessionToolsRsp], error)
}

// SessionDependencies SessionHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type SessionDependencies struct {
	ListByUser sessionquery.ListSessionsByUserHandler
	GetByUser  sessionquery.GetSessionByUserHandler
	ShareCache cache.ShareCache
	// 新增（详情接口性能优化）
	GetMetaByUser sessionquery.GetSessionMetaByUserHandler
	ListMessages  sessionquery.ListSessionMessagesHandler
	ListTools     sessionquery.ListSessionToolsHandler
}

type sessionHandler struct {
	listByUser    sessionquery.ListSessionsByUserHandler
	getByUser     sessionquery.GetSessionByUserHandler
	shareCache    cache.ShareCache
	getMetaByUser sessionquery.GetSessionMetaByUserHandler
	listMessages  sessionquery.ListSessionMessagesHandler
	listTools     sessionquery.ListSessionToolsHandler
}

// NewSessionHandler 创建Session处理器
//
//	@param deps SessionDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return SessionHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewSessionHandler(deps SessionDependencies) SessionHandler {
	return &sessionHandler{
		listByUser:    deps.ListByUser,
		getByUser:     deps.GetByUser,
		shareCache:    deps.ShareCache,
		getMetaByUser: deps.GetMetaByUser,
		listMessages:  deps.ListMessages,
		listTools:     deps.ListTools,
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
			ID:           v.ID,
			CreatedAt:    v.CreatedAt,
			UpdatedAt:    v.UpdatedAt,
			Summary:      v.Summary,
			MessageCount: v.MessageCount,
			ToolCount:    v.ToolCount,
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

	shareID, sharedErr := h.shareCache.GetSessionShareID(ctx, req.SessionID)
	if sharedErr != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Check session shared status failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(sharedErr))
		shareID = ""
	}

	rsp.Session = &dto.SessionDetail{
		ID:         view.ID,
		APIKeyName: view.APIKeyName,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
		Metadata:   view.Metadata,
		Messages:   messageItems,
		Tools:      toolItems,
		ShareID:    shareID,
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Get session detail by user",
		zap.Uint("sessionID", req.SessionID),
		zap.Uint("userID", userID),
		zap.Int("messageCount", len(messageItems)),
		zap.Int("toolCount", len(toolItems)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleCreateShare 创建分享链接（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.CreateShareReq
//	@return *dto.HTTPResponse[*dto.CreateShareRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleCreateShare(ctx context.Context, req *dto.CreateShareReq) (*dto.HTTPResponse[*dto.CreateShareRsp], error) {
	rsp := &dto.CreateShareRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	if req.Body == nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Create share: empty request body")
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	sessionID := req.Body.SessionID

	view, err := h.getByUser.Handle(ctx, sessionquery.GetSessionByUserQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: sessionID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Create share: verify session failed",
			zap.Uint("sessionID", sessionID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	if view == nil {
		rsp.Error = ierr.ErrDataNotExists.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	shareID, expiresAt, shareErr := h.shareCache.CreateShare(ctx, userID, sessionID)
	if shareErr != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Create share failed",
			zap.Uint("sessionID", sessionID), zap.Error(shareErr))
		rsp.Error = ierr.ToBizError(shareErr, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.ShareID = shareID
	rsp.ExpiresAt = expiresAt

	logger.WithCtx(ctx).Info("[SessionHandler] Share created",
		zap.String("shareID", shareID),
		zap.Uint("sessionID", sessionID),
		zap.Uint("userID", userID))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetShareContent 获取分享内容（公开接口，IP限流）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.GetShareContentReq
//	@return *dto.HTTPResponse[*dto.GetShareContentRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error) {
	rsp := &dto.GetShareContentRsp{}

	sessionID, err := h.shareCache.GetShareSessionID(ctx, req.ShareID)
	if err != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Get share content: share not found",
			zap.String("shareID", req.ShareID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrDataNotExists.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	view, viewErr := h.getByUser.Handle(ctx, sessionquery.GetSessionByUserQuery{
		UserID:             0,
		IsAdmin:            true,
		SkipOwnershipCheck: true,
		SessionID:          sessionID,
	})
	if viewErr != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Get share content: fetch session failed",
			zap.Uint("sessionID", sessionID), zap.Error(viewErr))
		rsp.Error = ierr.ToBizError(viewErr, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	if view == nil {
		rsp.Error = ierr.ErrDataNotExists.BizError()
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

	rsp.Session = &dto.ShareContentSessionDetail{
		ID:        view.ID,
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
		Metadata:  view.Metadata,
		Messages:  messageItems,
		Tools:     toolItems,
	}

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListShares 获取当前用户的分享列表（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.ListSharesReq
//	@return *dto.HTTPResponse[*dto.ListSharesRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleListShares(ctx context.Context, req *dto.ListSharesReq) (*dto.HTTPResponse[*dto.ListSharesRsp], error) {
	rsp := &dto.ListSharesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	shares, pageInfo, err := h.shareCache.ListUserShares(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List shares failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Shares = shares
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleDeleteShare 取消分享（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.DeleteShareReq
//	@return *dto.HTTPResponse[*dto.CommonRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleDeleteShare(ctx context.Context, req *dto.DeleteShareReq) (*dto.HTTPResponse[*dto.CommonRsp], error) {
	rsp := &dto.CommonRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	err := h.shareCache.DeleteShare(ctx, userID, req.ShareID)
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Delete share failed",
			zap.String("shareID", req.ShareID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Share deleted",
		zap.String("shareID", req.ShareID))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetSessionMetadata 获取 Session 元数据（不含 messages/tools 内容）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleGetSessionMetadata(ctx context.Context, req *dto.GetSessionMetadataReq) (*dto.HTTPResponse[*dto.GetSessionMetadataRsp], error) {
	rsp := &dto.GetSessionMetadataRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.getMetaByUser.Handle(ctx, sessionquery.GetSessionMetaByUserQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Get session metadata failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	shareID, sharedErr := h.shareCache.GetSessionShareID(ctx, req.SessionID)
	if sharedErr != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Check session shared status failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(sharedErr))
		shareID = ""
	}

	rsp.Session = &dto.SessionMetadata{
		ID:           view.ID,
		APIKeyName:   view.APIKeyName,
		CreatedAt:    view.CreatedAt,
		UpdatedAt:    view.UpdatedAt,
		Metadata:     view.Metadata,
		MessageCount: view.MessageCount,
		ToolCount:    view.ToolCount,
		ShareID:      shareID,
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Get session metadata",
		zap.Uint("sessionID", req.SessionID),
		zap.Uint("userID", userID),
		zap.Int("messageCount", view.MessageCount),
		zap.Int("toolCount", view.ToolCount))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListSessionMessages 分页获取 Session 消息
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleListSessionMessages(ctx context.Context, req *dto.ListSessionMessagesReq) (*dto.HTTPResponse[*dto.ListSessionMessagesRsp], error) {
	rsp := &dto.ListSessionMessagesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	result, err := h.listMessages.Handle(ctx, sessionquery.ListSessionMessagesQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
		Offset:    req.Offset,
		Limit:     req.Limit,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List session messages failed",
			zap.Uint("sessionID", req.SessionID),
			zap.Int("offset", req.Offset), zap.Int("limit", req.Limit), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Messages = lo.Map(result.Messages, func(m *sessionquery.MessageView, _ int) *dto.MessageItem {
		return &dto.MessageItem{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})
	rsp.PageInfo = &dto.OffsetPageInfo{
		Offset: req.Offset,
		Limit:  req.Limit,
		Total:  result.Total,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListSessionTools 分页获取 Session 工具
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleListSessionTools(ctx context.Context, req *dto.ListSessionToolsReq) (*dto.HTTPResponse[*dto.ListSessionToolsRsp], error) {
	rsp := &dto.ListSessionToolsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	result, err := h.listTools.Handle(ctx, sessionquery.ListSessionToolsQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
		Offset:    req.Offset,
		Limit:     req.Limit,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List session tools failed",
			zap.Uint("sessionID", req.SessionID),
			zap.Int("offset", req.Offset), zap.Int("limit", req.Limit), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Tools = lo.Map(result.Tools, func(t *sessionquery.ToolView, _ int) *dto.ToolItem {
		return &dto.ToolItem{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})
	rsp.PageInfo = &dto.OffsetPageInfo{
		Offset: req.Offset,
		Limit:  req.Limit,
		Total:  result.Total,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
