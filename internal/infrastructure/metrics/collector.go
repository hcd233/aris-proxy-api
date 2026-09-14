package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

// HTTPCollector 自实现的 HTTP 运行时指标采集器（替代 fiberprometheus）。
//
// 维护两项进程内指标：请求时延 histogram（请求总量 QPS 由 histogram 的 sample count 派生），
// 以及请求状态码 counter（各 HTTP 状态码的请求数，状态码面板数据源）。
//
//	@author centonhuang
//	@update 2026-09-14 10:00:00
type HTTPCollector struct {
	duration prometheus.Histogram
	requests *prometheus.CounterVec
	skipURIs map[string]struct{}
}

// NewHTTPCollector 在 registry 上注册 HTTP 指标并返回采集器
//
//	@param registry *prometheus.Registry
//	@return *HTTPCollector
//	@author centonhuang
//	@update 2026-09-14 10:00:00
func NewHTTPCollector(registry *prometheus.Registry) *HTTPCollector {
	duration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestDuration,
		Help:      constant.MetricRequestDurationHelp,
		Buckets:   constant.PrometheusRequestDurationBuckets,
	})
	registry.MustRegister(duration)

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceHTTP,
			Name:      constant.MetricNameRequests,
			Help:      constant.MetricRequestsHelp,
		},
		[]string{constant.MetricLabelStatusCode},
	)
	registry.MustRegister(requests)
	// 状态码集合无法枚举，不做子序列预置：无流量时该 metric family 不出现在 Gather() 结果中，
	// 快照 reqStatus 为空，聚合层自然不输出状态码曲线。

	skip := lo.SliceToMap(
		[]string{constant.RoutePathHealth, constant.RoutePathReady, constant.RoutePathSSEHealth, constant.RoutePathMetrics},
		func(p string) (string, struct{}) { return p, struct{}{} },
	)
	return &HTTPCollector{duration: duration, requests: requests, skipURIs: skip}
}

// Middleware 返回记录请求时延与状态码计数的 Fiber 中间件。
//
// 须全局挂载（app.Use(mw)）以覆盖所有业务路由；探活与指标路径被跳过。
//
//	@receiver hc *HTTPCollector
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-09-14 10:00:00
func (hc *HTTPCollector) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, skip := hc.skipURIs[c.Path()]; skip {
			return c.Next()
		}
		start := time.Now()
		err := c.Next()
		hc.duration.Observe(time.Since(start).Seconds())
		hc.requests.WithLabelValues(strconv.Itoa(c.Response().StatusCode())).Inc()
		return err
	}
}
