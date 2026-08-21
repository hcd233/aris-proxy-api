package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/resilience"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointKey 生成上游 endpoint 的熔断/隔离 key：BaseURL|Hash(APIKey)。
// 同源（同 baseURL 同 key）的不同模型共享熔断状态与并发限制。
// APIKey 经 SHA-256 截断哈希后才进入 key：该 key 会作为 Prometheus 指标 label
// 与 CircuitOpenError 错误消息对外暴露，明文写入会造成凭据泄漏。
//
//	@param ep vo.UpstreamEndpoint
//	@return string
//	@author centonhuang
//	@update 2026-08-21 10:00:00
func EndpointKey(ep vo.UpstreamEndpoint) string {
	sum := sha256.Sum256([]byte(ep.APIKey))
	return ep.BaseURL + "|" + hex.EncodeToString(sum[:])[:8]
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

// GuardLease 容错租约：持有 bulkhead 槽位与待上报的熔断结果。
// 流式请求的槽位占用与熔断上报延迟到 body 消费完成（Close）时执行，
// 保证慢流真实占用上游连接期间不超卖并发槽，且流中断计入熔断窗口。
type GuardLease struct {
	g       *EndpointGuard
	key     string
	release func()
	once    sync.Once
}

// Done 结束租约：释放 bulkhead 槽位并按 success 上报熔断结果。幂等，可安全 defer。
func (l *GuardLease) Done(success bool) {
	if l == nil {
		return
	}
	l.once.Do(func() {
		l.g.Report(l.key, success)
		l.release()
	})
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

// Allow 获取容错租约：先过熔断再取信号量；放行返回租约，拒绝返回熔断/满载错误。
// 租约须在请求生命周期结束时经 lease.Done(success) 结束（见 BindLease）。
//
//	@param ctx context.Context
//	@param key string
//	@return *GuardLease 容错租约（拒绝时为 nil）
//	@return err error
//	@author centonhuang
//	@update 2026-08-21 10:00:00
func (g *EndpointGuard) Allow(ctx context.Context, key string) (*GuardLease, error) {
	release, err := g.guard.Allow(ctx, key)
	if err != nil {
		return nil, err
	}
	return &GuardLease{g: g, key: key, release: release}, nil
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

// BindLease 把容错租约绑定到响应 body 的生命周期：返回的 ReadCloser 在 Close 时
// 先关闭原始 body，再释放 bulkhead 槽位并按 success 上报熔断结果。
// 流式链路（SSE）的槽位占用与熔断上报因此覆盖整个 body 消费阶段：
// 慢流不超卖并发槽，流中断计入熔断窗口。非流式调用方读完 body 后 Close 即可。
//
//	@param body io.ReadCloser 上游响应 body（可为 nil，此时立即结束租约）
//	@param lease Allow 返回的租约（可为 nil，原样返回 body）
//	@param success bool 调用是否成功（由调用方按业务判定）
//	@return io.ReadCloser 绑定租约的 body
//	@author centonhuang
//	@update 2026-08-21 10:00:00
func (g *EndpointGuard) BindLease(body io.ReadCloser, lease *GuardLease, success bool) io.ReadCloser {
	if lease == nil {
		return body
	}
	if body == nil {
		lease.Done(success)
		return nil
	}
	return &leaseBoundBody{ReadCloser: body, lease: lease, success: success}
}

// leaseBoundBody Close 时结束容错租约的响应 body 包装。
type leaseBoundBody struct {
	io.ReadCloser
	lease   *GuardLease
	success bool
}

func (b *leaseBoundBody) Close() error {
	err := b.ReadCloser.Close()
	b.lease.Done(b.success)
	return err
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
