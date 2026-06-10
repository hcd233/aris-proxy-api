package handler

import (
	"context"

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
	HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error)
	HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error)
	HandleTokenThroughput(ctx context.Context, req *dto.TokenThroughputReq) (*dto.HTTPResponse[*dto.TokenThroughputRsp], error)
	HandleTokenRate(ctx context.Context, req *dto.TokenRateReq) (*dto.HTTPResponse[*dto.TokenRateRsp], error)
	HandleModelUsage(ctx context.Context, req *dto.ModelUsageReq) (*dto.HTTPResponse[*dto.ModelUsageRsp], error)
	HandleFirstTokenLatency(ctx context.Context, req *dto.FirstTokenLatencyReq) (*dto.HTTPResponse[*dto.FirstTokenLatencyRsp], error)
}

type AuditDependencies struct {
	Service auditport.AuditService
}

type auditHandler struct {
	svc auditport.AuditService
}

func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{svc: deps.Service}
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
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = make([]*dto.AuditLogItem, 0, len(logs))
	for _, log := range logs {
		rsp.Logs = append(rsp.Logs, &dto.AuditLogItem{
			ID:                       log.ID,
			CreatedAt:                log.CreatedAt,
			Model:                    log.Model,
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
			TraceID:                  log.TraceID,
			APIKeyName:               log.APIKeyName,
			UserName:                 log.UserName,
			UserEmail:                log.UserEmail,
		})
	}
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

	items, err := h.svc.ListAuditOption(ctx, req.Field, req.Keyword, req.StartTime, req.EndTime)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit options failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Items = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error) {
	rsp := &dto.ModelTrendRsp{}
	points, err := h.svc.ModelTrend(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Model trend failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditport.FillTrendSeries(points, req.StartTime, req.EndTime, req.Granularity)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error) {
	rsp := &dto.RequestRateRsp{}
	points, err := h.svc.RequestRate(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Request rate failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditport.FillRateSeries(points, req.StartTime, req.EndTime, req.Granularity)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleTokenThroughput(ctx context.Context, req *dto.TokenThroughputReq) (*dto.HTTPResponse[*dto.TokenThroughputRsp], error) {
	rsp := &dto.TokenThroughputRsp{}
	points, err := h.svc.TokenThroughput(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token throughput failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditport.FillTokenThroughputSeries(points, req.StartTime, req.EndTime, req.Granularity)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleTokenRate(ctx context.Context, req *dto.TokenRateReq) (*dto.HTTPResponse[*dto.TokenRateRsp], error) {
	rsp := &dto.TokenRateRsp{}
	items, err := h.svc.TokenRate(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token rate failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleModelUsage(ctx context.Context, req *dto.ModelUsageReq) (*dto.HTTPResponse[*dto.ModelUsageRsp], error) {
	rsp := &dto.ModelUsageRsp{}
	items, err := h.svc.ModelUsage(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token usage failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleFirstTokenLatency(ctx context.Context, req *dto.FirstTokenLatencyReq) (*dto.HTTPResponse[*dto.FirstTokenLatencyRsp], error) {
	rsp := &dto.FirstTokenLatencyRsp{}
	items, err := h.svc.FirstTokenLatency(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] First token latency failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}
