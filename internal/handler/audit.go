package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AuditHandler 审计处理器
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
}

// AuditDependencies AuditHandler 依赖项
type AuditDependencies struct {
	ListAll    auditquery.ListAllAuditLogsHandler
	ListByUser auditquery.ListAuditLogsByUserHandler
}

type auditHandler struct {
	listAll    auditquery.ListAllAuditLogsHandler
	listByUser auditquery.ListAuditLogsByUserHandler
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{
		listAll:    deps.ListAll,
		listByUser: deps.ListByUser,
	}
}

// HandleListAuditLogs 分页获取审计日志列表，按当前 JWT 用户权限分级返回数据范围
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var (
		logs     []*auditquery.AuditLogView
		pageInfo *model.PageInfo
		err      error
	)

	switch permission {
	case enum.PermissionAdmin:
		logs, pageInfo, err = h.listAll.Handle(ctx, auditquery.ListAllAuditLogsQuery{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	case enum.PermissionUser:
		logs, pageInfo, err = h.listByUser.Handle(ctx, auditquery.ListAuditLogsByUserQuery{
			UserID:    userID,
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = make([]*dto.AuditLogItem, 0, len(logs))
	for _, log := range logs {
		rsp.Logs = append(rsp.Logs, &dto.AuditLogItem{
			ID:                       log.ID,
			CreatedAt:                log.CreatedAt,
			Model:                    log.Model,
			UpstreamProvider:         log.UpstreamProvider,
			APIProvider:              log.APIProvider,
			InputTokens:              log.InputTokens,
			OutputTokens:             log.OutputTokens,
			CacheCreationInputTokens: log.CacheCreationInputTokens,
			CacheReadInputTokens:     log.CacheReadInputTokens,
			FirstTokenLatencyMs:      log.FirstTokenLatencyMs,
			StreamDurationMs:         log.StreamDurationMs,
			UserAgent:                log.UserAgent,
			UpstreamStatusCode:       log.UpstreamStatusCode,
			ErrorMessage:             log.ErrorMessage,
			TraceID:                  log.TraceID,
			APIKeyName:               log.APIKeyName,
			UserName:                 log.UserName,
			UserEmail:                log.UserEmail,
		})
	}
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}
