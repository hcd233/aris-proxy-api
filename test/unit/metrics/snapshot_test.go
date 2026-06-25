package metrics_test

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	metricspkg "github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestBuildSnapshot_ExtractsRuntimeMetrics(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()

	inProgress := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestsInProgress,
	})
	duration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestDuration,
		Buckets:   constant.PrometheusRequestDurationBuckets,
	})
	registry.MustRegister(inProgress, duration)
	inProgress.Set(3)
	duration.Observe(0.02)
	duration.Observe(0.2)

	sse := metricspkg.NewSSEGauge(registry)
	sse.Inc(constant.SSEProviderOpenAI)
	sse.Inc(constant.SSEProviderOpenAI)
	sse.Dec(constant.SSEProviderOpenAI)

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}

	if snap.InProgress != 3 {
		t.Errorf("expected inProgress 3, got %f", snap.InProgress)
	}
	if snap.LatCount != 2 {
		t.Errorf("expected latCount 2, got %f", snap.LatCount)
	}
	if len(snap.LatBuckets) == 0 {
		t.Error("expected non-empty latBuckets")
	}
	if snap.SSEActive[constant.SSEProviderOpenAI] != 1 {
		t.Errorf("expected sse openai 1, got %f", snap.SSEActive[constant.SSEProviderOpenAI])
	}
}

func TestSSEGauge_IncDec(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	sse := metricspkg.NewSSEGauge(registry)

	sse.Inc(constant.SSEProviderOpenAI)
	sse.Inc(constant.SSEProviderOpenAI)
	sse.Dec(constant.SSEProviderOpenAI)

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if snap.SSEActive[constant.SSEProviderOpenAI] != 1 {
		t.Errorf("expected 1 (2 inc - 1 dec), got %f", snap.SSEActive[constant.SSEProviderOpenAI])
	}
}
