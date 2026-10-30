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

// dashboardScope 从 JWT 上下文取调用者视角
func dashboardScope(ctx context.Context) auditport.DashboardScope {
	return auditport.DashboardScope{
		Permission: util.CtxValuePermission(ctx),
		UserID:     util.CtxValueUint(ctx, constant.CtxKeyUserID),
	}
}

// HandleDashboardOverview dashboard KPI 总览（本期 + 上一等长周期 + 可用性状态条）
func (h *auditHandler) HandleDashboardOverview(ctx context.Context, req *dto.DashboardOverviewReq) (*dto.HTTPResponse[*dto.DashboardOverviewRsp], error) {
	ov, err := h.dashboard.Overview(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.Granularity, req.UptimeBuckets)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard overview failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardOverviewRsp{Data: auditport.ToDashboardOverviewDTO(ov)}, nil)
}

// HandleDashboardModelSeries dashboard 模型 × 时间桶用量
func (h *auditHandler) HandleDashboardModelSeries(ctx context.Context, req *dto.DashboardRangeReq) (*dto.HTTPResponse[*dto.DashboardModelSeriesRsp], error) {
	points, err := h.dashboard.ModelSeries(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.Granularity)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard model series failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardModelSeriesRsp{Data: auditport.ToDashboardModelSeriesDTO(points)}, nil)
}

// HandleDashboardErrors dashboard 热门错误
func (h *auditHandler) HandleDashboardErrors(ctx context.Context, req *dto.DashboardWindowReq) (*dto.HTTPResponse[*dto.DashboardErrorListRsp], error) {
	rows, err := h.dashboard.TopErrors(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.Limit)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard errors failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardErrorListRsp{Data: auditport.ToDashboardErrorsDTO(rows)}, nil)
}

// HandleDashboardEndpoints dashboard Endpoint 健康度
func (h *auditHandler) HandleDashboardEndpoints(ctx context.Context, req *dto.DashboardWindowReq) (*dto.HTTPResponse[*dto.DashboardEndpointListRsp], error) {
	rows, err := h.dashboard.EndpointHealth(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.Limit)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard endpoints failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardEndpointListRsp{Data: auditport.ToDashboardEndpointsDTO(rows)}, nil)
}

// HandleDashboardTop dashboard Top 排行
func (h *auditHandler) HandleDashboardTop(ctx context.Context, req *dto.DashboardTopReq) (*dto.HTTPResponse[*dto.DashboardTopListRsp], error) {
	top, err := h.dashboard.TopUsage(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.Dimension, req.Limit)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard top failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardTopListRsp{Data: auditport.ToDashboardTopDTO(top)}, nil)
}

// HandleDashboardHeatmap dashboard 活跃时段热力图
func (h *auditHandler) HandleDashboardHeatmap(ctx context.Context, req *dto.DashboardHeatmapReq) (*dto.HTTPResponse[*dto.DashboardHeatmapRsp], error) {
	cells, err := h.dashboard.Heatmap(ctx, dashboardScope(ctx), req.StartTime, req.EndTime, req.TZ)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Dashboard heatmap failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DashboardHeatmapRsp{Data: auditport.ToDashboardHeatmapDTO(cells)}, nil)
}
