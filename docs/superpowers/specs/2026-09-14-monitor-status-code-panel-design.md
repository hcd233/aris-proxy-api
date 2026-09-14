# Monitor 状态码面板设计

> 日期：2026-09-14
> 分支：`feature/monitor-status-code-panel-2026-09-14`

## 背景与目标

monitor 面板当前的 **Success Rate** 面板展示"网关返回 HTTP 200 的请求占比"，数据源为运行时指标管线的 `http_requests_total{result=success|failure}` counter（仅区分 200 / 非 200），信息量低——看不出失败到底是 4xx（客户端/限流）还是 5xx（上游/自身）。

目标：把 Success Rate 面板替换为 **状态码面板**，按桶展示各 HTTP 状态码的请求数量趋势（折线，每状态码一条线）。

## 决策（已与需求方确认）

| 决策点 | 结论 |
|--------|------|
| 展示形式 | 折线，每状态码一条线，与 monitor 其他面板一致 |
| 状态码粒度 | 原始状态码（`200` / `401` / `404` / `429` / `500` …），不分类聚合 |
| 采集层 label 变更 | 接受把 `http_requests_total` 的 label 从 `result` 改为 `status_code`（仓库内无 Prometheus/Grafana/告警依赖该 label） |

## 数据流

```
HTTPCollector.Middleware (c.Next() 后)
  → http_requests_total{status_code="<code>"} counter
  → Flusher 每 5s BuildSnapshot → Snapshot.ReqStatus{code: 累计计数}
  → Redis（24h 留存）
  → RuntimeMetrics.Aggregate：相邻快照按 code 求正向 delta，落桶，跨 pod 求和
  → RuntimeSeries.StatusCodes{code: [{time, value}]}
  → GET /api/web/v1/metrics/runtime
  → monitor 状态码面板（折线 + 图例 + tooltip）
```

## 改动点

### 后端

1. `internal/common/constant/metrics.go`
   - `MetricLabelResult = "result"` → `MetricLabelStatusCode = "status_code"`。
   - 删除 `HTTPResultSuccess` / `HTTPResultFailure` 枚举（无其他引用）；`MetricRequestsHelp` 文案改为 "by status code"。
2. `internal/infrastructure/metrics/collector.go`
   - `requests` CounterVec 的 label 改为 `status_code`，中间件按 `strconv.Itoa(c.Response().StatusCode())` 自增。
   - 不再预置子序列（状态码集合无法枚举）；无流量时该 metric family 不输出，快照 `reqStatus` 为空，属预期。
3. `internal/infrastructure/metrics/snapshot.go`
   - `Snapshot.ReqTotal` / `ReqSuccess` → `ReqStatus map[string]float64`（`json:"reqStatus,omitempty"`，code → 累计计数）。
   - 新增 `labeledCounterValues`（按 label 取全部子序列）供 `BuildSnapshot` 使用；`labeledCounterValue`（单值，token 方向用）保留。
4. `internal/application/metrics/query/runtime_metrics.go`
   - `bucketAgg` / `instanceDelta` 的 `reqTotal`/`reqSuccess` → `reqStatus map[string]float64`。
   - `instanceDeltas` 按 code 对相邻快照求正向 delta（`nonNeg`，负值/clamp 逻辑不变）。
   - `buildSeries` 输出 `StatusCodes`；状态码集合取「输出窗口内出现过的 code」（`collectStatusCodes`），缺失桶补 0 保证曲线连续。
   - 删除 successRate 的计算与 `SuccessRate` 字段。
5. `internal/dto/metrics.go`
   - `RuntimeSeries` 删除 `SuccessRate`，新增 `StatusCodes map[string][]RuntimePoint \`json:"statusCodes"\``。

### 前端

6. `web/src/lib/types.ts`：`RuntimeSeries.successRate` → `statusCodes?: Record<string, RuntimePoint[]>`。
7. `web/src/app/(dashboard)/monitor/page.tsx`
   - `SeriesState.successRate: Pt[]` → `statusCodes: Record<string, Pt[]>`；合并逻辑复用 `mergeSSE`（按 key 增量合并时序）。
   - 原 `sseChartData` 泛化为 `seriesMapChartData`（`Record<string, Pt[]>` → 同一时间轴多列），SSE 与状态码共用。
   - 面板：`titleKey`/series label 用 `monitor.status_code`，动态 key + 调色板循环取色，无单位。
8. `web/src/locales/{en,zh,ja}.json`：`monitor.success_rate` → `monitor.status_code`（en "Status Code" / zh "状态码" / ja "ステータスコード"），保持三语 key 对齐。

### 文档

9. `CONTEXT.md`：把 `SuccessRate（请求成功率）` 词条替换为 `StatusCodeCounts（状态码数量）`，说明新口径与数据流（注意与审计侧 `dashboard.request_rate` 的成功率口径不同——后者来自审计表，不在本次改动范围）。

## 兼容性与边界

- **旧快照**：Redis 中 24h 内已写入的旧快照没有 `reqStatus` 字段，解码为 nil map；delta 循环对 nil map 天然安全（读为 0、反向 delta 被 `nonNeg` 截断为 0）。过渡期内状态码面板会短暂缺数据，不报错、不影响其他曲线。
- **监控口径**：业务错误按项目约定以 HTTP 200 + `{"error":{...}}` 下发，因此「状态码数量」不等于「业务错误数量」；429（限流）、413（BodyLimit）、404/405（路由）、502（上游连接失败）、上游透传状态码仍会如实反映在面板上。这一点在 CONTEXT.md 词条中写明，避免误读。
- **不回填**：历史桶（run 之前）无状态码数据，不做任何补算。
- **状态码基数**：HTTP 状态码取值有界，CounterVec 不会造成 label 基数爆炸。

## 测试策略

- 单元测试 `test/unit/metrics/aggregate_test.go`
  - 跨 pod 同 code 求和（200×3 + 500×1 双 pod → 200=6、500=2）。
  - 无请求桶不产生状态码曲线（`StatusCodes` 为空）。
  - 某 code 仅出现在部分桶时，其余桶补 0（曲线连续），窗口内未出现的 code 不输出。
  - 旧快照（`ReqStatus` nil）与新快照混排不 panic、不产生负数。
- 单元测试 `test/unit/metrics/snapshot_test.go`
  - `HTTPCollector` 打两个 200、一个 500 后，快照 `ReqStatus` 分别为 2 与 1。
- E2E `test/e2e/metrics/metrics_endpoint_test.go`：把断言 key 从 `successRate` 换成 `statusCodes`。
- 前端：`npm run lint && npm run test && npm run build`，并按 `next-dev-loop` 做一轮运行时验证。
