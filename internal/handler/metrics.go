package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	metricsport "github.com/hcd233/aris-proxy-api/internal/application/metrics/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// MetricsHandler 运行时指标处理器
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type MetricsHandler interface {
	HandleGetRuntimeMetrics(ctx context.Context, req *dto.RuntimeMetricsReq) (*dto.HTTPResponse[*dto.RuntimeMetricsRsp], error)
}

// MetricsDependencies MetricsHandler 依赖项
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type MetricsDependencies struct {
	RuntimeMetrics metricsport.RuntimeMetricsService
}

type metricsHandler struct {
	runtime metricsport.RuntimeMetricsService
}

// NewMetricsHandler 创建指标处理器
//
//	@param deps MetricsDependencies
//	@return MetricsHandler
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewMetricsHandler(deps MetricsDependencies) MetricsHandler {
	return &metricsHandler{runtime: deps.RuntimeMetrics}
}

// HandleGetRuntimeMetrics 获取跨 pod 聚合后的运行时指标时序
//
//	@receiver h *metricsHandler
//	@param ctx context.Context
//	@param req *dto.RuntimeMetricsReq
//	@return *dto.HTTPResponse[*dto.RuntimeMetricsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (h *metricsHandler) HandleGetRuntimeMetrics(ctx context.Context, req *dto.RuntimeMetricsReq) (*dto.HTTPResponse[*dto.RuntimeMetricsRsp], error) {
	rsp := &dto.RuntimeMetricsRsp{}
	series, latest, err := h.runtime.RuntimeMetrics(ctx, req.Range, req.Since)
	if err != nil {
		logger.WithCtx(ctx).Error("[MetricsHandler] Query runtime metrics failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Series = series
	rsp.LatestTime = latest
	return apiutil.WrapHTTPResponse(rsp, nil)
}
