package metrics_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	metricspkg "github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestGatherMetricFamilies_Counter(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_counter", Help: "test help"},
		[]string{"method"},
	)
	registry.MustRegister(counter)
	counter.WithLabelValues("GET").Add(5)

	families, err := metricspkg.GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "test_counter" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("test_counter not found in gathered families")
	}
	if found.Type != "COUNTER" {
		t.Errorf("expected type 'COUNTER', got '%s'", found.Type)
	}
	if found.Help != "test help" {
		t.Errorf("expected help 'test help', got '%s'", found.Help)
	}
	if len(found.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(found.Samples))
	}
	if found.Samples[0].Value != 5 {
		t.Errorf("expected value 5, got %f", found.Samples[0].Value)
	}
	if found.Samples[0].Labels["method"] != "GET" {
		t.Errorf("expected label method=GET, got '%s'", found.Samples[0].Labels["method"])
	}
}

func TestGatherMetricFamilies_Gauge(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_gauge", Help: "gauge help"})
	registry.MustRegister(gauge)
	gauge.Set(42)

	families, err := metricspkg.GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "test_gauge" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("test_gauge not found")
	}
	if found.Type != "GAUGE" {
		t.Errorf("expected type 'GAUGE', got '%s'", found.Type)
	}
	if found.Samples[0].Value != 42 {
		t.Errorf("expected value 42, got %f", found.Samples[0].Value)
	}
}

func TestGatherMetricFamilies_EmptyRegistry(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	families, err := metricspkg.GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}
	if families == nil {
		t.Fatal("expected non-nil families slice")
	}
}

func TestSSEGauge_IncDec(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	sseGauge := metricspkg.NewSSEGauge(registry)

	sseGauge.Inc("openai")
	sseGauge.Inc("openai")
	sseGauge.Dec("openai")

	families, err := metricspkg.GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "sse_active_connections" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("sse_active_connections not found")
	}
	if found.Samples[0].Value != 1 {
		t.Errorf("expected value 1 (2 inc - 1 dec), got %f", found.Samples[0].Value)
	}
	if found.Samples[0].Labels["provider"] != "openai" {
		t.Errorf("expected label provider=openai, got '%s'", found.Samples[0].Labels["provider"])
	}
}
