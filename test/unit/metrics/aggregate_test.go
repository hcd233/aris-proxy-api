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

func TestAggregate_TokenRateAndSuccessRate(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	// 桶内两份快照：token 输入 0→120、输出 0→30；请求 0→2（成功 1）——每 pod 相同，跨 2 pod 聚合。
	instanceSnaps := []metrics.Snapshot{
		{TS: 0, TokenInput: 0, TokenOutput: 0, ReqTotal: 0, ReqSuccess: 0},
		{TS: 30, TokenInput: 120, TokenOutput: 30, ReqTotal: 2, ReqSuccess: 1},
	}
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": instanceSnaps,
		"pod-b": instanceSnaps,
	}

	got := metricsquery.Aggregate(byInstance, 0, bucket, 60, 0)

	// 输入速率：每 pod 120/60=2 tokens/s，跨 2 pod = 4
	if len(got.TokenInput) == 0 || got.TokenInput[0].Value != 4 {
		t.Errorf("expected tokenInput rate 4, got %+v", got.TokenInput)
	}
	// 输出速率：每 pod 30/60=0.5，跨 2 pod = 1
	if len(got.TokenOutput) == 0 || got.TokenOutput[0].Value != 1 {
		t.Errorf("expected tokenOutput rate 1, got %+v", got.TokenOutput)
	}
	// 成功率：跨 pod 合并 reqSuccess=2 / reqTotal=4 = 50%
	if len(got.SuccessRate) == 0 || got.SuccessRate[0].Value != 50 {
		t.Errorf("expected successRate 50, got %+v", got.SuccessRate)
	}
}

func TestAggregate_SuccessRateSkipsEmptyBucket(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	// 桶内有快照但无任何请求（reqTotal=0），successRate 不应输出 0% 误导点。
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": {
			{TS: 0, ReqTotal: 0, ReqSuccess: 0},
			{TS: 30, ReqTotal: 0, ReqSuccess: 0},
		},
	}
	got := metricsquery.Aggregate(byInstance, 0, bucket, 60, 0)
	if len(got.SuccessRate) != 0 {
		t.Errorf("expected empty successRate for empty bucket, got %+v", got.SuccessRate)
	}
}
