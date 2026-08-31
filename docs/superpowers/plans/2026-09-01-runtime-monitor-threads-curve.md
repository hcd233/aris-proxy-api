# 运行时监控协程数看板增加 M 曲线 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 协程数看板在现有 per-pod G 曲线基础上，叠加 per-pod M（OS 线程数）虚线曲线。

**Architecture:** 数据从 Prometheus 指标 `go_threads`（Go collector 恒定导出）经 `BuildSnapshot` 写入 Redis 快照，聚合层按桶内均值输出 per-pod `threads` 曲线（`Threads > 0` 才计入，0 为旧快照"无数据"哨兵），前端 monitor 页把各 pod 的 G/M 合并为同时间轴多列，M 与所属 pod 同色虚线。

**Tech Stack:** Go（prometheus client_golang v1.23.2 / sonic / lo）、Next.js + recharts、Redis ZSET 快照存储。

**Spec:** `docs/superpowers/specs/2026-09-01-runtime-monitor-threads-curve-design.md`

## Global Constraints

- 本任务按用户既定偏好**直接在 master 开发**，不建 worktree；**禁止 push**（push master 触发自动部署）。
- 所有 shell 命令加 `rtk` 前缀。
- 编辑任何 Go 文件**之前**必须先跑（禁止用 head/tail/grep 截断输出，逐字执行）：
  `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <准备编辑的 go 文件路径>`
  返回指南具有权威性；与项目硬约束（`docs/agents/go-backend.md`）冲突时项目约束优先。
- 项目硬约束：DTO 层禁 `any`/`interface{}`；业务错误走 `internal/common/ierr`（本任务不涉及新错误路径）。
- 浮点输出统一 `round2`（`math.Round(v*scale)/scale`），禁止 `int64(v*scale+0.5)` 截断。
- Go 命名遵循 `golang-naming`；提交信息格式 `type(scope): 中文描述`（参照 git log 惯例）。
- 前端每步改动后验证：`cd web && npx tsc --noEmit`；lint 用 `npx eslint <改动文件>`。
- commit 前用 Serena 写入工程经验（Task 5）。

---

### Task 1: 采集层 — Snapshot 读取 go_threads

**Files:**
- Modify: `internal/common/constant/metrics.go:54-60`（MetricFull 常量区）
- Modify: `internal/infrastructure/metrics/snapshot.go:19-31`（Snapshot struct）、`snapshot.go:62-72`（BuildSnapshot）
- Test: `test/unit/metrics/snapshot_test.go`

**Interfaces:**
- Consumes: `firstGaugeValue(f *metricpb.MetricFamily) float64`（snapshot.go:77 既有）。
- Produces: 常量 `constant.MetricFullGoThreads = "go_threads"`；`metrics.Snapshot.Threads float64`（json `threads`）。Task 2 的聚合层消费 `Snapshot.Threads`。

- [ ] **Step 1: 跑 use-modern-go list**

```sh
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/metrics/snapshot.go
```

- [ ] **Step 2: 写失败测试**

在 `test/unit/metrics/snapshot_test.go` 末尾追加（import 区已有 `prometheus`、`constant`、`time`，无需新增 import）：

```go
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
```

- [ ] **Step 3: 跑测试确认失败**

Run: `rtk go test ./test/unit/metrics/ -run TestBuildSnapshot_ExtractsGoThreads -v`
Expected: FAIL（`snap.Threads` 不存在，编译错误 `unknown field Threads`）

- [ ] **Step 4: 最小实现**

`internal/common/constant/metrics.go` 的 MetricFull 区块（gofmt 对齐）：

```go
	MetricFullRequestDuration = "http_request_duration_seconds"
	MetricFullGoGoroutines    = "go_goroutines"
	MetricFullGoThreads       = "go_threads"
	MetricFullGoHeapAlloc     = "go_memstats_alloc_bytes"
	MetricFullProcessCPU      = "process_cpu_seconds_total"
	MetricFullSSEActive       = "sse_active_connections"
	MetricFullTokenUsage      = "llm_token_usage_total"
	MetricFullHTTPRequests    = "http_requests_total"
```

`internal/infrastructure/metrics/snapshot.go` Snapshot struct 在 `Goroutines` 行后加一行（注释风格同邻行）：

```go
type Snapshot struct {
	TS          int64              `json:"ts"`                   // unix 秒
	Goroutines  float64            `json:"goroutines"`           // gauge
	Threads     float64            `json:"threads"`              // gauge：OS 线程数 M（旧快照无此字段解码为 0）
	HeapBytes   float64            `json:"heapBytes"`            // gauge
```

`BuildSnapshot` 的 `snap := &Snapshot{...}` 中 `Goroutines` 行后加：

```go
		Goroutines:  firstGaugeValue(byName[constant.MetricFullGoGoroutines]),
		Threads:     firstGaugeValue(byName[constant.MetricFullGoThreads]),
		HeapBytes:   firstGaugeValue(byName[constant.MetricFullGoHeapAlloc]),
```

- [ ] **Step 5: 跑测试确认通过**

Run: `rtk go test ./test/unit/metrics/ -run TestBuildSnapshot -v`
Expected: PASS（新旧用例全绿）

- [ ] **Step 6: Commit**

```bash
rtk git add internal/common/constant/metrics.go internal/infrastructure/metrics/snapshot.go test/unit/metrics/snapshot_test.go
rtk git commit -m "feat(metrics): 快照采集 go_threads（M）"
```

---

### Task 2: 聚合层与 DTO — per-pod threads 曲线

**Files:**
- Modify: `internal/dto/metrics.go:40-44`（RuntimeInstanceSeries）
- Modify: `internal/application/metrics/query/runtime_metrics.go:153`（accumulateInstance 调用）、`:165-187`（instanceGauges）、`:322-342`（aggregateOneInstance、instanceSeriesHasPoints）
- Test: `test/unit/metrics/aggregate_test.go`

**Interfaces:**
- Consumes: `metrics.Snapshot.Threads float64`（Task 1）。
- Produces: `dto.RuntimeInstanceSeries.Threads []RuntimePoint`（json `threads`）；`instanceGauges` 新签名 `(gSum, gHeap, gThreads []float64, gSSE []map[string]float64, gCount, tCount []float64)`（包内私有，仅 runtime_metrics.go 两个调用点）。Task 4 前端消费 json 字段 `threads`。

- [ ] **Step 1: 跑 use-modern-go list**

```sh
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/application/metrics/query/runtime_metrics.go
```

- [ ] **Step 2: 写失败测试**

在 `test/unit/metrics/aggregate_test.go` 末尾追加：

```go
func TestAggregate_ThreadsPerPodSkipsLegacySnapshots(t *testing.T) {
	t.Parallel()
	const bucket int64 = 60
	// 桶0：新快照 Threads 30、旧版快照（无 threads 字段解码为 0）、新快照 Threads 50
	// → threads 均值 (30+50)/2=40，不得被 0 稀释成 80/3。
	// 桶1：仅旧版快照 → threads 无有效样本，不输出该点。
	byInstance := map[string][]metrics.Snapshot{
		"pod-a": {
			{TS: 0, Goroutines: 10, Threads: 30},
			{TS: 15, Goroutines: 20},
			{TS: 30, Goroutines: 20, Threads: 50},
			{TS: 90, Goroutines: 30},
		},
	}

	got := metricsquery.Aggregate(byInstance, 0, bucket, 120, 0)

	a := got.Instances["pod-a"]
	if len(a.Threads) != 1 {
		t.Fatalf("expected 1 threads point, got %+v", a.Threads)
	}
	if a.Threads[0].Time != 0 || a.Threads[0].Value != 40 {
		t.Errorf("expected threads 40@0, got %+v", a.Threads[0])
	}
	// goroutines 曲线不受 threads 哨兵逻辑影响：桶0 均值 50/3、桶1 原值 30
	if len(a.Goroutines) != 2 {
		t.Errorf("expected 2 goroutines points, got %+v", a.Goroutines)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `rtk go test ./test/unit/metrics/ -run TestAggregate_Threads -v`
Expected: FAIL（`Snapshot` 无 `Threads` 字段则编译错；若 Task 1 已合并则 FAIL 于 `expected 1 threads point, got []`）

- [ ] **Step 4: 最小实现**

`internal/dto/metrics.go` RuntimeInstanceSeries 加字段：

```go
type RuntimeInstanceSeries struct {
	Goroutines []RuntimePoint `json:"goroutines" doc:"goroutine 数（单 pod）"`
	HeapMB     []RuntimePoint `json:"heapMB" doc:"堆内存 MB（单 pod）"`
	CPUPercent []RuntimePoint `json:"cpuPercent" doc:"CPU 使用率 %（单 pod，0-100）"`
	Threads    []RuntimePoint `json:"threads" doc:"OS 线程数 M（单 pod）"`
}
```

`internal/application/metrics/query/runtime_metrics.go`：

`instanceGauges` 整体替换（加 gThreads/tCount，`Threads > 0` 才计入）：

```go
// instanceGauges 按桶累加单实例的 gauge 原值与计数（用于后续求桶内均值）。
// threads 单独计数：旧版快照无该字段（解码为 0），仅累计有效样本避免稀释均值。
func instanceGauges(snaps []metrics.Snapshot, alignedStart, bucket int64, n int) (gSum, gHeap, gThreads []float64, gSSE []map[string]float64, gCount, tCount []float64) {
	gSum = make([]float64, n)
	gHeap = make([]float64, n)
	gThreads = make([]float64, n)
	gSSE = make([]map[string]float64, n)
	gCount = make([]float64, n)
	tCount = make([]float64, n)
	for _, s := range snaps {
		idx := int((s.TS - alignedStart) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		gSum[idx] += s.Goroutines
		gHeap[idx] += s.HeapBytes
		gCount[idx]++
		if s.Threads > 0 {
			gThreads[idx] += s.Threads
			tCount[idx]++
		}
		if gSSE[idx] == nil {
			gSSE[idx] = map[string]float64{}
		}
		for prov, v := range s.SSEActive {
			gSSE[idx][prov] += v
		}
	}
	return gSum, gHeap, gThreads, gSSE, gCount, tCount
}
```

`accumulateInstance` 内的调用点同步改签名（丢弃新增返回值）：

```go
	_, _, _, gSSE, gCount, _ := instanceGauges(snaps, alignedStart, bucket, n)
```

`aggregateOneInstance` 整体替换（输出 threads 点）：

```go
// aggregateOneInstance 单实例按桶输出 goroutines/heapMB/cpuPercent/threads 曲线。
func aggregateOneInstance(snaps []metrics.Snapshot, alignedStart, bucket int64, n int, outputStart int64) dto.RuntimeInstanceSeries {
	gSum, gHeap, gThreads, _, gCount, tCount := instanceGauges(snaps, alignedStart, bucket, n)
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
		// 桶内无有效 threads 样本（纯旧快照）时不输出该点，前端自然断线。
		if tCount[idx] > 0 {
			s.Threads = append(s.Threads, dto.RuntimePoint{Time: t, Value: round2(gThreads[idx] / tCount[idx])})
		}
	}
	return s
}
```

`instanceSeriesHasPoints` 加 threads：

```go
func instanceSeriesHasPoints(s dto.RuntimeInstanceSeries) bool {
	return len(s.Goroutines) > 0 || len(s.HeapMB) > 0 || len(s.CPUPercent) > 0 || len(s.Threads) > 0
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `rtk go test ./test/unit/metrics/ -v`
Expected: PASS（aggregate + snapshot 全部用例）

- [ ] **Step 6: Commit**

```bash
rtk git add internal/dto/metrics.go internal/application/metrics/query/runtime_metrics.go test/unit/metrics/aggregate_test.go
rtk git commit -m "feat(metrics): runtime 聚合输出 per-pod threads 曲线"
```

---

### Task 3: 前端 — RuntimeChart 虚线支持与类型

**Files:**
- Modify: `web/src/lib/types.ts:753-757`（RuntimeInstanceSeries）
- Modify: `web/src/components/charts/runtime-chart.tsx:16-20`（RuntimeChartSeries）、`:138-149`（Line 渲染）

**Interfaces:**
- Consumes: 无（独立改动）。
- Produces: TS 类型 `RuntimeInstanceSeries.threads?: RuntimePoint[]`；`RuntimeChartSeries.dashed?: boolean`（Task 4 的 M series 用 `dashed: true`）。

- [ ] **Step 1: 修改类型与组件**

`web/src/lib/types.ts`：

```ts
export interface RuntimeInstanceSeries {
  goroutines?: RuntimePoint[];
  heapMB?: RuntimePoint[];
  cpuPercent?: RuntimePoint[];
  threads?: RuntimePoint[];
}
```

`web/src/components/charts/runtime-chart.tsx` 的 `RuntimeChartSeries` 加可选 `dashed`：

```tsx
interface RuntimeChartSeries {
  key: string;
  label: string;
  color: string;
  dashed?: boolean;
}
```

`series.map` 渲染的 `<Line>` 加 `strokeDasharray`（其余 props 不动）：

```tsx
              {series.map((s) => (
                <Line
                  key={s.key}
                  type="monotone"
                  dataKey={s.key}
                  stroke={s.color}
                  strokeWidth={2}
                  strokeOpacity={getStrokeOpacity(s.key)}
                  strokeDasharray={s.dashed ? "6 4" : undefined}
                  dot={false}
                  isAnimationActive={false}
                />
              ))}
```

- [ ] **Step 2: 类型检查与 lint**

Run: `cd web && npx tsc --noEmit && npx eslint src/lib/types.ts src/components/charts/runtime-chart.tsx`
Expected: 无输出（0 error）

- [ ] **Step 3: Commit**

```bash
rtk git add web/src/lib/types.ts web/src/components/charts/runtime-chart.tsx
rtk git commit -m "feat(web): RuntimeChart 支持虚线 series 与 threads 类型"
```

---

### Task 4: 前端 — monitor 页 G/M 组装与 i18n

**Files:**
- Modify: `web/src/app/(dashboard)/monitor/page.tsx`（InstanceState:38-42、EMPTY_INSTANCE:55、mergeInstances:94-110、新函数 gmChartData、goroutines 图 :366-372）
- Modify: `web/src/locales/zh.json:183`、`web/src/locales/en.json:294`、`web/src/locales/ja.json:294`

**Interfaces:**
- Consumes: `RuntimeChartSeries.dashed`、`RuntimeInstanceSeries.threads`（Task 3）。
- Produces: 页面级函数 `gmChartData(instances)`；模块常量 `THREADS_KEY_PREFIX = "m:"`（M 列名 = `m:` + pod 名，RFC 1123 主机名不含冒号，无冲突）。

- [ ] **Step 1: 修改 monitor 页**

`InstanceState` 与 `EMPTY_INSTANCE`：

```tsx
interface InstanceState {
  goroutines: Pt[];
  heapMB: Pt[];
  cpuPercent: Pt[];
  threads: Pt[];
}
```

```tsx
const EMPTY_INSTANCE: InstanceState = {
  goroutines: [],
  heapMB: [],
  cpuPercent: [],
  threads: [],
};
```

`mergeInstances` 的 out[pod] 赋值加 threads（其余行不动）：

```tsx
    out[pod] = {
      goroutines: mergePoints(p.goroutines, inc.goroutines ?? [], cutoff),
      heapMB: mergePoints(p.heapMB, inc.heapMB ?? [], cutoff),
      cpuPercent: mergePoints(p.cpuPercent, inc.cpuPercent ?? [], cutoff),
      threads: mergePoints(p.threads, inc.threads ?? [], cutoff),
    };
```

在 `podChartData` 函数后新增模块常量与 `gmChartData`：

```tsx
const THREADS_KEY_PREFIX = "m:";

// gmChartData 把各 pod 的 G/M 曲线合并为同一时间轴多列：列名 pod 为 G，`m:`+pod 为 M。
function gmChartData(
  instances: Record<string, InstanceState>,
): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const [pod, inst] of Object.entries(instances)) {
    for (const p of inst.goroutines) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[pod] = p.value;
      rows.set(p.time, row);
    }
    for (const p of inst.threads) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[`${THREADS_KEY_PREFIX}${pod}`] = p.value;
      rows.set(p.time, row);
    }
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}
```

组件内 `podSeries` 定义后新增 gmSeries（M 与所属 pod 同色虚线）：

```tsx
  const gmSeries = [
    ...pods.map((pod, i) => ({
      key: pod,
      label: pod,
      color: seriesColors[i % seriesColors.length],
    })),
    ...pods.map((pod, i) => ({
      key: `${THREADS_KEY_PREFIX}${pod}`,
      label: `${pod} (M)`,
      color: seriesColors[i % seriesColors.length],
      dashed: true,
    })),
  ];
```

协程数图（原 `data={podChartData(state.instances, "goroutines")}` `series={podSeries}`）替换为：

```tsx
          <RuntimeChart
            title={t("monitor.goroutines_chart")}
            data={gmChartData(state.instances)}
            series={gmSeries}
            rangeKey={range}
            emptyLabel={t("monitor.collecting")}
          />
```

- [ ] **Step 2: 更新三份 locale 的图标题**

`web/src/locales/zh.json` `"monitor.goroutines_chart": "协程数"` → `"协程 / 线程数"`
`web/src/locales/en.json` `"monitor.goroutines_chart": "Goroutines"` → `"Goroutines / Threads"`
`web/src/locales/ja.json` `"monitor.goroutines_chart": "Goroutines"` → `"ゴルーチン / スレッド数"`

- [ ] **Step 3: 类型检查与 lint**

Run: `cd web && npx tsc --noEmit && npx eslint "src/app/(dashboard)/monitor/page.tsx"`
Expected: 无输出（0 error）

- [ ] **Step 4: Commit**

```bash
rtk git add "web/src/app/(dashboard)/monitor/page.tsx" web/src/locales/zh.json web/src/locales/en.json web/src/locales/ja.json
rtk git commit -m "feat(web): 协程数看板叠加 per-pod M 虚线曲线"
```

---

### Task 5: 全量验证与收尾

**Files:**
- 无新改动（验证 + 经验沉淀；如验证发现问题，修复后单独 commit）

**Interfaces:**
- Consumes: Task 1-4 全部产出。
- Produces: Serena 工程经验 memory。

- [ ] **Step 1: 前端构建**

Run: `cd web && npm run build`
Expected: 构建成功（Google Fonts 下载瞬时失败可重跑一次）

- [ ] **Step 2: 构建 embed dist 并跑 Go 全量测试**

```bash
rm -rf internal/web/dist && cp -r web/out internal/web/dist
rtk go test ./cmd/... ./internal/... ./test/...
```

Expected: 全部 PASS（blocked_command 等历史用例无预失败）

- [ ] **Step 3: ponytail-review 审查本次 diff**

对 `git diff e290632..HEAD` 跑 `ponytail-review`（投机抽象/重复造轮子/死代码逐项排查）。
Expected: 无可删项，或删完后重跑 Step 1-2。

- [ ] **Step 4: 浏览器 mock 验证（推荐，可选）**

沿用 `web/monitor-chart-legend-highlight` 经验：临时 Node mock 服务器提供 `GET /api/v1/user/current`（`{user:{permission:"admin"}}`）与 `GET /api/v1/metrics/runtime?range=1h`（构造 2 个 pod 的 `instances`，每个 pod `goroutines` 与 `threads` 各若干点，latestTime=now 对齐 60s 桶）；`NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npx next dev -p 3000`；chrome-devtools MCP 打开 `/web/monitor`，断言：协程图每 pod 一条实线 + 一条 `${pod} (M)` 虚线（path 元素 `stroke-dasharray="6 4"`），图例 4 项，悬停高亮正常。mock 文件为临时文件不入仓库。

- [ ] **Step 5: Serena 沉淀工程经验**

`serena_write_memory` 写入 `metrics/runtime-threads-curve`：go_threads 数据源、tCount 哨兵语义（0=旧快照无字段）、前端 `m:` 前缀列名与 dashed series 约定、验证方式。完成后向用户汇报，**等用户明确要求再 push/部署**。

---

## Self-Review 记录

- **Spec coverage**：采集层（Task 1）、聚合层+DTO（Task 2）、RuntimeChart 虚线+类型（Task 3）、monitor 页组装+i18n（Task 4）、全量验证+沉淀（Task 5）——spec 各节均有对应任务；头部卡片/集群聚合 M 在 spec「明确不做」，无任务 ✓。
- **Placeholder scan**：所有代码步骤含完整代码与精确命令，无 TBD/TODO ✓。
- **Type consistency**：`Snapshot.Threads`（Go float64）→ json `threads` → `RuntimeInstanceSeries.Threads []RuntimePoint` → TS `threads?: RuntimePoint[]` → 列名 `m:${pod}` / `RuntimeChartSeries.dashed`，各任务签名一致 ✓。
