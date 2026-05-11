package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AuditHandler 审计处理器
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
}

// AuditDependencies AuditHandler 依赖项
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditDependencies struct {
	List auditquery.ListAuditLogsHandler
}

type auditHandler struct {
	list auditquery.ListAuditLogsHandler
}

// NewAuditHandler 创建审计处理器
//
//	@param deps AuditDependencies
//	@return AuditHandler
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{list: deps.List}
}

// HandleListAuditLogs 分页获取审计日志列表
//
//	@receiver h *auditHandler
//	@param ctx context.Context
//	@param req *dto.ListAuditLogsReq
//	@return *dto.HTTPResponse[*dto.ListAuditLogsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	apiKeyID := util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID)

	audits, pageInfo, err := h.list.Handle(ctx, auditquery.ListAuditLogsQuery{
		APIKeyID:  apiKeyID,
		Page:      req.Page,
		PageSize:  req.PageSize,
		Query:     req.Query,
		Sort:      req.Sort,
		SortField: req.SortField,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) *dto.AuditLogItem {
		return &dto.AuditLogItem{
			ID:                        a.AggregateID(),
			CreatedAt:                 a.CreatedAt(),
			Model:                     a.Model(),
			UpstreamProvider:          a.UpstreamProvider(),
			APIProvider:               a.APIProvider(),
			InputTokens:               a.Tokens().Input(),
			OutputTokens:              a.Tokens().Output(),
			CacheCreationInputTokens:  a.Tokens().CacheCreation(),
			CacheReadInputTokens:      a.Tokens().CacheRead(),
			FirstTokenLatencyMs:       a.Latency().FirstTokenMs(),
			StreamDurationMs:          a.Latency().StreamMs(),
			UserAgent:                 a.UserAgent(),
			UpstreamStatusCode:        a.Status().UpstreamStatusCode(),
			ErrorMessage:              a.Status().ErrorMessage(),
			TraceID:                   a.TraceID(),
		}
	})
	rsp.PageInfo = pageInfo
	return util.WrapHTTPResponse(rsp, nil)
}
