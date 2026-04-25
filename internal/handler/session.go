// Package handler Session处理器
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// SessionHandler Session处理器
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type SessionHandler interface {
	HandleListSessions(ctx context.Context, req *dto.ListSessionsReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error)
	HandleGetSession(ctx context.Context, req *dto.GetSessionReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error)
}

type sessionHandler struct {
	list sessionquery.ListSessionsHandler
	get  sessionquery.GetSessionHandler
}

// NewSessionHandler 创建Session处理器
//
//	@return SessionHandler
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewSessionHandler() SessionHandler {
	sessionReadRepo := repository.NewSessionReadRepository()
	return &sessionHandler{
		list: sessionquery.NewListSessionsHandler(sessionReadRepo),
		get:  sessionquery.NewGetSessionHandler(sessionReadRepo),
	}
}

// HandleListSessions 分页获取Session列表
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.ListSessionsReq
//	@return *dto.HTTPResponse[*dto.ListSessionsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func (h *sessionHandler) HandleListSessions(ctx context.Context, req *dto.ListSessionsReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error) {
	rsp := &dto.ListSessionsRsp{}
	apiKeyName := util.CtxValueString(ctx, constant.CtxKeyUserName)

	views, pageInfo, err := h.list.Handle(ctx, sessionquery.ListSessionsQuery{
		OwnerAPIKeyName: apiKeyName,
		Page:            req.Page,
		PageSize:        req.PageSize,
	})
	if err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
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
	return util.WrapHTTPResponse(rsp, nil)
}

// HandleGetSession 获取Session详情
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.GetSessionReq
//	@return *dto.HTTPResponse[*dto.GetSessionRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func (h *sessionHandler) HandleGetSession(ctx context.Context, req *dto.GetSessionReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error) {
	rsp := &dto.GetSessionRsp{}
	apiKeyName := util.CtxValueString(ctx, constant.CtxKeyUserName)

	view, err := h.get.Handle(ctx, sessionquery.GetSessionQuery{
		SessionID:       req.SessionID,
		OwnerAPIKeyName: apiKeyName,
	})
	if err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
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

	logger.WithCtx(ctx).Info("[SessionHandler] Get session detail",
		zap.Uint("sessionID", req.SessionID),
		zap.String("apiKeyName", apiKeyName),
		zap.Int("messageCount", len(messageItems)),
		zap.Int("toolCount", len(toolItems)))

	return util.WrapHTTPResponse(rsp, nil)
}
