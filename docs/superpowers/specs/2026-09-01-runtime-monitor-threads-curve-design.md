# 运行时监控协程数看板增加 M（OS 线程）曲线 — 设计文档

> 日期：2026-09-01
> 状态：已批准（用户确认 per-pod 虚线口径）

## 背景与目标

monitor 页协程数看板目前只画各 pod 的 G（goroutine 数）曲线。需求：同一看板内补充 M（OS 线程数）曲线，用于观察线程资源占用与线程泄漏。

## 关键决策

| 决策点 | 结论 | 理由 |
|--------|------|------|
| M 曲线口径 | **per-pod**，M 用虚线区分 | 沿用决策 C（monitor 三张图只画各 pod 曲线）；不复活已删除的集群聚合字段；单 pod 时即"G 一条 + M 一条" |
| M 数据源 | Prometheus 指标 `go_threads` | client_golang v1.23.2 的 baseGoCollector 恒定导出，语义为"Number of OS threads created"；Go 的 M 极少退出，近似等于当前线程数。零新增 collector |
| 旧快照兼容 | `Threads == 0` 视为"无数据"哨兵 | Redis 旧快照解码后该字段为 0；真实 Go 进程不可能 0 线程。聚合层跳过这些点，不稀释均值 |
| 头部卡片 | 不加 M 卡片 | 需求未要求，YAGNI |

## 数据链路（改动点）

```
BuildSnapshot ──> Snapshot.Threads ──> Redis ZSET ──> Aggregate/aggregateOneInstance
                                                          │
                              dto.RuntimeInstanceSeries.Threads ──> 前端 monitor 页 ──> RuntimeChart
```

### 1. 采集层

- `internal/common/constant/metrics.go`：新增 `MetricFullGoThreads = "go_threads"`。
- `internal/infrastructure/metrics/snapshot.go`：`Snapshot` 新增 `Threads float64`（gauge，json tag `threads`，风格同 `Goroutines`）；`BuildSnapshot` 用 `firstGaugeValue(byName[constant.MetricFullGoThreads])` 读取。Flusher/Redis 写入链路零改动。

### 2. 聚合层与 DTO

- `internal/dto/metrics.go`：`RuntimeInstanceSeries` 新增 `Threads []RuntimePoint`（`json:"threads"`，doc 注明"OS 线程数 M（单 pod）"）。
- `internal/application/metrics/query/runtime_metrics.go`：
  - `instanceGauges` 增加 threads 求和与**独立计数** `tCount`：仅当 `s.Threads > 0` 时累计（保证旧快照不稀释均值、不产生假 0 塌线）。
  - `aggregateOneInstance`：`tCount[idx] > 0` 的桶输出 `s.Threads`（桶内均值，`round2`），否则跳过该点。
  - `instanceSeriesHasPoints` 增加 `len(s.Threads) > 0`。
  - 集群聚合（`buildSeries`/`bucketAgg`）**不动**，`accumulateInstance` 调用处对新增返回值用 `_` 丢弃。

### 3. 前端

- `web/src/lib/types.ts`：`RuntimeInstanceSeries` 加 `threads?: RuntimePoint[]`。
- `web/src/app/(dashboard)/monitor/page.tsx`：
  - `InstanceState`/`EMPTY_INSTANCE`/`mergeInstances` 同步加 `threads`。
  - 协程数图改用新 builder：每行 `{ time, [pod]: G值, ["m:"+pod]: M值 }`（`m:` 前缀避免与 pod 名冲突）；series = 每 pod 一条 G 实线（沿用 `podSeries` 配色）+ 一条 M 虚线（与所属 pod **同色**），M 图例 label 为 `${pod} (M)`。
- `web/src/components/charts/runtime-chart.tsx`：`RuntimeChartSeries` 加可选 `dashed?: boolean`，`Line` 透传 `strokeDasharray={s.dashed ? "6 4" : undefined}`。其余 7 张图不受影响。
- i18n（`zh.json`/`en.json`/`ja.json`）：图标题 `monitor.goroutines_chart` 更新为「协程 / 线程数」/ "Goroutines / Threads" / 「ゴルーチン / スレッド数」。

## 兼容性与平滑过渡

- 滚动发布过渡期：旧快照无 threads → M 线该时段断点（recharts 默认断线渲染），随新快照积累自然补满（快照保留 24h）。
- 新旧 pod 混布：各自独立成线，互不干扰。
- API 为纯新增字段，无破坏性变更。

## 验证

- 后端单测（query 包既有测试文件补用例）：
  - threads 桶内均值正确；
  - 旧快照 `Threads == 0` 被跳过、不稀释均值；
  - `instanceSeriesHasPoints` 对仅 threads 有值的 series 返回 true。
- 前端：`npx tsc --noEmit` + eslint + `next build`；本地 mock `/api/v1/metrics/runtime`（沿用 monitor-chart-legend-highlight 的 mock 方法）浏览器验证 G/M 双线、虚线样式与图例。
- 全量 `go test` 前先构建 `internal/web/dist`（`rm -rf internal/web/dist && cp -r web/out internal/web/dist`）。

## 明确不做

- 头部 M 卡片、集群聚合 M 曲线、`go_threads` 之外的线程指标（如 `/proc` 实际线程数）。
- 历史快照回填。
