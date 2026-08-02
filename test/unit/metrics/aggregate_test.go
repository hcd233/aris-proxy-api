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

	// QPS：每 pod 60/60s=1，跨 2 pod = 2（聚合指标仍保留）
	if got.QPS[0].Value != 2 {
		t.Errorf("expected cross-pod qps 2, got %f", got.QPS[0].Value)
	}
	// P95：跨 pod 合并 bucket 后 total=120，le=0.1 → 100ms
	if got.P95Ms[0].Value != 100 {
		t.Errorf("expected p95 100ms, got %f", got.P95Ms[0].Value)
	}
	// per-instance：goroutines 桶内均值 (10+20)/2=15，不跨 pod 求和
	if got.Instances["pod-a"].Goroutines[0].Value != 15 {
		t.Errorf("expected per-pod goroutines 15, got %f", got.Instances["pod-a"].Goroutines[0].Value)
	}
	// per-instance：CPU% 6/60s*100=10（单 pod 独立 0-100）
	if got.Instances["pod-a"].CPUPercent[0].Value != 10 {
		t.Errorf("expected per-pod cpu%% 10, got %f", got.Instances["pod-a"].CPUPercent[0].Value)
	}
	// 两个 pod 都有独立曲线
	if _, ok := got.Instances["pod-b"]; !ok {
		t.Error("expected instance pod-b to be present")
	}
}

func TestAggregate_InstancesPerPodSkipsEmptyBucket(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	const alignedStart int64 = 0
	const end int64 = 180
	const outputStart int64 = 0

	// pod-a：桶0（t=0..60）两份快照；桶1（t=60..120）无快照（抖动丢点）；桶2（t=120..180）一份快照。
	// pod-z：仅桶0 有快照。
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": {
			{TS: 0, Goroutines: 10},
			{TS: 30, Goroutines: 20},
			{TS: 150, Goroutines: 30},
		},
		"pod-z": {
			{TS: 0, Goroutines: 100},
			{TS: 30, Goroutines: 100},
		},
	}

	got := metricsquery.Aggregate(byInstance, alignedStart, bucket, end, outputStart)

	a := got.Instances["pod-a"]
	if len(a.Goroutines) != 2 {
		t.Fatalf("expected 2 goroutines points for pod-a, got %+v", a.Goroutines)
	}
	// 桶0 均值 (10+20)/2=15；桶1 无快照被跳过；桶2 单快照原值 30
	if a.Goroutines[0].Value != 15 || a.Goroutines[0].Time != 0 {
		t.Errorf("expected first point 15@0, got %+v", a.Goroutines[0])
	}
	if a.Goroutines[1].Value != 30 || a.Goroutines[1].Time != 120 {
		t.Errorf("expected second point 30@120, got %+v", a.Goroutines[1])
	}
	if _, ok := got.Instances["pod-z"]; ok {
		t.Error("expected stale instance pod-z to be skipped")
	}
}

func TestAggregate_StaleInstanceSkipped(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	const end int64 = 600
	const outputStart int64 = 0

	// pod-live：最后快照就在 end 附近，视为在线，应输出。
	// pod-dead：最后快照停在 120s（距 end 超过 2 个桶），视为已下线，应跳过。
	byInstance := map[string][]metrics.Snapshot{
		"pod-live": {
			{TS: 0, Goroutines: 10},
			{TS: 540, Goroutines: 20},
			{TS: 570, Goroutines: 30},
		},
		"pod-dead": {
			{TS: 0, Goroutines: 100},
			{TS: 60, Goroutines: 110},
			{TS: 120, Goroutines: 120},
		},
	}

	got := metricsquery.Aggregate(byInstance, 0, bucket, end, outputStart)

	if _, ok := got.Instances["pod-live"]; !ok {
		t.Error("expected live instance pod-live to be present")
	}
	if _, ok := got.Instances["pod-dead"]; ok {
		t.Error("expected stale instance pod-dead to be skipped")
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

func TestAggregate_SuccessRateRoundsRepeatingDecimal(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	// 1/3 成功率：round2 必须把 33.333333333333336 收敛为 33.33，
	// 否则前端拿到 33.33333333333333 这类尾差值直接渲染。
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": {
			{TS: 0, ReqTotal: 0, ReqSuccess: 0},
			{TS: 30, ReqTotal: 3, ReqSuccess: 1},
		},
	}
	got := metricsquery.Aggregate(byInstance, 0, bucket, 60, 0)
	if len(got.SuccessRate) == 0 || got.SuccessRate[0].Value != 33.33 {
		t.Errorf("expected successRate 33.33 (rounded), got %+v", got.SuccessRate)
	}
}
