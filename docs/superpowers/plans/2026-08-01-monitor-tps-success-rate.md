# Monitor TPS 与 Success Rate 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** monitor 页面新增 Request TPS（输入/输出双曲线）与 Success Rate（%）两个图表；api 侧扩展 `GET /api/v1/metrics/runtime` 响应字段。

**Architecture:** 复用现有 runtime metrics 管线：`recordModelCall` 上报 token 到 `TokenUsageCounter`（llm_token_usage_total{direction}）、`HTTPCollector` 中间件按 200 计数（http_requests_total{result}）→ Flusher 5s 快照（Snapshot 增 4 个 counter 字段）→ Redis → Aggregate（token 速率 + successRate 比例）→ RuntimeSeries 响应扩展 → monitor 前端两个图表。

**Tech Stack:** Go + Prometheus client + samber/lo；Next.js + Recharts + shadcn/ui。

**参考设计文档:** `docs/superpowers/specs/2026-08-01-monitor-tps-success-rate-design.md`

## Global Constraints

- 测试目录：`test/unit/<topic>/`、`test/e2e/<topic>/`；禁止 `internal/` 内测试。
- 测试/生产统一 `github.com/bytedance/sonic`；只用标准库 `testing`。
- 业务错误走 `internal/common/ierr`；HTTP 状态码用 `fiber.StatusXxx`。
- 日志 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`。
- 常量放 `internal/common/constant/metrics.go`；禁止业务包本地 const 块。
- 前端：调用走 `api-client.ts`；DTO 同步 `types.ts`；i18n 三语言同步；禁内联 fetch/style。
- 代码风格：samber/lo 函数式优先；ponytail full 级别最简实现。

---

### Task 1: 常量定义

**Files:**
- Modify: `internal/common/constant/metrics.go`

新增 token 吞吐与 HTTP 请求结果 counter 的指标名、label、枚举值（见设计文档第 1 节）。无独立测试（常量）。

### Task 2: 采集层 — TokenUsageCounter + HTTPCollector + Snapshot

**Files:**
- Modify: `internal/infrastructure/metrics/prometheus.go`（新增 TokenUsageCounter）
- Modify: `internal/infrastructure/metrics/collector.go`（HTTPCollector 加 requests CounterVec + 计数）
- Modify: `internal/infrastructure/metrics/snapshot.go`（Snapshot 4 字段 + BuildSnapshot 抽取）
- Modify: `internal/bootstrap/modules/infra.go`（注册 NewTokenUsageCounter）
- Test: `test/unit/metrics/snapshot_test.go`（扩展）

**Interfaces:**
- Produces: `metrics.TokenUsageCounter`（`AddInput(n int64)` / `AddOutput(n int64)`）；`Snapshot.TokenInput/TokenOutput/ReqTotal/ReqSuccess float64`；`NewTokenUsageCounter(registry) *TokenUsageCounter`

TDD：先写测试（BuildSnapshot 抽取 counter、TokenUsageCounter 累加、HTTPCollector 200/非 200 计数）→ 实现 → 绿。

### Task 3: 业务层 token 上报（recordModelCall）

**Files:**
- Modify: `internal/application/llmproxy/usecase/recorder.go`（recordModelCall/auditFailure/auditFailureWithProviders 增加 tokenMetrics 参数；apply 后上报 input/output）
- Modify: `internal/application/llmproxy/usecase/openai.go` / `anthropic.go`（struct 字段 + 构造参数）
- Modify: `internal/application/llmproxy/usecase/openai_chat.go` / `openai_response.go` / `anthropic_message.go` / `common.go`（调用点替换）
- Modify: `internal/bootstrap/modules/application.go`（传入 counter）
- Test: `test/unit/llmproxy_usecase/`（新增 recordModelCall 上报测试）

TDD：先写测试（recordModelCall 后 counter 值等于 input/output）→ 实现 → 绿。

### Task 4: 聚合层

**Files:**
- Modify: `internal/application/metrics/query/runtime_metrics.go`（instanceDeltas/bucketAgg/mergeRateBucket/buildSeries）
- Modify: `internal/dto/metrics.go`（RuntimeSeries 三字段）
- Test: `test/unit/metrics/aggregate_test.go`（扩展）

**Interfaces:**
- Consumes: `Snapshot.TokenInput/TokenOutput/ReqTotal/ReqSuccess`
- Produces: `dto.RuntimeSeries.TokenInput/TokenOutput/SuccessRate []RuntimePoint`

TDD：先写测试（跨 pod token 速率、successRate 比例、reqTotal=0 不输出）→ 实现 → 绿。

### Task 5: E2E 断言

**Files:**
- Modify: `test/e2e/metrics/metrics_endpoint_test.go`

断言响应 `series` 含 `tokenInput`/`tokenOutput`/`successRate` 字段。

### Task 6: Web 前端

**Files:**
- Modify: `web/src/lib/types.ts`（RuntimeSeries 三字段）
- Modify: `web/src/app/(dashboard)/monitor/page.tsx`（state/merge/两图表）
- Modify: `web/src/locales/{en,zh,ja}.json`（monitor.request_tps / request_tps_input / request_tps_output / success_rate）

**Interfaces:**
- Consumes: `api.getRuntimeMetrics` 返回的扩展 series

验证：`cd web && npm run lint && npm run build`。

### Task 7: 全量验证与提交

- `make test`（Go 全量）
- `make lint`
- 提交到 worktree 分支
