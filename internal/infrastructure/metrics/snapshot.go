package metrics

import (
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/metrics/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
	metricpb "github.com/prometheus/client_model/go"
)

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
func BuildSnapshot(gatherer prometheus.Gatherer, now time.Time) (*port.Snapshot, error) {
	families, err := gatherer.Gather()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]*metricpb.MetricFamily, len(families))
	for _, f := range families {
		byName[f.GetName()] = f
	}

	requests := byName[constant.MetricFullHTTPRequests]
	snap := &port.Snapshot{
		TS:          now.Unix(),
		Goroutines:  firstGaugeValue(byName[constant.MetricFullGoGoroutines]),
		Threads:     firstGaugeValue(byName[constant.MetricFullGoThreads]),
		HeapBytes:   firstGaugeValue(byName[constant.MetricFullGoHeapAlloc]),
		CPUSeconds:  firstCounterValue(byName[constant.MetricFullProcessCPU]),
		SSEActive:   labeledGaugeValues(byName[constant.MetricFullSSEActive], constant.MetricLabelProvider),
		TokenInput:  labeledCounterValue(byName[constant.MetricFullTokenUsage], constant.MetricLabelDirection, constant.TokenUsageDirectionInput),
		TokenOutput: labeledCounterValue(byName[constant.MetricFullTokenUsage], constant.MetricLabelDirection, constant.TokenUsageDirectionOutput),
		ReqStatus:   labeledCounterValues(requests, constant.MetricLabelStatusCode),
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

// labeledCounterValue 取指定 label 值对应的 counter 累计值；缺失时返回 0。
func labeledCounterValue(f *metricpb.MetricFamily, label, want string) float64 {
	if f == nil || len(f.GetMetric()) == 0 {
		return 0
	}
	for _, m := range f.GetMetric() {
		key := ""
		for _, l := range m.GetLabel() {
			if l.GetName() == label {
				key = l.GetValue()
				break
			}
		}
		if key == want {
			return m.GetCounter().GetValue()
		}
	}
	return 0
}

// labeledCounterValues 按 label 取全部子序列的 counter 累计值；无子序列时返回 nil。
func labeledCounterValues(f *metricpb.MetricFamily, label string) map[string]float64 {
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
		out[key] = m.GetCounter().GetValue()
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
