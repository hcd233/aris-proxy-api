package metrics

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

// HTTPCollector 自实现的 HTTP 运行时指标采集器（替代 fiberprometheus）。
//
// 维护两项进程内指标：在途请求数 gauge 与请求时延 histogram。
// 请求总量 QPS 由 histogram 的 sample count 派生，无需独立 counter。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type HTTPCollector struct {
	inProgress prometheus.Gauge
	duration   prometheus.Histogram
	skipURIs   map[string]struct{}
}

// NewHTTPCollector 在 registry 上注册 HTTP 指标并返回采集器
//
//	@param registry *prometheus.Registry
//	@return *HTTPCollector
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewHTTPCollector(registry *prometheus.Registry) *HTTPCollector {
	inProgress := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestsInProgress,
		Help:      constant.MetricRequestsInProgressHelp,
	})
	duration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestDuration,
		Help:      constant.MetricRequestDurationHelp,
		Buckets:   constant.PrometheusRequestDurationBuckets,
	})
	registry.MustRegister(inProgress, duration)

	skip := lo.SliceToMap(
		[]string{constant.RoutePathHealth, constant.RoutePathReady, constant.RoutePathSSEHealth, constant.RoutePathMetrics},
		func(p string) (string, struct{}) { return p, struct{}{} },
	)
	return &HTTPCollector{inProgress: inProgress, duration: duration, skipURIs: skip}
}

// Middleware 返回记录在途请求数与请求时延的 Fiber 中间件。
//
// 须全局挂载（app.Use(mw)）以覆盖所有业务路由；探活与指标路径被跳过。
//
//	@receiver c *HTTPCollector
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (hc *HTTPCollector) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, skip := hc.skipURIs[c.Path()]; skip {
			return c.Next()
		}
		hc.inProgress.Inc()
		start := time.Now()
		err := c.Next()
		hc.duration.Observe(time.Since(start).Seconds())
		hc.inProgress.Dec()
		return err
	}
}
