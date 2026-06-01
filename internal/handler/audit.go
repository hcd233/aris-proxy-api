package handler

import (
	"context"
	"errors"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
	HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error)
	HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error)
}

type AuditDependencies struct {
	Service auditquery.AuditService
}

type auditHandler struct {
	svc auditquery.AuditService
}

func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{svc: deps.Service}
}

func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	logs, pageInfo, err := h.svc.ListLogs(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		auditquery.ListAuditLogsParams{
			Page: req.Page, PageSize: req.PageSize, Query: req.Query,
			Sort: req.Sort, SortField: req.SortField,
			StartTime: req.StartTime, EndTime: req.EndTime,
		},
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = bizErrorFrom(err)
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

func (h *auditHandler) HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error) {
	rsp := &dto.ModelTrendRsp{}
	points, err := h.svc.ModelTrend(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Model trend failed", zap.Error(err))
		rsp.Error = bizErrorFrom(err)
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditquery.FillTrendSeries(points)
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
		rsp.Error = bizErrorFrom(err)
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditquery.FillRateSeries(points)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// bizErrorFrom 把 ierr error 转换为可挂在 rsp.Error 上的业务错误。
func bizErrorFrom(err error) *model.Error {
	if errors.Is(err, ierr.ErrUnauthorized) {
		return ierr.ErrUnauthorized.BizError()
	}
	return ierr.ToBizError(err, ierr.ErrInternal.BizError())
}
