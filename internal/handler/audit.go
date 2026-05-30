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
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
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
	ListAll           auditquery.ListAllAuditLogsHandler
	ListByUser        auditquery.ListAuditLogsByUserHandler
	ModelTrend        auditquery.ModelTrendHandler
	ModelTrendByUser  auditquery.ModelTrendByUserHandler
	RequestRate       auditquery.RequestRateHandler
	RequestRateByUser auditquery.RequestRateByUserHandler
}

type auditHandler struct {
	listAll           auditquery.ListAllAuditLogsHandler
	listByUser        auditquery.ListAuditLogsByUserHandler
	modelTrend        auditquery.ModelTrendHandler
	modelTrendByUser  auditquery.ModelTrendByUserHandler
	requestRate       auditquery.RequestRateHandler
	requestRateByUser auditquery.RequestRateByUserHandler
}

func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{
		listAll:           deps.ListAll,
		listByUser:        deps.ListByUser,
		modelTrend:        deps.ModelTrend,
		modelTrendByUser:  deps.ModelTrendByUser,
		requestRate:       deps.RequestRate,
		requestRateByUser: deps.RequestRateByUser,
	}
}

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

func (h *auditHandler) HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error) {
	rsp := &dto.ModelTrendRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var points []*modelcall.ModelTrendPoint
	var err error

	switch permission {
	case enum.PermissionAdmin:
		points, err = h.modelTrend.Handle(ctx, auditquery.ModelTrendQuery{
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	case enum.PermissionUser:
		points, err = h.modelTrendByUser.Handle(ctx, auditquery.ModelTrendByUserQuery{
			UserID:      userID,
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Model trend failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Data = groupTrendPoints(points)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error) {
	rsp := &dto.RequestRateRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var points []*modelcall.RequestRatePoint
	var err error

	switch permission {
	case enum.PermissionAdmin:
		points, err = h.requestRate.Handle(ctx, auditquery.RequestRateQuery{
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	case enum.PermissionUser:
		points, err = h.requestRateByUser.Handle(ctx, auditquery.RequestRateByUserQuery{
			UserID:      userID,
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Request rate failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Data = groupRatePoints(points)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func groupTrendPoints(points []*modelcall.ModelTrendPoint) []*dto.ModelTrendItem {
	modelMap := make(map[string][]*dto.TrendPoint)
	modelOrder := make([]string, 0)
	for _, p := range points {
		if _, ok := modelMap[p.Model]; !ok {
			modelOrder = append(modelOrder, p.Model)
		}
		modelMap[p.Model] = append(modelMap[p.Model], &dto.TrendPoint{
			Time:  p.Time,
			Count: p.Count,
		})
	}
	items := make([]*dto.ModelTrendItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		items = append(items, &dto.ModelTrendItem{
			Model:  m,
			Points: modelMap[m],
		})
	}
	return items
}

func groupRatePoints(points []*modelcall.RequestRatePoint) []*dto.RequestRateItem {
	modelMap := make(map[string][]*dto.RatePoint)
	modelOrder := make([]string, 0)
	for _, p := range points {
		if _, ok := modelMap[p.Model]; !ok {
			modelOrder = append(modelOrder, p.Model)
		}
		failed := p.Total - p.Success
		var rate float64
		if p.Total > 0 {
			rate = float64(p.Success) / float64(p.Total)
		}
		modelMap[p.Model] = append(modelMap[p.Model], &dto.RatePoint{
			Time:        p.Time,
			Total:       p.Total,
			Success:     p.Success,
			Failed:      failed,
			SuccessRate: rate,
		})
	}
	items := make([]*dto.RequestRateItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		items = append(items, &dto.RequestRateItem{
			Model:  m,
			Points: modelMap[m],
		})
	}
	return items
}
