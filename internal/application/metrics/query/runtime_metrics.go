// Package query 运行时指标聚合查询
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
package query

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/metrics/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
)

// SnapshotReader 运行时快照读取能力（由 cache.RuntimeMetricsCache 实现）。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type SnapshotReader interface {
	ListInstances(ctx context.Context, sinceUnix int64) ([]string, error)
	ReadSnapshots(ctx context.Context, instanceID string, startUnix, endUnix int64) ([][]byte, error)
}

type runtimeMetricsHandler struct {
	reader SnapshotReader
}

// NewRuntimeMetricsHandler 创建运行时指标聚合查询服务
//
//	@param reader SnapshotReader
//	@return port.RuntimeMetricsService
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewRuntimeMetricsHandler(reader SnapshotReader) port.RuntimeMetricsService {
	return &runtimeMetricsHandler{reader: reader}
}

func (h *runtimeMetricsHandler) RuntimeMetrics(ctx context.Context, rangeKey string, since int64) (dto.RuntimeSeries, int64, error) {
	rw := metrics.ResolveRange(rangeKey)
	now := time.Now()
	end := now.Unix()
	bucket := int64(rw.Bucket.Seconds())
	windowStart := now.Add(-rw.Window).Unix()

	// 增量：since 之后只重算尾部，多回溯一个桶以刷新未封口的桶
	effectiveStart := windowStart
	if since > windowStart {
		effectiveStart = since - bucket
	}
	if effectiveStart < windowStart {
		effectiveStart = windowStart
	}
	alignedStart := effectiveStart - mod(effectiveStart, bucket)
	outputStart := alignedStart
	if since > 0 {
		outputStart = since - mod(since, bucket)
	}

	instances, err := h.reader.ListInstances(ctx, windowStart)
	if err != nil {
		return dto.RuntimeSeries{}, 0, err
	}

	byInstance := make(map[string][]metrics.Snapshot, len(instances))
	for _, inst := range instances {
		payloads, readErr := h.reader.ReadSnapshots(ctx, inst, alignedStart, end)
		if readErr != nil {
			return dto.RuntimeSeries{}, 0, readErr
		}
		byInstance[inst] = decodeSnapshots(payloads)
	}

	series := Aggregate(byInstance, alignedStart, bucket, end, outputStart)
	latest := end - mod(end, bucket)
	return series, latest, nil
}

// Aggregate 把各 instance 的快照按桶聚合成可展示时序：
// gauge 取桶内均值后跨 instance 求和；counter 取相邻快照正向 delta（reset 清零）求速率后跨 instance 求和；
// histogram 取相邻快照各 le 的正向 delta 后跨 instance 合并、求 P95。只返回时间 >= outputStart 的桶。
//
//	@param byInstance map[string][]metrics.Snapshot
//	@param alignedStart int64 对齐到桶边界的起始 unix 秒
//	@param bucket int64 桶宽秒
//	@param end int64 结束 unix 秒
//	@param outputStart int64 输出下界 unix 秒
//	@return dto.RuntimeSeries
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func Aggregate(byInstance map[string][]metrics.Snapshot, alignedStart, bucket, end, outputStart int64) dto.RuntimeSeries {
	n := int((end-alignedStart)/bucket) + 1
	if n <= 0 {
		return emptySeries()
	}
	agg := newBucketAggs(n)
	for _, snaps := range byInstance {
		if len(snaps) == 0 {
			continue
		}
		accumulateInstance(agg, snaps, alignedStart, bucket, n)
	}
	return buildSeries(agg, alignedStart, bucket, outputStart)
}

// — 聚合内部结构与算法 —

type bucketAgg struct {
	goroutines  float64
	heap        float64
	inProgress  float64
	sse         map[string]float64
	qps         float64
	cpuPercent  float64
	histBuckets map[string]float64
	histTotal   float64
}

func newBucketAggs(n int) []bucketAgg {
	aggs := make([]bucketAgg, n)
	for i := range aggs {
		aggs[i].sse = map[string]float64{}
		aggs[i].histBuckets = map[string]float64{}
	}
	return aggs
}

func decodeSnapshots(payloads [][]byte) []metrics.Snapshot {
	snaps := make([]metrics.Snapshot, 0, len(payloads))
	for _, p := range payloads {
		var s metrics.Snapshot
		if err := sonic.Unmarshal(p, &s); err != nil {
			continue
		}
		snaps = append(snaps, s)
	}
	return snaps
}

func accumulateInstance(agg []bucketAgg, snaps []metrics.Snapshot, alignedStart, bucket int64, n int) {
	gSum, gHeap, gInProg, gSSE, gCount := instanceGauges(snaps, alignedStart, bucket, n)
	dCount, dCPU, dHist := instanceDeltas(snaps, alignedStart, bucket, n)

	bucketSeconds := float64(bucket)
	for idx := range n {
		mergeGaugeBucket(&agg[idx], gSum[idx], gHeap[idx], gInProg[idx], gSSE[idx], gCount[idx])
		mergeRateBucket(&agg[idx], dCount[idx], dCPU[idx], dHist[idx], bucketSeconds)
	}
}

// instanceGauges 按桶累加单实例的 gauge 原值与计数（用于后续求桶内均值）。
func instanceGauges(snaps []metrics.Snapshot, alignedStart, bucket int64, n int) (gSum, gHeap, gInProg []float64, gSSE []map[string]float64, gCount []float64) {
	gSum = make([]float64, n)
	gHeap = make([]float64, n)
	gInProg = make([]float64, n)
	gSSE = make([]map[string]float64, n)
	gCount = make([]float64, n)
	for _, s := range snaps {
		idx := int((s.TS - alignedStart) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		gSum[idx] += s.Goroutines
		gHeap[idx] += s.HeapBytes
		gInProg[idx] += s.InProgress
		gCount[idx]++
		if gSSE[idx] == nil {
			gSSE[idx] = map[string]float64{}
		}
		for prov, v := range s.SSEActive {
			gSSE[idx][prov] += v
		}
	}
	return gSum, gHeap, gInProg, gSSE, gCount
}

// instanceDeltas 按桶累加单实例相邻快照的正向 delta（速率与 histogram），归属到后一个快照所在的桶。
func instanceDeltas(snaps []metrics.Snapshot, alignedStart, bucket int64, n int) (dCount, dCPU []float64, dHist []map[string]float64) {
	dCount = make([]float64, n)
	dCPU = make([]float64, n)
	dHist = make([]map[string]float64, n)
	for i := 1; i < len(snaps); i++ {
		prev, cur := snaps[i-1], snaps[i]
		idx := int((cur.TS - alignedStart) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		dCount[idx] += nonNeg(cur.LatCount - prev.LatCount)
		dCPU[idx] += nonNeg(cur.CPUSeconds - prev.CPUSeconds)
		if dHist[idx] == nil {
			dHist[idx] = map[string]float64{}
		}
		for le, cum := range cur.LatBuckets {
			dHist[idx][le] += nonNeg(cum - prev.LatBuckets[le])
		}
	}
	return dCount, dCPU, dHist
}

// mergeGaugeBucket 把单实例某桶的 gauge 桶内均值跨实例累加进全局桶。
func mergeGaugeBucket(b *bucketAgg, sum, heap, inProg float64, sse map[string]float64, count float64) {
	if count <= 0 {
		return
	}
	b.goroutines += sum / count
	b.heap += heap / count
	b.inProgress += inProg / count
	for prov, v := range sse {
		b.sse[prov] += v / count
	}
}

// mergeRateBucket 把单实例某桶的速率与 histogram delta 跨实例累加进全局桶。
func mergeRateBucket(b *bucketAgg, dCount, dCPU float64, dHist map[string]float64, bucketSeconds float64) {
	if bucketSeconds > 0 {
		b.qps += dCount / bucketSeconds
		b.cpuPercent += dCPU / bucketSeconds * constant.RuntimeMetricsPercentToRatio
	}
	b.histTotal += dCount
	for le, d := range dHist {
		b.histBuckets[le] += d
	}
}

func buildSeries(agg []bucketAgg, alignedStart, bucket, outputStart int64) dto.RuntimeSeries {
	series := emptySeries()
	providers := collectProviders(agg)

	for idx := range agg {
		t := alignedStart + int64(idx)*bucket
		if t < outputStart {
			continue
		}
		series.Goroutines = append(series.Goroutines, dto.RuntimePoint{Time: t, Value: round2(agg[idx].goroutines)})
		series.HeapMB = append(series.HeapMB, dto.RuntimePoint{Time: t, Value: round2(agg[idx].heap / constant.RuntimeMetricsBytesPerMB)})
		series.InProgress = append(series.InProgress, dto.RuntimePoint{Time: t, Value: round2(agg[idx].inProgress)})
		series.QPS = append(series.QPS, dto.RuntimePoint{Time: t, Value: round2(agg[idx].qps)})
		series.CPUPercent = append(series.CPUPercent, dto.RuntimePoint{Time: t, Value: round2(agg[idx].cpuPercent)})
		series.P95Ms = append(series.P95Ms, dto.RuntimePoint{Time: t, Value: round2(percentileP95(agg[idx].histBuckets, agg[idx].histTotal))})
		for _, prov := range providers {
			series.SSEActive[prov] = append(series.SSEActive[prov], dto.RuntimePoint{Time: t, Value: round2(agg[idx].sse[prov])})
		}
	}
	return series
}

func collectProviders(agg []bucketAgg) []string {
	set := map[string]struct{}{}
	for i := range agg {
		for prov := range agg[i].sse {
			set[prov] = struct{}{}
		}
	}
	providers := make([]string, 0, len(set))
	for prov := range set {
		providers = append(providers, prov)
	}
	sort.Strings(providers)
	return providers
}

func percentileP95(buckets map[string]float64, total float64) float64 {
	if total <= 0 || len(buckets) == 0 {
		return 0
	}
	type lePoint struct {
		le    float64
		count float64
	}
	points := make([]lePoint, 0, len(buckets))
	for le, c := range buckets {
		v, err := strconv.ParseFloat(le, constant.ParseFloat64BitSize)
		if err != nil {
			continue
		}
		points = append(points, lePoint{le: v, count: c})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].le < points[j].le })

	target := total * constant.RuntimeMetricsP95Percentile
	for _, p := range points {
		if p.count >= target {
			return p.le * constant.RuntimeMetricsMsPerSecond
		}
	}
	return 0
}

func emptySeries() dto.RuntimeSeries {
	return dto.RuntimeSeries{SSEActive: map[string][]dto.RuntimePoint{}}
}

func nonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

func mod(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

func round2(v float64) float64 {
	return float64(int64(v*constant.RuntimeMetricsRoundScale+constant.RuntimeMetricsRoundHalf)) / constant.RuntimeMetricsRoundScale
}
