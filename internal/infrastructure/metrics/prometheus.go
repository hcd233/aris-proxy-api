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
