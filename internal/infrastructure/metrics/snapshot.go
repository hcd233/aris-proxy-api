package metrics

import (
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
	metricpb "github.com/prometheus/client_model/go"
)

// Snapshot 单个 instance 在某一时刻的运行时指标快照（写入 Redis 的最小单位）。
//
// 仅存"可直接相加的原值"：gauge 原值 + counter 累计值 + histogram 桶计数；
// 速率与分位的计算全部留给聚合层。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type Snapshot struct {
	TS         int64              `json:"ts"`                   // unix 秒
	Goroutines float64            `json:"goroutines"`           // gauge
	HeapBytes  float64            `json:"heapBytes"`            // gauge
	CPUSeconds float64            `json:"cpuSeconds"`           // counter 累计值 → 聚合层求 CPU%
	SSEActive  map[string]float64 `json:"sseActive,omitempty"`  // provider -> gauge
	LatBuckets map[string]float64 `json:"latBuckets,omitempty"` // le -> 累计计数 → 聚合层求 P95
	LatCount   float64            `json:"latCount"`             // histogram 累计样本数 → 聚合层求 QPS
}

// SnapshotStore flusher 写入快照所需的存储能力（由 cache.RuntimeMetricsCache 实现）。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type SnapshotStore interface {
	WriteSnapshot(instanceID string, score int64, payload []byte, retentionCutoff int64) error
}

// BuildSnapshot 从 Gatherer 采集当前所有运行时指标，组装成一份快照。
//
//	@param gatherer prometheus.Gatherer
//	@param now time.Time
//	@return *Snapshot
//	@return error
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func BuildSnapshot(gatherer prometheus.Gatherer, now time.Time) (*Snapshot, error) {
	families, err := gatherer.Gather()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]*metricpb.MetricFamily, len(families))
	for _, f := range families {
		byName[f.GetName()] = f
	}

	snap := &Snapshot{
		TS:         now.Unix(),
		Goroutines: firstGaugeValue(byName[constant.MetricFullGoGoroutines]),
		HeapBytes:  firstGaugeValue(byName[constant.MetricFullGoHeapAlloc]),
		CPUSeconds: firstCounterValue(byName[constant.MetricFullProcessCPU]),
		SSEActive:  labeledGaugeValues(byName[constant.MetricFullSSEActive], constant.MetricLabelProvider),
	}
	snap.LatBuckets, snap.LatCount = histogramBuckets(byName[constant.MetricFullRequestDuration])
	return snap, nil
}

func firstGaugeValue(f *metricpb.MetricFamily) float64 {
	if f == nil || len(f.GetMetric()) == 0 {
		return 0
	}
	return f.GetMetric()[0].GetGauge().GetValue()
}

func firstCounterValue(f *metricpb.MetricFamily) float64 {
	if f == nil || len(f.GetMetric()) == 0 {
		return 0
	}
	return f.GetMetric()[0].GetCounter().GetValue()
}

func labeledGaugeValues(f *metricpb.MetricFamily, label string) map[string]float64 {
	if f == nil || len(f.GetMetric()) == 0 {
		return nil
	}
	out := make(map[string]float64, len(f.GetMetric()))
	for _, m := range f.GetMetric() {
		key := ""
		for _, l := range m.GetLabel() {
			if l.GetName() == label {
				key = l.GetValue()
				break
			}
		}
		out[key] = m.GetGauge().GetValue()
	}
	return out
}

func histogramBuckets(f *metricpb.MetricFamily) (buckets map[string]float64, count float64) {
	if f == nil || len(f.GetMetric()) == 0 {
		return nil, 0
	}
	h := f.GetMetric()[0].GetHistogram()
	buckets = make(map[string]float64, len(h.GetBucket()))
	for _, b := range h.GetBucket() {
		le := strconv.FormatFloat(b.GetUpperBound(), 'g', -1, constant.ParseFloat64BitSize)
		buckets[le] = float64(b.GetCumulativeCount())
	}
	return buckets, float64(h.GetSampleCount())
}

// RangeWindow 单个预设 range 档对应的窗口长度与桶宽。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RangeWindow struct {
	Window time.Duration
	Bucket time.Duration
}

var rangeWindows = map[string]RangeWindow{
	constant.RuntimeMetricsRange15m: {Window: constant.RuntimeMetricsWindow15m, Bucket: constant.RuntimeMetricsBucket15m},
	constant.RuntimeMetricsRange1h:  {Window: constant.RuntimeMetricsWindow1h, Bucket: constant.RuntimeMetricsBucket1h},
	constant.RuntimeMetricsRange6h:  {Window: constant.RuntimeMetricsWindow6h, Bucket: constant.RuntimeMetricsBucket6h},
	constant.RuntimeMetricsRange24h: {Window: constant.RuntimeMetricsWindow24h, Bucket: constant.RuntimeMetricsBucket24h},
}

// ResolveRange 解析预设 range 档，非法时回退到 1h。
//
//	@param r string
//	@return RangeWindow
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func ResolveRange(r string) RangeWindow {
	if rw, ok := rangeWindows[r]; ok {
		return rw
	}
	return rangeWindows[constant.RuntimeMetricsRange1h]
}
