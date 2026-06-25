package metrics_test

import (
	"testing"

	metricsquery "github.com/hcd233/aris-proxy-api/internal/application/metrics/query"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
)

func TestAggregate_CrossPodSumRateAndP95(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	const alignedStart int64 = 0
	const end int64 = 60
	const outputStart int64 = 0

	// 桶内两份快照：goroutine 10→20、LatCount 0→60、CPU 0→6s、histogram le=0.1 0→60
	instanceSnaps := []metrics.Snapshot{
		{TS: 0, Goroutines: 10, LatCount: 0, CPUSeconds: 0, LatBuckets: map[string]float64{"0.1": 0}},
		{TS: 30, Goroutines: 20, LatCount: 60, CPUSeconds: 6, LatBuckets: map[string]float64{"0.1": 60}},
	}
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": instanceSnaps,
		"pod-b": instanceSnaps,
	}

	got := metricsquery.Aggregate(byInstance, alignedStart, bucket, end, outputStart)

	if len(got.Goroutines) == 0 {
		t.Fatal("expected non-empty goroutines series")
	}
	// 第一个桶（t=0）：goroutine 桶内均值 15，跨 2 个 pod 求和 = 30
	if got.Goroutines[0].Value != 30 {
		t.Errorf("expected cross-pod goroutines 30, got %f", got.Goroutines[0].Value)
	}
	// QPS：每 pod 60/60s=1，跨 2 pod = 2
	if got.QPS[0].Value != 2 {
		t.Errorf("expected cross-pod qps 2, got %f", got.QPS[0].Value)
	}
	// CPU%：每 pod 6/60*100=10，跨 2 pod = 20
	if got.CPUPercent[0].Value != 20 {
		t.Errorf("expected cross-pod cpu%% 20, got %f", got.CPUPercent[0].Value)
	}
	// P95：跨 pod 合并 bucket 后 total=120，le=0.1 → 100ms
	if got.P95Ms[0].Value != 100 {
		t.Errorf("expected p95 100ms, got %f", got.P95Ms[0].Value)
	}
}

func TestAggregate_CounterResetClamped(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	// LatCount 100→0（pod 重启），负 delta 应被 clamp 为 0
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": {
			{TS: 0, LatCount: 100},
			{TS: 30, LatCount: 0},
		},
	}
	got := metricsquery.Aggregate(byInstance, 0, bucket, 60, 0)
	if got.QPS[0].Value != 0 {
		t.Errorf("expected qps 0 after reset, got %f", got.QPS[0].Value)
	}
}
