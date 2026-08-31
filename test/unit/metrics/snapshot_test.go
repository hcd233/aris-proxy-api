package metrics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	metricspkg "github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestBuildSnapshot_ExtractsRuntimeMetrics(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()

	duration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: constant.MetricNamespaceHTTP,
		Name:      constant.MetricNameRequestDuration,
		Buckets:   constant.PrometheusRequestDurationBuckets,
	})
	registry.MustRegister(duration)
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

func TestBuildSnapshot_ExtractsGoThreads(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	threads := prometheus.NewGauge(prometheus.GaugeOpts{Name: constant.MetricFullGoThreads})
	registry.MustRegister(threads)
	threads.Set(37)

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if snap.Threads != 37 {
		t.Errorf("expected threads 37, got %f", snap.Threads)
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

func TestTokenUsageCounter_AddAndSnapshot(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	counter := metricspkg.NewTokenUsageCounter(registry)

	counter.AddInput(10)
	counter.AddOutput(25)

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if snap.TokenInput != 10 {
		t.Errorf("expected tokenInput 10, got %f", snap.TokenInput)
	}
	if snap.TokenOutput != 25 {
		t.Errorf("expected tokenOutput 25, got %f", snap.TokenOutput)
	}
}

func TestHTTPCollector_CountsSuccessAndFailure(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	collector := metricspkg.NewHTTPCollector(registry)

	app := fiber.New()
	app.Use(collector.Middleware())
	app.Get("/ok", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/bad", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusInternalServerError) })

	reqOK := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ok", http.NoBody)
	respOK, err := app.Test(reqOK, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request /ok failed: %v", err)
	}
	respOK.Body.Close()
	reqOK2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ok", http.NoBody)
	respOK2, err := app.Test(reqOK2, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request /ok failed: %v", err)
	}
	respOK2.Body.Close()
	reqBad := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/bad", http.NoBody)
	respBad, err := app.Test(reqBad, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request /bad failed: %v", err)
	}
	respBad.Body.Close()

	snap, err := metricspkg.BuildSnapshot(registry, time.Now())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if snap.ReqSuccess != 2 {
		t.Errorf("expected reqSuccess 2, got %f", snap.ReqSuccess)
	}
	if snap.ReqTotal != 3 {
		t.Errorf("expected reqTotal 3, got %f", snap.ReqTotal)
	}
}
