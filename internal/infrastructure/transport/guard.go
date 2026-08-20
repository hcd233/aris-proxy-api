package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointKey 生成上游 endpoint 的熔断/隔离 key：BaseURL|APIKey。
// 同源（同 baseURL 同 key）的不同模型共享熔断状态与并发限制。
//
//	@param ep vo.UpstreamEndpoint
//	@return string
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func EndpointKey(ep vo.UpstreamEndpoint) string {
	return ep.BaseURL + "|" + ep.APIKey
}

// IsCircuitError 判断上游错误是否计入熔断失败。
//
// 计入：网络层错误（*model.UpstreamConnectionError）、5xx（*model.UpstreamError StatusCode >= 500）。
// 不计入：429（上游存活仅限流）、其他 4xx、本地构建错误。
//
//	@param err error
//	@return bool
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func IsCircuitError(err error) bool {
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return true
	}
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode >= http.StatusInternalServerError
	}
	return false
}

// EndpointGuard proxy 链路的容错守卫：熔断 + 信号量组合，对上游 endpoint 维度生效。
type EndpointGuard struct {
	guard *resilience.Guard
}

// NewEndpointGuard 从 config 全局变量组装通用 Guard，并把指标注册到 registry。
// registry 为 nil 时跳过指标注册（测试/无指标环境）。
//
//	@param registry *prometheus.Registry
//	@return *EndpointGuard
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func NewEndpointGuard(registry *prometheus.Registry) *EndpointGuard {
	var m resilience.Metrics
	if registry != nil {
		m = newGuardMetrics(registry)
	}
	cfg := resilience.GuardConfig{
		CircuitEnabled:             config.UpstreamCircuitEnabled,
		CircuitWindow:              config.UpstreamCircuitWindow,
		CircuitMinRequests:         config.UpstreamCircuitMinRequests,
		CircuitErrorThreshold:      config.UpstreamCircuitErrorThreshold,
		CircuitOpenTimeout:         config.UpstreamCircuitOpenTimeout,
		CircuitHalfOpenMaxRequests: config.UpstreamCircuitHalfOpenMaxRequests,
		BulkheadEnabled:            config.UpstreamBulkheadEnabled,
		BulkheadMaxConcurrent:      config.UpstreamBulkheadMaxConcurrent,
		BulkheadAcquireTimeout:     config.UpstreamBulkheadAcquireTimeout,
	}
	return &EndpointGuard{guard: resilience.NewGuard(cfg, m)}
}

// Allow 放行则返回幂等 release；熔断打开/满载返回错误。
//
//	@param ctx context.Context
//	@param key string
//	@return release func()
//	@return err error
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *EndpointGuard) Allow(ctx context.Context, key string) (func(), error) {
	return g.guard.Allow(ctx, key)
}

// Report 上报一次调用结果。
//
//	@param key string
//	@param success bool
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func (g *EndpointGuard) Report(key string, success bool) {
	g.guard.Report(key, success)
}

// guardMetrics resilience.Metrics 的 Prometheus 实现。
type guardMetrics struct {
	state    *prometheus.GaugeVec
	open     *prometheus.CounterVec
	rejected *prometheus.CounterVec
	bulkhead *prometheus.CounterVec
}

func newGuardMetrics(registry *prometheus.Registry) *guardMetrics {
	m := &guardMetrics{
		state: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitStateName,
			Help:      constant.MetricUpstreamCircuitStateHelp,
		}, []string{constant.MetricLabelKey}),
		open: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitOpenTotalName,
			Help:      constant.MetricUpstreamCircuitOpenTotalHelp,
		}, []string{constant.MetricLabelKey}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamCircuitRejectedTotalName,
			Help:      constant.MetricUpstreamCircuitRejectedTotalHelp,
		}, []string{constant.MetricLabelKey}),
		bulkhead: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceUpstream,
			Name:      constant.MetricUpstreamBulkheadRejectedTotalName,
			Help:      constant.MetricUpstreamBulkheadRejectedTotalHelp,
		}, []string{constant.MetricLabelKey}),
	}
	registry.MustRegister(m.state, m.open, m.rejected, m.bulkhead)
	return m
}

func (m *guardMetrics) SetBreakerState(key string, state enum.BreakerState) {
	m.state.WithLabelValues(key).Set(float64(state))
}

func (m *guardMetrics) IncCircuitOpen(key string) {
	m.open.WithLabelValues(key).Inc()
}

func (m *guardMetrics) IncCircuitRejected(key string) {
	m.rejected.WithLabelValues(key).Inc()
}

func (m *guardMetrics) IncBulkheadRejected(key string) {
	m.bulkhead.WithLabelValues(key).Inc()
}
