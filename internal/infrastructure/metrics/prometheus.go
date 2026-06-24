// Package metrics Prometheus 指标采集基础设施
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
package metrics

import (
	fiberprometheus "github.com/gofiber/contrib/v3/prometheus"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
)

// SSEGauge SSE 并发连接数指标接口
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type SSEGauge interface {
	Inc(provider string)
	Dec(provider string)
}

type sseGauge struct {
	gauge *prometheus.GaugeVec
}

func (g *sseGauge) Inc(provider string) {
	g.gauge.WithLabelValues(provider).Inc()
}

func (g *sseGauge) Dec(provider string) {
	g.gauge.WithLabelValues(provider).Dec()
}

// NewRegistry 创建 Prometheus Registry
//
//	@return *prometheus.Registry
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// NewSSEGauge 在 Registry 上注册并返回 SSE gauge
//
//	@param registry *prometheus.Registry
//	@return SSEGauge
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewSSEGauge(registry *prometheus.Registry) SSEGauge {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: constant.MetricSSEActiveConnectionsName,
			Help: constant.MetricSSEActiveConnectionsHelp,
		},
		[]string{constant.MetricLabelProvider},
	)
	registry.MustRegister(gauge)
	return &sseGauge{gauge: gauge}
}

// NewMiddleware 创建 fiberprometheus 中间件
//
//	@param registry *prometheus.Registry
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewMiddleware(registry *prometheus.Registry) fiber.Handler {
	return fiberprometheus.New(fiberprometheus.Config{
		Service:                constant.MetricServiceName,
		Namespace:              constant.MetricNamespace,
		Registerer:             registry,
		Gatherer:               registry,
		RequestDurationBuckets: constant.PrometheusRequestDurationBuckets,
		SkipURIs: []string{
			constant.RoutePathHealth,
			constant.RoutePathReady,
			constant.RoutePathSSEHealth,
		},
	})
}
