package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
	HandleListAuditOption(ctx context.Context, req *dto.AuditOptionListReq) (*dto.HTTPResponse[*dto.AuditOptionListRsp], error)
	HandleDashboardOverview(ctx context.Context, req *dto.DashboardRangeReq) (*dto.HTTPResponse[*dto.DashboardOverviewRsp], error)
	HandleDashboardModelSeries(ctx context.Context, req *dto.DashboardRangeReq) (*dto.HTTPResponse[*dto.DashboardModelSeriesRsp], error)
	HandleDashboardErrors(ctx context.Context, req *dto.DashboardWindowReq) (*dto.HTTPResponse[*dto.DashboardErrorListRsp], error)
	HandleDashboardEndpoints(ctx context.Context, req *dto.DashboardWindowReq) (*dto.HTTPResponse[*dto.DashboardEndpointListRsp], error)
	HandleDashboardTop(ctx context.Context, req *dto.DashboardTopReq) (*dto.HTTPResponse[*dto.DashboardTopListRsp], error)
	HandleDashboardHeatmap(ctx context.Context, req *dto.DashboardHeatmapReq) (*dto.HTTPResponse[*dto.DashboardHeatmapRsp], error)
}

type AuditDependencies struct {
	Service   auditport.AuditService
	Dashboard auditport.DashboardService
}

type auditHandler struct {
	svc       auditport.AuditService
	dashboard auditport.DashboardService
}

func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{svc: deps.Service, dashboard: deps.Dashboard}
}

func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	logs, pageInfo, err := h.svc.ListLogs(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		auditport.ListAuditLogsParams{
			Page: req.Page, PageSize: req.PageSize, Query: req.Query,
			Sort: req.Sort, SortField: req.SortField,
			StartTime: req.StartTime, EndTime: req.EndTime,
			Filter: req.Filter,
		},
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Logs = lo.Map(logs, func(log *auditport.AuditLogView, _ int) *dto.AuditLogItem {
		return &dto.AuditLogItem{
			ID:                       log.ID,
			CreatedAt:                log.CreatedAt,
			ModelID:                  log.ModelID,
			UpstreamProtocol:         log.UpstreamProtocol,
			APIProtocol:              log.APIProtocol,
			Endpoint:                 log.Endpoint,
			InputTokens:              log.InputTokens,
			OutputTokens:             log.OutputTokens,
			CacheCreationInputTokens: log.CacheCreationInputTokens,
			CacheReadInputTokens:     log.CacheReadInputTokens,
			FirstTokenLatencyMs:      log.FirstTokenLatencyMs,
			StreamDurationMs:         log.StreamDurationMs,
			UserAgent:                log.UserAgent,
			UpstreamStatusCode:       log.UpstreamStatusCode,
			ErrorMessage:             log.ErrorMessage,
			IsError:                  log.IsError,
			TraceID:                  log.TraceID,
			Cost:                     costDisplayFromMicro(log.CostMicro),
			PricingCurrency:          log.PricingCurrency,
			APIKeyName:               log.APIKeyName,
			UserName:                 log.UserName,
			UserEmail:                log.UserEmail,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListAuditOption 获取审计筛选选项
//
//	@receiver h *auditHandler
//	@param ctx context.Context
//	@param req *dto.AuditOptionListReq
//	@return *dto.HTTPResponse[*dto.AuditOptionListRsp]
//	@return error
//	@author centonhuang
//	@update 2026-06-10 12:00:00
func (h *auditHandler) HandleListAuditOption(ctx context.Context, req *dto.AuditOptionListReq) (*dto.HTTPResponse[*dto.AuditOptionListRsp], error) {
	rsp := &dto.AuditOptionListRsp{}

	items, err := h.svc.ListAuditOption(
		ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.Field,
		req.Keyword,
		req.StartTime,
		req.EndTime,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit options failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Items = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// costDisplayFromMicro 微单位费用指针 → 展示单位指针（nil 原样透传=未计价）
func costDisplayFromMicro(micro *int64) *float64 {
	if micro == nil {
		return nil
	}
	v := dto.PriceDisplayFromMicro(*micro)
	return &v
}
