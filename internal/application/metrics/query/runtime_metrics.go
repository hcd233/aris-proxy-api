// Package query 运行时指标聚合查询
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
package query

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

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
	// latest 取当前桶起点；无样本的桶在 buildSeries 中被跳过，故空的末桶不会塌成 0。
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
	series := buildSeries(agg, alignedStart, bucket, outputStart)
	series.Instances = aggregateInstances(byInstance, alignedStart, bucket, end, outputStart)
	return series
}

// — 聚合内部结构与算法 —

type bucketAgg struct {
	sse         map[string]float64
	qps         float64
	cpuPercent  float64
	histBuckets map[string]float64
	histTotal   float64
	tokenInput  float64 // 桶内累计输入 token delta（跨实例求和）→ 除以桶宽得速率
	tokenOutput float64 // 桶内累计输出 token delta（跨实例求和）
	reqTotal    float64 // 桶内累计请求 delta（跨实例求和）
	reqSuccess  float64 // 桶内累计 200 请求 delta（跨实例求和）
	samples     float64 // 桶内跨实例累计的快照数；为 0 表示该桶无数据，不应输出
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
	return lo.FilterMap(payloads, func(p []byte, _ int) (metrics.Snapshot, bool) {
		var s metrics.Snapshot
		if err := sonic.Unmarshal(p, &s); err != nil {
			return metrics.Snapshot{}, false
		}
		return s, true
	})
}

func accumulateInstance(agg []bucketAgg, snaps []metrics.Snapshot, alignedStart, bucket int64, n int) {
	_, _, gSSE, gCount := instanceGauges(snaps, alignedStart, bucket, n)
	deltas := instanceDeltas(snaps, alignedStart, bucket, n)

	bucketSeconds := float64(bucket)
	for idx := range n {
		mergeGaugeBucket(&agg[idx], gSSE[idx], gCount[idx])
		mergeRateBucket(&agg[idx], deltas[idx].count, deltas[idx].cpu, deltas[idx].hist, bucketSeconds)
		mergeTokenBucket(&agg[idx], deltas[idx].tokenIn, deltas[idx].tokenOut)
		mergeReqBucket(&agg[idx], deltas[idx].reqTotal, deltas[idx].reqSuccess)
	}
}

// instanceGauges 按桶累加单实例的 gauge 原值与计数（用于后续求桶内均值）。
func instanceGauges(snaps []metrics.Snapshot, alignedStart, bucket int64, n int) (gSum, gHeap []float64, gSSE []map[string]float64, gCount []float64) {
	gSum = make([]float64, n)
	gHeap = make([]float64, n)
	gSSE = make([]map[string]float64, n)
	gCount = make([]float64, n)
	for _, s := range snaps {
		idx := int((s.TS - alignedStart) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		gSum[idx] += s.Goroutines
		gHeap[idx] += s.HeapBytes
		gCount[idx]++
		if gSSE[idx] == nil {
			gSSE[idx] = map[string]float64{}
		}
		for prov, v := range s.SSEActive {
			gSSE[idx][prov] += v
		}
	}
	return gSum, gHeap, gSSE, gCount
}

// instanceDelta 单实例某桶的相邻快照正向 delta 汇总（速率与比例在跨实例合并后统一计算）。
type instanceDelta struct {
	count      float64
	cpu        float64
	hist       map[string]float64
	tokenIn    float64
	tokenOut   float64
	reqTotal   float64
	reqSuccess float64
}

// instanceDeltas 按桶累加单实例相邻快照的正向 delta（速率与 histogram），归属到后一个快照所在的桶。
func instanceDeltas(snaps []metrics.Snapshot, alignedStart, bucket int64, n int) []instanceDelta {
	deltas := make([]instanceDelta, n)
	for i := 1; i < len(snaps); i++ {
		prev, cur := snaps[i-1], snaps[i]
		idx := int((cur.TS - alignedStart) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		deltas[idx].count += nonNeg(cur.LatCount - prev.LatCount)
		deltas[idx].cpu += nonNeg(cur.CPUSeconds - prev.CPUSeconds)
		deltas[idx].tokenIn += nonNeg(cur.TokenInput - prev.TokenInput)
		deltas[idx].tokenOut += nonNeg(cur.TokenOutput - prev.TokenOutput)
		deltas[idx].reqTotal += nonNeg(cur.ReqTotal - prev.ReqTotal)
		deltas[idx].reqSuccess += nonNeg(cur.ReqSuccess - prev.ReqSuccess)
		if deltas[idx].hist == nil {
			deltas[idx].hist = map[string]float64{}
		}
		for le, cum := range cur.LatBuckets {
			deltas[idx].hist[le] += nonNeg(cum - prev.LatBuckets[le])
		}
	}
	return deltas
}

// mergeGaugeBucket 把单实例某桶的 SSE gauge 桶内均值跨实例累加进全局桶。
func mergeGaugeBucket(b *bucketAgg, sse map[string]float64, count float64) {
	if count <= 0 {
		return
	}
	b.samples += count
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

// mergeTokenBucket 把单实例某桶的 token delta 跨实例累加进全局桶（速率在 buildSeries 统一除以桶宽）。
func mergeTokenBucket(b *bucketAgg, dTokenIn, dTokenOut float64) {
	b.tokenInput += dTokenIn
	b.tokenOutput += dTokenOut
}

// mergeReqBucket 把单实例某桶的请求结果 delta 跨实例累加进全局桶（比例在 buildSeries 统一计算）。
func mergeReqBucket(b *bucketAgg, dReqTotal, dReqSuccess float64) {
	b.reqTotal += dReqTotal
	b.reqSuccess += dReqSuccess
}

func buildSeries(agg []bucketAgg, alignedStart, bucket, outputStart int64) dto.RuntimeSeries {
	series := emptySeries()
	providers := collectProviders(agg)
	bucketSeconds := float64(bucket)

	for idx := range agg {
		t := alignedStart + int64(idx)*bucket
		// t < outputStart：增量下界之前的桶不重复输出；
		// samples == 0：该桶无任何快照（含末尾未采集到的当前桶、抖动丢点的空桶），
		// 输出会让 gauge 塌成 0 形成断崖，故直接跳过。
		if t < outputStart || agg[idx].samples == 0 {
			continue
		}
		series.QPS = append(series.QPS, dto.RuntimePoint{Time: t, Value: round2(agg[idx].qps)})
		series.P95Ms = append(series.P95Ms, dto.RuntimePoint{Time: t, Value: round2(percentileP95(agg[idx].histBuckets, agg[idx].histTotal))})
		series.TokenInput = append(series.TokenInput, dto.RuntimePoint{Time: t, Value: round2(agg[idx].tokenInput / bucketSeconds)})
		series.TokenOutput = append(series.TokenOutput, dto.RuntimePoint{Time: t, Value: round2(agg[idx].tokenOutput / bucketSeconds)})
		// 无请求的桶不输出 successRate，避免 0% 误导。
		if agg[idx].reqTotal > 0 {
			series.SuccessRate = append(series.SuccessRate, dto.RuntimePoint{Time: t, Value: round2(agg[idx].reqSuccess / agg[idx].reqTotal * constant.RuntimeMetricsPercentToRatio)})
		}
		for _, prov := range providers {
			series.SSEActive[prov] = append(series.SSEActive[prov], dto.RuntimePoint{Time: t, Value: round2(agg[idx].sse[prov])})
		}
	}
	return series
}

// aggregateInstances 把每个 instance 的快照各自按桶聚合成独立曲线（不跨实例求和）：
// goroutines/heapMB 取桶内均值；cpuPercent 取桶内 cpu 正 delta ÷ 桶宽。实例按名称排序输出，
// 保证响应稳定。只输出"最近仍在产出快照"的活跃实例：实例注册表保留 24h，滚动发布遗留的
// 已下线实例在窗口内仍有历史快照，若照常输出，前端对头部集群总和求和时会把死实例计入，
// 造成数值虚高；最后一条快照距 end 超过 2 个桶的实例视为已下线，直接不输出。
func aggregateInstances(byInstance map[string][]metrics.Snapshot, alignedStart, bucket, end, outputStart int64) map[string]dto.RuntimeInstanceSeries {
	names := lo.Filter(lo.Keys(byInstance), func(n string, _ int) bool {
		snaps := byInstance[n]
		if len(snaps) == 0 {
			return false
		}
		return snaps[len(snaps)-1].TS >= end-2*bucket
	})
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	n := int((end-alignedStart)/bucket) + 1
	if n <= 0 {
		return nil
	}
	out := make(map[string]dto.RuntimeInstanceSeries, len(names))
	for _, name := range names {
		if s := aggregateOneInstance(byInstance[name], alignedStart, bucket, n, outputStart); instanceSeriesHasPoints(s) {
			out[name] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// aggregateOneInstance 单实例按桶输出 goroutines/heapMB/cpuPercent 曲线。
func aggregateOneInstance(snaps []metrics.Snapshot, alignedStart, bucket int64, n int, outputStart int64) dto.RuntimeInstanceSeries {
	gSum, gHeap, _, gCount := instanceGauges(snaps, alignedStart, bucket, n)
	deltas := instanceDeltas(snaps, alignedStart, bucket, n)
	bucketSeconds := float64(bucket)

	var s dto.RuntimeInstanceSeries
	for idx := range n {
		t := alignedStart + int64(idx)*bucket
		if t < outputStart || gCount[idx] == 0 {
			continue
		}
		s.Goroutines = append(s.Goroutines, dto.RuntimePoint{Time: t, Value: round2(gSum[idx] / gCount[idx])})
		s.HeapMB = append(s.HeapMB, dto.RuntimePoint{Time: t, Value: round2(gHeap[idx] / gCount[idx] / constant.RuntimeMetricsBytesPerMB)})
		s.CPUPercent = append(s.CPUPercent, dto.RuntimePoint{Time: t, Value: round2(deltas[idx].cpu / bucketSeconds * constant.RuntimeMetricsPercentToRatio)})
	}
	return s
}

func instanceSeriesHasPoints(s dto.RuntimeInstanceSeries) bool {
	return len(s.Goroutines) > 0 || len(s.HeapMB) > 0 || len(s.CPUPercent) > 0
}

func collectProviders(agg []bucketAgg) []string {
	set := map[string]struct{}{}
	for i := range agg {
		for prov := range agg[i].sse {
			set[prov] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func percentileP95(buckets map[string]float64, total float64) float64 {
	if total <= 0 || len(buckets) == 0 {
		return 0
	}
	type lePoint struct {
		le    float64
		count float64
	}
	points := lo.FilterMap(lo.Entries(buckets), func(e lo.Entry[string, float64], _ int) (lePoint, bool) {
		v, err := strconv.ParseFloat(e.Key, constant.ParseFloat64BitSize)
		if err != nil {
			return lePoint{}, false
		}
		return lePoint{le: v, count: e.Value}, true
	})
	slices.SortFunc(points, func(a, b lePoint) int { return cmp.Compare(a.le, b.le) })

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
