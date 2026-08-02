# Monitor CPU / Heap / Goroutines 支持多 pod 曲线绘制 — 设计文档

> 日期：2026-08-02
> 状态：已评审（用户确认）

## 背景与目标

web 端 monitor 页面的 **CPU Usage / Heap Memory / Goroutines** 三张图当前只绘制跨 pod 聚合后的**单条曲线**（goroutines/heapMB/cpuPercent 均为"集群求和"），无法查看各 pod 各自的运行曲线。

本次需求：这三张图**只显示各 pod 曲线**（去掉聚合线），每个 pod 一条曲线、图例用 pod 名区分；头部 Goroutines / Heap / SSE Active 三张 gauge 卡片保留"集群总和"展示，其中 **Goroutines 与 SSE Active 显示整数**。

api 侧通过**扩展 `GET /api/v1/metrics/runtime` 响应字段**（`RuntimeSeries`）提供 per-pod 能力，不新增接口；上报与存储链路（Flusher → Redis ZSET）**零改动**——快照本就按 instanceID（= hostname = K8s pod 名）独立存储。

## 关键决策（已与用户确认）

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 展示形态 | **C：只显示各 pod 曲线**（CPU/Heap/Goroutines 去掉聚合线） | 关注各 pod 分布；QPS/P95/TPS/SuccessRate/SSE 图不受影响，仍为跨 pod 聚合 |
| 头部卡片 | **1：保留"集群总和"** | 前端对各 pod 最新值求和显示，一眼仍见总量 |
| 卡片数值 | **Goroutines / SSE Active 显示整数** | 两者语义上就是整数（连接数、goroutine 数），避免均值求和产生小数 |

## 架构

完全复用现有 runtime metrics 管线，仅调整**聚合输出**与**前端三张图的数据来源**：

```
Flusher(5s, per pod, instanceID=hostname=pod名)
  → Redis ZSET（member=instanceID, score=unix秒）
  → ListInstances + ReadSnapshots（按 pod 读回快照）
  → Aggregate：QPS/P95/TPS/SuccessRate/SSE 保持跨 pod 聚合输出
  → aggregateInstances（新增）：CPU/Heap/Goroutines 逐 pod 输出曲线
  → GET /api/v1/metrics/runtime → monitor 页（三张图多曲线 + 卡片集群和）
```

## 组件设计

### 1. DTO（`internal/dto/metrics.go`）

`RuntimeSeries` 删除聚合字段 `goroutines` / `heapMB` / `cpuPercent`，新增 per-pod 结构：

```go
type RuntimeSeries struct {
    // QPS / P95Ms / SSEActive / TokenInput / TokenOutput / SuccessRate 保持不变（跨 pod 聚合）
    Instances map[string]RuntimeInstanceSeries `json:"instances"` // pod 名 → 该 pod 曲线
}

type RuntimeInstanceSeries struct {
    Goroutines []RuntimePoint `json:"goroutines"` // 单 pod gauge 桶内均值（不再跨实例求和）
    HeapMB     []RuntimePoint `json:"heapMB"`     // 单 pod 堆内存 MB
    CPUPercent []RuntimePoint `json:"cpuPercent"` // 单 pod CPU 使用率 %（0-100）
}
```

删除聚合字段的理由：前端不再绘制聚合线（决策 C），保留会造成 API 冗余与"集群求和"语义残留。

### 2. 聚合层（`internal/application/metrics/query/runtime_metrics.go`）

- **新增 `aggregateInstances`**：遍历 `byInstance`，对每个实例单独产出 `RuntimeInstanceSeries`（goroutines/heapMB 用现有 `instanceGauges` 的桶内均值，不跨实例求和；cpuPercent 用现有 `instanceDeltas` 的 cpu 正 delta ÷ 桶宽 × 100）。实例按名称排序输出（`slices.Sorted`），保证响应稳定、前端图例顺序确定。
- **`Aggregate` 改造**：保留 QPS / P95 / token / successRate / SSE 的跨 pod 聚合逻辑；`buildSeries` 中删除 goroutines/heapMB/cpuPercent 的输出分支。
- 空桶行为沿用：`samples == 0` 的桶不输出点（避免 gauge 塌成 0 断崖）。

### 3. 采集 / 缓存 / 上报层（`internal/infrastructure/metrics/`、`internal/infrastructure/cache/runtime_metrics.go`）

**零改动**。`Flusher` 已用 `os.Hostname()` 作为 instanceID（K8s 容器 hostname 默认即 pod 名，`k8s/deployment.yaml` 未覆盖 `spec.hostname`）；`Snapshot` 已含 Goroutines / HeapBytes / CPUSeconds 原值；`ListInstances` 返回的实例 ID 即 pod 名，直接作为 `Instances` map 的 key。

### 4. 单元测试（`test/unit/metrics/aggregate_test.go`）

- `TestAggregate_CrossPodSumRateAndP95` 更新：goroutines 断言由"跨 2 pod 求和 30"改为 per-instance（每 pod 15）；CPU% 断言由"跨 2 pod 求和 20"改为 per-instance（每 pod 10）。QPS/P95/token/successRate 聚合断言不变。
- 新增 per-instance 输出用例：两实例产生独立曲线、实例名排序稳定、空桶跳过。

### 5. Web 端

- **`web/src/lib/types.ts`**：`RuntimeSeries` 删除 `goroutines` / `heapMB` / `cpuPercent`，新增 `instances?: Record<string, RuntimeInstanceSeries>`；`RuntimeInstanceSeries` = `{ goroutines?, heapMB?, cpuPercent? }`。
- **`web/src/app/(dashboard)/monitor/page.tsx`**：
  - `SeriesState` 增加 `instances`，poll merge 按 pod 名逐实例合并（复用 `mergePoints` 模式）。
  - **CPU / Heap / Goroutines 三张图**：从 `instances` 构造每 pod 一列的多列数据 + 多 series（每 pod 一个，颜色循环，图例显示 pod 名）。`RuntimeChart` 已支持多 series（`sseActive` / TPS 已用），**零组件改动**。
  - **头部卡片**：
    - Goroutines = 各 pod `goroutines` 最新值求和，**四舍五入为整数**；
    - Heap = 各 pod `heapMB` 最新值求和（MB）；
    - SSE Active = 各 provider 最新值求和，**四舍五入为整数**。
  - 空状态（无活跃 pod）沿用 `monitor.collecting` 文案。
- **`web/src/locales/{en,zh,ja}.json`**：无需新增键（图表标题/文案不变；卡片 label 复用现有 `monitor.goroutines` / `monitor.heap` / `monitor.sse_active`）。

## 测试计划

| 层 | 位置 | 断言 |
|----|------|------|
| 单元 | `test/unit/metrics/aggregate_test.go` | per-instance goroutines/heapMB/cpuPercent 曲线；QPS/P95/token/successRate 聚合不变；空桶跳过；实例排序稳定 |
| 前端构建 | `web/` lint + build | 类型同步后无编译错误 |

## 边界与说明

- **CPU% 语义变化**：per-pod 曲线为单 pod 独立 0-100%，不再是集群求和（原聚合可能 >100%）。
- **图例 = pod 名**：即 hostname（K8s 下为 pod 名，`replicas: 2`）。若未来 deployment 覆盖 `spec.hostname`，图例名称会随之变化，但当前不涉及。
- **数据量**：per-pod 三曲线 × N pod，24h 档（桶宽 5m）每曲线 ≤288 点，JSON 体积可控。
- **`since` 增量拉取**：逻辑不变，per-pod 数据同样按桶增量合并。
- **pod 数量较多时曲线拥挤**：方案 C 已知取舍，颜色循环分配，不做额外折叠（YAGNI）。
