// Package metrics 运行时指标采集基础设施
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
package metrics

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// SSEGauge SSE 并发连接数指标
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type SSEGauge struct {
	gauge *prometheus.GaugeVec
}

func (g *SSEGauge) Inc(provider string) {
	g.gauge.WithLabelValues(provider).Inc()
}

func (g *SSEGauge) Dec(provider string) {
	g.gauge.WithLabelValues(provider).Dec()
}

// NewRegistry 创建 Prometheus Registry，并注册 Go runtime / process 默认采集器。
//
// 默认采集器提供 go_goroutines / go_memstats_alloc_bytes / process_cpu_seconds_total，
// 是运行时大盘的 goroutine / heap / CPU 数据来源。
//
//	@return *prometheus.Registry
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

// NewSSEGauge 在 Registry 上注册并返回 SSE gauge
//
//	@param registry *prometheus.Registry
//	@return *SSEGauge
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewSSEGauge(registry *prometheus.Registry) *SSEGauge {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: constant.MetricSSEActiveConnectionsName,
			Help: constant.MetricSSEActiveConnectionsHelp,
		},
		[]string{constant.MetricLabelProvider},
	)
	registry.MustRegister(gauge)
	// 预热已知 provider 的子序列：GaugeVec 在首次 WithLabelValues 之前不产生任何序列，
	// 否则在没有任何 SSE 流量时 Gather 不输出 sse_active_connections，前端 sseActive 恒为空 {}。
	// 预置为 0 后，快照始终携带各 provider，活跃流到来时自然抬升。
	for _, provider := range []string{constant.SSEProviderOpenAI, constant.SSEProviderAnthropic} {
		gauge.WithLabelValues(provider).Set(0)
	}
	return &SSEGauge{gauge: gauge}
}

// TokenUsageCounter LLM token 吞吐 counter（输入/输出两个方向）。
//
// 数据来源：recordModelCall 收尾处把每次模型调用的 usage（input/output）累加进来；
// 聚合层对相邻快照做正向 delta ÷ 桶宽得到 tokens/sec 速率。
//
//	@author centonhuang
//	@update 2026-08-01 10:00:00
type TokenUsageCounter struct {
	counter *prometheus.CounterVec
}

// AddInput 累加输入 token。
//
//	@receiver c *TokenUsageCounter
//	@param n int64
//	@author centonhuang
//	@update 2026-08-01 10:00:00
func (c *TokenUsageCounter) AddInput(n int64) {
	if n > 0 {
		c.counter.WithLabelValues(constant.TokenUsageDirectionInput).Add(float64(n))
	}
}

// AddOutput 累加输出 token。
//
//	@receiver c *TokenUsageCounter
//	@param n int64
//	@author centonhuang
//	@update 2026-08-01 10:00:00
func (c *TokenUsageCounter) AddOutput(n int64) {
	if n > 0 {
		c.counter.WithLabelValues(constant.TokenUsageDirectionOutput).Add(float64(n))
	}
}

// NewTokenUsageCounter 在 Registry 上注册并返回 token 吞吐 counter。
//
// 预置 input/output 两个 label 序列（与 SSEGauge 同因：避免无流量时 Gather 不输出）。
//
//	@param registry *prometheus.Registry
//	@return *TokenUsageCounter
//	@author centonhuang
//	@update 2026-08-01 10:00:00
func NewTokenUsageCounter(registry *prometheus.Registry) *TokenUsageCounter {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: constant.MetricNamespaceLLM,
			Name:      constant.MetricNameTokenUsage,
			Help:      constant.MetricTokenUsageHelp,
		},
		[]string{constant.MetricLabelDirection},
	)
	registry.MustRegister(counter)
	for _, direction := range []string{constant.TokenUsageDirectionInput, constant.TokenUsageDirectionOutput} {
		counter.WithLabelValues(direction).Add(0)
	}
	return &TokenUsageCounter{counter: counter}
}
