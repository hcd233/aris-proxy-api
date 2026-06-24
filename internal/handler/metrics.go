package handler

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// MetricsHandler 指标处理器
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsHandler interface {
	HandleGetMetricsJSON(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.MetricsJSONRsp], error)
}

// MetricsDependencies MetricsHandler 依赖项
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsDependencies struct {
	Registry *prometheus.Registry
}

type metricsHandler struct {
	gatherer prometheus.Gatherer
}

// NewMetricsHandler 创建指标处理器
//
//	@param deps MetricsDependencies
//	@return MetricsHandler
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewMetricsHandler(deps MetricsDependencies) MetricsHandler {
	return &metricsHandler{gatherer: deps.Registry}
}

// HandleGetMetricsJSON 获取 Prometheus 指标的 JSON 表示
//
//	@receiver h *metricsHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.MetricsJSONRsp]
//	@return error
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func (h *metricsHandler) HandleGetMetricsJSON(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.MetricsJSONRsp], error) {
	families, err := metrics.GatherMetricFamilies(h.gatherer)
	if err != nil {
		logger.WithCtx(ctx).Error("[MetricsHandler] Gather metrics failed", zap.Error(err))
		rsp := &dto.MetricsJSONRsp{}
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(&dto.MetricsJSONRsp{Metrics: families}, nil)
}
