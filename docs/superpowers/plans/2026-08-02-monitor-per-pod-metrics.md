# Monitor CPU/Heap/Goroutines 多 pod 曲线绘制 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** monitor 页面的 CPU Usage / Heap Memory / Goroutines 三张图改为只显示各 pod 曲线（每 pod 一条线、图例为 pod 名），头部 Goroutines/Heap/SSE Active 卡片保留集群总和且 Goroutines/SSE 显示整数。

**Architecture:** 上报与存储链路（Flusher → Redis ZSET，instanceID=hostname=pod 名）零改动；只改 `GET /api/v1/metrics/runtime` 响应结构——`RuntimeSeries` 删除聚合字段 `goroutines/heapMB/cpuPercent`，新增 `Instances map[pod名]RuntimeInstanceSeries` per-pod 曲线；前端三张图从 `instances` 构造每 pod 一列的多曲线数据（复用 `RuntimeChart` 多 series 能力），卡片对 per-pod 最新值求和并整数化。

**Tech Stack:** Go 1.x（samber/lo、bytedance/sonic）、huma DTO、Next.js 16 + recharts、Redis ZSET。

## Global Constraints

- 展示形态 C：CPU/Heap/Goroutines 三张图**只显示各 pod 曲线**，不画聚合线。
- 头部卡片：Goroutines = 各 pod 最新值求和后四舍五入为整数；Heap = 各 pod 最新值求和（MB）；SSE Active = 各 provider 最新值求和后四舍五入为整数。
- QPS/P95/TPS/SuccessRate/SSE 图表保持跨 pod 聚合，**不动**。
- per-pod CPU% 为单 pod 独立 0-100%（不再集群求和）。
- 空桶（无快照）不输出点；实例按名称排序保证响应稳定。
- 上报/存储层（`internal/infrastructure/metrics/`、`internal/infrastructure/cache/runtime_metrics.go`）**零改动**。

---

### Task 1: 后端 DTO + 聚合层 per-instance 输出（TDD）

**Files:**
- Modify: `internal/dto/metrics.go`（`RuntimeSeries` 删聚合字段、新增 `Instances` + `RuntimeInstanceSeries`）
- Modify: `internal/application/metrics/query/runtime_metrics.go`（`buildSeries` 删输出、新增 `aggregateInstances`/`aggregateOneInstance`、handler 组装）
- Test: `test/unit/metrics/aggregate_test.go`

**Interfaces:**
- Consumes: 现有 `Aggregate(byInstance map[string][]metrics.Snapshot, alignedStart, bucket, end, outputStart int64) dto.RuntimeSeries`、`instanceGauges`、`instanceDeltas`、`constant.RuntimeMetricsBytesPerMB`、`constant.RuntimeMetricsPercentToRatio`。
- Produces: `dto.RuntimeSeries` 新增字段 `Instances map[string]dto.RuntimeInstanceSeries`；`dto.RuntimeInstanceSeries{Goroutines, HeapMB, CPUPercent []dto.RuntimePoint}`（后续 Task 2 前端按此契约消费）。

- [ ] **Step 1: 更新测试（红）**

替换 `test/unit/metrics/aggregate_test.go` 的 `TestAggregate_CrossPodSumRateAndP95` 为 per-instance 断言，并新增 per-instance 用例。删除对 `got.Goroutines` / `got.CPUPercent` 聚合字段的全部引用：

```go
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
	if _, ok := got.Instances["pod-z"]; !ok {
		t.Error("expected instance pod-z to be present")
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/monitor-per-pod-2026-08-02 && go test -count=1 ./test/unit/metrics/...`
Expected: 编译失败 — `got.Instances` undefined（`RuntimeSeries` 尚无 `Instances` 字段）。

- [ ] **Step 3: 实现 DTO 变更**

修改 `internal/dto/metrics.go`：`RuntimeSeries` 删除 `Goroutines` / `HeapMB` / `CPUPercent` 三个聚合字段，新增 `Instances`；新增 `RuntimeInstanceSeries` 结构：

```go
// RuntimeSeries 各运行时指标的时序
//
//	@author centonhuang
//	@update 2026-08-02 10:00:00
type RuntimeSeries struct {
	QPS         []RuntimePoint            `json:"qps" doc:"每秒请求数（跨 pod 求和）"`
	P95Ms       []RuntimePoint            `json:"p95Ms" doc:"P95 请求时延 ms（跨 pod 合并 bucket）"`
	SSEActive   map[string][]RuntimePoint `json:"sseActive" doc:"各 provider 的 SSE 活跃连接数"`
	TokenInput  []RuntimePoint            `json:"tokenInput" doc:"输入 token 速率 /s（跨 pod 求和）"`
	TokenOutput []RuntimePoint            `json:"tokenOutput" doc:"输出 token 速率 /s（跨 pod 求和）"`
	SuccessRate []RuntimePoint            `json:"successRate" doc:"HTTP 200 请求占比 %（0-100）"`
	Instances   map[string]RuntimeInstanceSeries `json:"instances" doc:"各 pod 的运行时曲线（goroutines/heapMB/cpuPercent）"`
}

// RuntimeInstanceSeries 单个 pod 的运行时曲线
//
//	@author centonhuang
//	@update 2026-08-02 10:00:00
type RuntimeInstanceSeries struct {
	Goroutines []RuntimePoint `json:"goroutines" doc:"goroutine 数（单 pod）"`
	HeapMB     []RuntimePoint `json:"heapMB" doc:"堆内存 MB（单 pod）"`
	CPUPercent []RuntimePoint `json:"cpuPercent" doc:"CPU 使用率 %（单 pod，0-100）"`
}
```

- [ ] **Step 4: 实现聚合层改造**

修改 `internal/application/metrics/query/runtime_metrics.go`：

(1) `RuntimeMetrics` handler 中，`Aggregate` 之后组装 `Instances`：

```go
	series := Aggregate(byInstance, alignedStart, bucket, end, outputStart)
	series.Instances = aggregateInstances(byInstance, alignedStart, bucket, end, outputStart)
	// latest 取当前桶起点；无样本的桶在 buildSeries 中被跳过，故空的末桶不会塌成 0。
	latest := end - mod(end, bucket)
	return series, latest, nil
```

(2) `bucketAgg` 删除 `goroutines` / `heap` 字段（不再跨实例求和输出），`mergeGaugeBucket` 同步简化：

```go
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
```

```go
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
```

(3) `buildSeries` 删除 goroutines/heapMB/cpuPercent 输出分支：

```go
	for idx := range agg {
		t := alignedStart + int64(idx)*bucket
		// t < outputStart：增量下界之前的桶不重复输出；
		// samples == 0：该桶无任何快照（含末尾未采集到的当前桶、抖动丢点的空桶），
		// 输出会让指标塌成 0 形成断崖，故直接跳过。
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
```

(4) 新增 per-instance 聚合函数（追加在 `collectProviders` 之前）：

```go
// aggregateInstances 把每个 instance 的快照各自按桶聚合成独立曲线（不跨实例求和）：
// goroutines/heapMB 取桶内均值；cpuPercent 取桶内 cpu 正 delta ÷ 桶宽。实例按名称排序输出，
// 保证响应稳定；无任何输出的实例（空桶）不出现。
func aggregateInstances(byInstance map[string][]metrics.Snapshot, alignedStart, bucket, end, outputStart int64) map[string]dto.RuntimeInstanceSeries {
	names := lo.Filter(lo.Keys(byInstance), func(n string, _ int) bool {
		return len(byInstance[n]) > 0
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
```

- [ ] **Step 5: 运行测试验证通过**

Run: `go test -count=1 ./test/unit/metrics/... ./internal/application/metrics/...`
Expected: PASS（`TestAggregate_CrossPodSumRateAndP95`、`TestAggregate_InstancesPerPodSkipsEmptyBucket`、`TestAggregate_CounterResetClamped`、`TestAggregate_TokenRateAndSuccessRate`、`TestAggregate_SuccessRateSkipsEmptyBucket` 全绿）。

- [ ] **Step 6: lint 验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/monitor-per-pod-2026-08-02 && go vet ./internal/application/metrics/... ./internal/dto/... && go build ./...`
Expected: 无输出（通过）。若 `bucketAgg.goroutines/heap` 字段删除后 `instanceGauges` 返回值仍被使用（aggregateOneInstance 用 gSum/gHeap），无死代码告警。

- [ ] **Step 7: Commit**

```bash
git add internal/dto/metrics.go internal/application/metrics/query/runtime_metrics.go test/unit/metrics/aggregate_test.go
git commit -m "feat(metrics): runtime 接口返回 per-pod 的 goroutines/heap/cpu 曲线"
```

---

### Task 2: 前端 types + monitor 页面 per-pod 曲线

**Files:**
- Modify: `web/src/lib/types.ts`（`RuntimeSeries` 同步契约）
- Modify: `web/src/app/(dashboard)/monitor/page.tsx`（三张图 per-pod 多曲线、卡片集群和 + 整数化）

**Interfaces:**
- Consumes: Task 1 产生的 API 契约 —— `series.instances: Record<pod名, { goroutines?, heapMB?, cpuPercent?: RuntimePoint[] }>`。
- Produces: monitor 页 CPU/Heap/Goroutines 三图按 pod 绘制多曲线；头部 Goroutines/SSE Active 卡片整数。

- [ ] **Step 1: 同步 types.ts**

修改 `web/src/lib/types.ts` 中 `RuntimeSeries`（当前约 595-604 行）——删除 `goroutines` / `heapMB` / `cpuPercent` 三个字段，新增 `instances` 与 `RuntimeInstanceSeries`：

```ts
export interface RuntimeSeries {
  qps?: RuntimePoint[];
  p95Ms?: RuntimePoint[];
  sseActive?: Record<string, RuntimePoint[]>;
  tokenInput?: RuntimePoint[]; // 输入 token 速率 /s
  tokenOutput?: RuntimePoint[]; // 输出 token 速率 /s
  successRate?: RuntimePoint[]; // HTTP 200 占比 %（0-100）
  instances?: Record<string, RuntimeInstanceSeries>; // pod 名 → 该 pod 曲线
}

export interface RuntimeInstanceSeries {
  goroutines?: RuntimePoint[];
  heapMB?: RuntimePoint[];
  cpuPercent?: RuntimePoint[];
}
```

- [ ] **Step 2: 改造 monitor 页面**

修改 `web/src/app/(dashboard)/monitor/page.tsx`：

(1) `SeriesState` 中删除 `goroutines/heapMB/cpuPercent` 三组聚合数据，新增 per-pod `instances`；`EMPTY_STATE` 同步：

```ts
interface InstanceState {
  goroutines: Pt[];
  heapMB: Pt[];
  cpuPercent: Pt[];
}

interface SeriesState {
  qps: Pt[];
  p95Ms: Pt[];
  sseActive: Record<string, Pt[]>;
  tokenInput: Pt[];
  tokenOutput: Pt[];
  successRate: Pt[];
  instances: Record<string, InstanceState>;
}

const EMPTY_INSTANCE: InstanceState = { goroutines: [], heapMB: [], cpuPercent: [] };

const EMPTY_STATE: SeriesState = {
  qps: [],
  p95Ms: [],
  sseActive: {},
  tokenInput: [],
  tokenOutput: [],
  successRate: [],
  instances: {},
};
```

(2) 新增 per-pod 合并与数据构造辅助函数（放在 `mergeSSE` 之后）：

```ts
function mergeInstances(
  prev: Record<string, InstanceState>,
  incoming: Record<string, InstanceState>,
  cutoff: number,
): Record<string, InstanceState> {
  const pods = new Set([...Object.keys(prev), ...Object.keys(incoming)]);
  const out: Record<string, InstanceState> = {};
  for (const pod of pods) {
    const p = prev[pod] ?? EMPTY_INSTANCE;
    const inc = incoming[pod] ?? EMPTY_INSTANCE;
    out[pod] = {
      goroutines: mergePoints(p.goroutines, inc.goroutines ?? [], cutoff),
      heapMB: mergePoints(p.heapMB, inc.heapMB ?? [], cutoff),
      cpuPercent: mergePoints(p.cpuPercent, inc.cpuPercent ?? [], cutoff),
    };
  }
  return out;
}

// podChartData 把各 pod 的同一指标曲线合并为同一时间轴上的多列（列名 = pod 名）。
function podChartData(instances: Record<string, InstanceState>, metric: keyof InstanceState): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const [pod, inst] of Object.entries(instances)) {
    for (const p of inst[metric]) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[pod] = p.value;
      rows.set(p.time, row);
    }
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}

// podSumLatest 各 pod 最新值求和（用于头部卡片集群总和）。
function podSumLatest(instances: Record<string, InstanceState>, metric: keyof InstanceState): number {
  let sum = 0;
  for (const inst of Object.values(instances)) {
    sum += inst[metric].at(-1)?.value ?? 0;
  }
  return sum;
}
```

(3) `poll` 的 `setState` 中删除 `goroutines/heapMB/cpuPercent` 三行，新增 `instances`：

```ts
        setState((prev) => ({
          qps: mergePoints(prev.qps, s.qps ?? [], cutoff),
          p95Ms: mergePoints(prev.p95Ms, s.p95Ms ?? [], cutoff),
          sseActive: mergeSSE(prev.sseActive, s.sseActive ?? {}, cutoff),
          tokenInput: mergePoints(prev.tokenInput, s.tokenInput ?? [], cutoff),
          tokenOutput: mergePoints(prev.tokenOutput, s.tokenOutput ?? [], cutoff),
          successRate: mergePoints(prev.successRate, s.successRate ?? [], cutoff),
          instances: mergeInstances(prev.instances, s.instances ?? {}, cutoff),
        }));
```

(4) 组件体内新增 pod 系列与卡片数值计算（替换原来的 `sseTotal` 定义附近逻辑）：

```ts
  const pods = Object.keys(state.instances).sort();
  const podSeries = pods.map((pod, i) => ({
    key: pod,
    label: pod,
    color: seriesColors[i % seriesColors.length],
  }));
  const goroutinesTotal = Math.round(podSumLatest(state.instances, "goroutines"));
  const heapTotal = podSumLatest(state.instances, "heapMB");
  const sseTotal = Math.round(sseProviders.reduce((sum, prov) => sum + lastValue(state.sseActive[prov]), 0));
```

(5) 头部三张卡片改用集群总和值：

```tsx
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <RuntimeGaugeCard label={t("monitor.goroutines")} value={goroutinesTotal} icon={<Activity className="size-4" />} tone="primary" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.heap")} value={heapTotal} unit="MB" icon={<MemoryStick className="size-4" />} tone="blue" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.sse_active")} value={sseTotal} icon={<Radio className="size-4" />} tone="violet" loading={loading} />
      </div>
```

(6) CPU / Heap / Goroutines 三张图改为 per-pod 多曲线：

```tsx
        <RuntimeChart title={t("monitor.cpu_usage")} data={podChartData(state.instances, "cpuPercent")} series={podSeries} unit="%" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.heap_memory")} data={podChartData(state.instances, "heapMB")} series={podSeries} unit=" MB" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.goroutines_chart")} data={podChartData(state.instances, "goroutines")} series={podSeries} rangeKey={range} emptyLabel={t("monitor.collecting")} />
```

- [ ] **Step 3: 前端 lint + 构建验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/monitor-per-pod-2026-08-02/web && npm run lint && npm run build`
Expected: lint 无错误；`next build` 成功（含 TypeScript 类型检查）。

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/app/'(dashboard)'/monitor/page.tsx
git commit -m "feat(web): monitor CPU/Heap/Goroutines 按 pod 绘制多曲线，头部卡片集群和整数化"
```

---

### Task 3: 全量回归验证

**Files:** 无代码改动。

- [ ] **Step 1: 后端全量测试 + lint**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/monitor-per-pod-2026-08-02 && go test -count=1 ./cmd/... ./internal/... ./test/... 2>&1 | tail -30`
Expected: 全部 PASS（`make test` 等价命令）。

Run: `make lint`
Expected: 两阶段 lint（lint-conv + lint-static）通过。

- [ ] **Step 2: 汇报并询问集成方式**

向用户汇报验证结果，询问是否需要提 MR 或合并到 master（不擅自推送）。
