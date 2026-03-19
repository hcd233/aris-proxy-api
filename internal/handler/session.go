// Package handler Session处理器
package handler

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
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
	svc service.SessionService
}

// NewSessionHandler 创建Session处理器
//
//	@return SessionHandler
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func NewSessionHandler() SessionHandler {
	return &sessionHandler{
		svc: service.NewSessionService(),
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
//	@update 2026-03-19 10:00:00
func (h *sessionHandler) HandleListSessions(ctx context.Context, req *dto.ListSessionsReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error) {
	return util.WrapHTTPResponse(h.svc.ListSessions(ctx, req))
}

// HandleGetSession 获取Session详情
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.GetSessionReq
//	@return *dto.HTTPResponse[*dto.GetSessionRsp]
//	@return error
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func (h *sessionHandler) HandleGetSession(ctx context.Context, req *dto.GetSessionReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error) {
	return util.WrapHTTPResponse(h.svc.GetSession(ctx, req))
}
