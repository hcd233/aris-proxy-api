# Monitor 增加 Request TPS 与 Success Rate 图表 — 设计文档

> 日期：2026-08-01
> 状态：已评审（用户确认）

## 背景与目标

web 端 monitor 页面目前展示 6 个运行时图表（goroutines / heap / qps / cpuPercent / p95Ms / sseActive）。本次需求：

1. **Request TPS**：记录调用大模型服务时的 token 每秒速率，包含**输入**和**输出**两条曲线。
2. **Success Rate**：正确返回 200 的请求占所有请求的比例。

api 侧通过**扩展 `GET /api/v1/metrics/runtime` 响应字段**（`RuntimeSeries`）提供对应能力，不新增接口。

## 关键决策（已与用户确认）

| 决策点 | 选择 | 理由 |
|--------|------|------|
| TPS 统计范围 | **B：所有模型调用**（流式+非流式） | 与审计 `ModelCallAuditTask`、Token Rate Limiter 的 token 统计口径一致；实现改动最小 |
| Success Rate 口径 | **A：HTTP 网关层** | 复用现有 `HTTPCollector` 中间件，与 QPS 图同口径；跳过 health/metrics 等探活路径 |

## 架构

完全复用现有 runtime metrics 管线，仅扩展数据源、快照字段、聚合输出与前端图表：

```
[业务层] recordModelCall (token usage) ──┐
                                        ├→ Prometheus registry → Flusher(5s) → Redis ZSET → Aggregate → GET /api/v1/metrics/runtime → monitor 页
[采集层] HTTPCollector (HTTP 200 计数) ──┘
```

## 组件设计

### 1. 常量（`internal/common/constant/metrics.go`）

新增指标名与 label：

```go
// token 吞吐 counter
MetricNamespaceLLM = "llm"
MetricNameTokenUsage = "token_usage_total"        // 完整名 llm_token_usage_total
MetricFullTokenUsage = "llm_token_usage_total"
MetricLabelDirection = "direction"
TokenUsageDirectionInput  = "input"
TokenUsageDirectionOutput = "output"

// HTTP 请求结果 counter（Success Rate 数据源）
MetricNameRequests = "requests_total"             // 完整名 http_requests_total
MetricFullHTTPRequests = "http_requests_total"
MetricLabelResult = "result"
HTTPResultSuccess = "success"
HTTPResultFailure = "failure"
```

### 2. 采集层（`internal/infrastructure/metrics/`）

**新增 `TokenUsageCounter`**（仿 `SSEGauge` 模式）：

```go
type TokenUsageCounter struct { counter *prometheus.CounterVec } // llm_token_usage_total{direction}
func NewTokenUsageCounter(registry) *TokenUsageCounter            // 预置 input/output 两 label=0 序列
func (c *TokenUsageCounter) AddInput(n int64)
func (c *TokenUsageCounter) AddOutput(n int64)
```

**扩展 `HTTPCollector`**：

- 新增 `requests *prometheus.CounterVec`：`http_requests_total{result="success"|"failure"}`，预置两 label 序列。
- `Middleware` 中 `c.Next()` 返回后按 `c.Response().StatusCode() == 200` 计数；skip 逻辑不变（health/metrics 探活不计数，与 QPS 同口径）。

**扩展 `Snapshot`**（Redis 快照最小单位，仅存可相加原值）：

```go
TokenInput   float64 `json:"tokenInput"`   // counter 累计输入 token
TokenOutput  float64 `json:"tokenOutput"`  // counter 累计输出 token
ReqTotal     float64 `json:"reqTotal"`     // counter 累计业务请求数（success+failure）
ReqSuccess   float64 `json:"reqSuccess"`   // counter 累计 200 请求数
```

`BuildSnapshot` 抽取：`llm_token_usage_total{direction=input|output}`、`http_requests_total{result=success|failure}`（counter，带 label）。

### 3. 业务层 token 上报（`internal/application/llmproxy/usecase/`）

- **`recordModelCall`**（所有 forward 路径收尾的唯一 seam）中，在 `out.usage.apply(task)` 之后（此时 `task.InputTokens/OutputTokens` 已填充）上报 `AddInput/AddOutput`。
- 依赖注入：`openAIUseCase` / `anthropicUseCase` 增加 `tokenMetrics *metrics.TokenUsageCounter` 字段；`NewOpenAIUseCase` / `NewAnthropicUseCase` 增加参数；`recordModelCall` 及相关包级入口（`auditFailure`/`auditFailureWithProviders`）增加 counter 参数。
- `internal/bootstrap/modules/infra.go` 注册 `NewTokenUsageCounter`；`application.go` 传入 usecase 构造。

### 4. 聚合层（`internal/application/metrics/query/runtime_metrics.go`）

- `instanceDeltas` 扩展四组相邻快照正向 delta：`dTokenInput`、`dTokenOutput`、`dReqTotal`、`dReqSuccess`（复用 `nonNeg` reset clamp）。
- `bucketAgg` 新增：`tokenInputRate` / `tokenOutputRate`（delta÷桶宽，与 qps 同算法）、`reqTotal` / `reqSuccess`（跨实例求和，不除桶宽）。
- `buildSeries` 输出：
  - `tokenInput` / `tokenOutput`：tokens/sec 两条时序。
  - `successRate`：`reqSuccess/reqTotal*100`（%），`reqTotal==0` 的桶**不输出点**（避免误显 0%）。

### 5. DTO（`internal/dto/metrics.go`）

```go
type RuntimeSeries struct {
    // ...现有字段
    TokenInput  []RuntimePoint `json:"tokenInput"`  // 输入 token 速率 /s（跨 pod 求和）
    TokenOutput []RuntimePoint `json:"tokenOutput"` // 输出 token 速率 /s
    SuccessRate []RuntimePoint `json:"successRate"` // 成功率 %（0-100）
}
```

### 6. Web 端

- `web/src/lib/types.ts`：`RuntimeSeries` 增加 `tokenInput` / `tokenOutput` / `successRate`。
- `web/src/app/(dashboard)/monitor/page.tsx`：
  - `SeriesState` 增加三组数据；`poll` merge 逻辑扩展（复用 `mergePoints`）。
  - 新增图表：
    - **Request TPS**：双曲线（input/output），复用 `RuntimeChart` 多 series 能力，单位 tokens/s。
    - **Success Rate**：单曲线，unit `%`。
- `web/src/locales/{en,zh,ja}.json`：新增 `monitor.request_tps`、`monitor.request_tps_input`、`monitor.request_tps_output`、`monitor.success_rate` 键。

## 测试计划

| 层 | 位置 | 断言 |
|----|------|------|
| 单元 | `test/unit/metrics/aggregate_test.go` | 跨 pod 聚合 tokenInput/tokenOutput 速率与 successRate 比例；reqTotal=0 桶不输出 |
| 单元 | `test/unit/metrics/snapshot_test.go` | BuildSnapshot 抽取 token/request counter |
| 单元 | `test/unit/metrics/`（新） | TokenUsageCounter AddInput/AddOutput |
| 单元 | `test/unit/llmproxy_usecase/` | recordModelCall 上报 input/output token |
| 单元 | `test/unit/metrics/`（新） | HTTPCollector success/failure 计数 |
| E2E | `test/e2e/metrics/metrics_endpoint_test.go` | 响应 series 含 tokenInput/tokenOutput/successRate 字段 |

## 边界与说明

- Success Rate 与 QPS 同口径（HTTP 网关层，跳过探活路径），包含管理端接口与 LLM 代理接口。
- Token TPS 与审计/限流同口径（所有模型调用的 usage），流式在最终 chunk/message_delta 中携带 usage。
- 无数据桶（`samples==0`）不输出任何 series 点（沿用现有行为）；`reqTotal==0` 桶不输出 successRate 点。
