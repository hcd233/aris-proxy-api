# API 服务容错设计（降级/熔断/隔离）

## 背景与动机

当前 aris-proxy-api 对上游 LLM provider 的故障防护只有一层：`SendUpstreamWithRetry` 对网络错误和 5xx/429 做指数退避重试。缺少以下能力：

1. **无熔断**：上游持续故障时，每个请求都完整走完重试（默认最多 6 次发送 + 退避 ~2s），请求量放大且全部失败，白白消耗资源；故障恢复探测只能靠真实请求撞运气。
2. **无隔离**：单个慢上游可无限占用连接与协程，httpclient 是全局单例，一个上游卡死会拖累其他上游的请求。
3. **无降级**：故障期间客户端拿到的是超时/5xx 原始错误，没有明确的"上游暂时不可用，请稍后重试"信号。

本设计新增三个互补机制，全部位于 transport 层（`internal/infrastructure/transport/`），对上层 usecase/handler 透明：

- **熔断（Circuit Breaker）**：按上游 endpoint 统计滑动窗口错误率，超阈值后快速失败，周期性半开探测恢复。
- **隔离（Bulkhead）**：按上游 endpoint 限制最大并发转发数，防止慢上游耗尽进程资源。
- **降级（快速失败 503）**：熔断打开或 bulkhead 满载时，返回 HTTP 503 + `Retry-After` 头 + 协议格式错误体，让客户端明确退避。

## 方案比较

| 方案 | 熔断实现 | 隔离实现 | 优点 | 缺点 |
|------|---------|---------|------|------|
| **A. 自研轻量组件（推荐）** | 手写三态熔断器（~150 行） | 手写 channel 信号量（~50 行） | 无新依赖；与 `inflight.Tracker` 手写风格一致；指标钩子直接嵌入；行为完全可控 | 需自行充分测试 |
| B. 引入 sony/gobreaker | 成熟库 | 手写 | 熔断逻辑经过生产验证 | 新依赖；gobreaker 统计口径（连续失败/比例）与需求需适配；状态钩子需桥接指标 |
| C. 仅熔断，不做隔离 | 手写 | 无 | 改动最小 | 不满足"隔离"需求 |

**推荐方案 A**：需求本身简单（滑动窗口错误率 + 三态 + 半开探测），自研代码量小且与现有 `inflight.Tracker`、`retry.go` 的手写风格一致；bulkhead 用 buffered channel 是 Go 标准模式。引入外部库反而要适配其统计口径和状态回调。

## 设计

### 总体架构

组件分两层，熔断/bulkhead 本身**不依赖 LLM 领域类型**，任何出站调用点（proxy 链路、COS、未来的外部集成）都可接入：

```
┌─ internal/common/resilience/ ──────────────── 通用层（纯组件，不读 config、不依赖领域类型）
│   Breaker    — 按 string key 的三态熔断器（滑动窗口）
│   Semaphore  — 按 string key 的并发信号量（bulkhead）
│   Guard      — Allow(ctx, key) → (release, err) 组合入口；Report(key, success)
└──────────────────────────────────────────────
        ▲  proxy 适配层（internal/infrastructure/transport/guard.go）
        │   endpointKey(ep) → string；isCircuitError(err) → bool；
        │   NewEndpointGuard(registry) 读 config 全局变量组装 Guard
        │
doUpstreamRequest (openai.go) / sendRequest (anthropic.go)
  → guard.Allow(ctx, key)            # 熔断 Allow + bulkhead Acquire
  │    ├─ 熔断打开 → *model.CircuitOpenError → 503 + Retry-After
  │    └─ 信号量满（等待超时）→ *model.BulkheadFullError → 503 + Retry-After
  → SendUpstreamWithRetry(...)       # 现有：发送 + 指数退避重试
  → guard.Report(key, !isCircuitError(err))，defer release 信号量
```

- **key（proxy 适配层生成）**：`BaseURL + "|" + APIKey`（`vo.UpstreamEndpoint` 三个字段中的后两个），同源不同模型共享熔断与并发限制——源站故障时一起熔断是期望行为；不用 model 作 key 避免每模型一套计数器。其他接入方自定 key 语义（如 COS 用 bucket 名）。
- **作用域**：熔断只拦截"打开新上游连接"的请求。已建立的 SSE 流不受影响（符合语义——故障期间已有连接继续读完或按现有 SSE 排空逻辑结束）。流式请求在 `OpenChatCompletionStream` 等入口同样经过 `doUpstreamRequest`/`sendRequest`，自动受保护。

### 1. 熔断器（`internal/common/resilience/circuit_breaker.go`）

经典三态状态机，按 string key 注册实例（`sync.Map` 或 mutex map 懒创建）。组件不感知业务错误类型——成功/失败由调用方在 `Report(key, success bool)` 中判定，通用层只做计数。

```
Closed ──错误率 ≥ error_threshold 且窗口请求数 ≥ min_requests──→ Open
Open   ──等待 open_timeout──→ HalfOpen
HalfOpen ──探测请求成功（连续 halfopen_max_requests 个）──→ Closed
HalfOpen ──探测请求失败──→ Open（重新计时）
```

- **滑动窗口**：固定时间桶（window 分 6 桶，每桶 10s），只记录 success/failure 计数，无逐请求存储，无需清理逻辑。误差 ≤ 桶宽，可接受。
- **错误分类（proxy 适配层 `isCircuitError`，与 `IsRetryableError` 独立）**：

| 错误 | 计入熔断失败？ | 理由 |
|------|--------------|------|
| `*model.UpstreamConnectionError`（网络层） | ✅ | 连接超时/拒绝/DNS 失败是典型源站故障 |
| `*model.UpstreamError` StatusCode ≥ 500 | ✅ | 5xx 源站故障 |
| `*model.UpstreamError` 429 | ❌ | 上游存活，只是限流；熔断是"故障"保护而非"配额"保护 |
| 其他（请求构建错误等） | ❌ | 本地错误，与上游健康无关 |

- **熔断打开时**：`Allow(key)` 直接返回 `*model.CircuitOpenError`，携带 `RetryAfter`（剩余打开时间，向上取整秒）。
- **指标**（注册到现有 Prometheus registry，自动进监控快照）：
  - `upstream_circuit_state{key}` gauge：0=Closed, 1=Open, 2=HalfOpen
  - `upstream_circuit_open_total{key}` counter：打开次数
  - `upstream_circuit_rejected_total{key}` counter：熔断拒绝请求数
  - `upstream_bulkhead_rejected_total{key}` counter：bulkhead 等待超时被拒数

### 2. 隔离：bulkhead（`internal/common/resilience/bulkhead.go`）

- 每 key 一个 buffered channel 信号量（容量 = `max_concurrent`），`Acquire(ctx, key)` 尝试获取：
  - 立即可得 → 返回 release 函数
  - 不可得 → 等待至 `acquire_timeout`；超时返回 `*model.BulkheadFullError`
  - 等待期间监听 `ctx.Done()`
- release 是幂等闭包（`sync.Once` 防重复释放 panic）。
- **指标**：`upstream_bulkhead_rejected_total{key}` counter（等待超时被拒数）。

### 3. 降级：快速失败 503

`upstreamProxyError`（`internal/application/llmproxy/usecase/common.go:41`）新增对 `*model.CircuitOpenError` / `*model.BulkheadFullError` 的处理，映射为 `*port.ProxyError`：

| 场景 | HTTP 状态 | 响应头 | 错误体（按协议格式） |
|------|----------|--------|---------------------|
| 熔断打开 | 503 | `Retry-After: <剩余秒>` | OpenAI：`{"error":{"message":"上游服务暂时不可用，请稍后重试或更换模型","type":"circuit_open","code":503}}`；Anthropic：`{"type":"error","error":{"type":"overloaded_error","message":"..."}}` |
| bulkhead 满载 | 503 | `Retry-After: 5` | 同上，type 为 `bulkhead_full` / `overloaded_error` |

`port.ProxyError.Headers` 已支持自定义头，`WriteUpstreamError` 原样下发，无需改 handler。

### 4. 配置项（`internal/config/config.go`，沿用现有 viper 全局变量模式）

| 全局变量 | viper key | 环境变量 | 默认值 |
|---------|-----------|---------|--------|
| `UpstreamCircuitEnabled` | `upstream.circuit.enabled` | `UPSTREAM_CIRCUIT_ENABLED` | `true` |
| `UpstreamCircuitWindow` | `upstream.circuit.window` | `UPSTREAM_CIRCUIT_WINDOW` | `60s` |
| `UpstreamCircuitMinRequests` | `upstream.circuit.min_requests` | `UPSTREAM_CIRCUIT_MIN_REQUESTS` | `10` |
| `UpstreamCircuitErrorThreshold` | `upstream.circuit.error_threshold` | `UPSTREAM_CIRCUIT_ERROR_THRESHOLD` | `0.5` |
| `UpstreamCircuitOpenTimeout` | `upstream.circuit.open_timeout` | `UPSTREAM_CIRCUIT_OPEN_TIMEOUT` | `30s` |
| `UpstreamCircuitHalfOpenMaxRequests` | `upstream.circuit.halfopen_max_requests` | `UPSTREAM_CIRCUIT_HALFOPEN_MAX_REQUESTS` | `1` |
| `UpstreamBulkheadEnabled` | `upstream.bulkhead.enabled` | `UPSTREAM_BULKHEAD_ENABLED` | `true` |
| `UpstreamBulkheadMaxConcurrent` | `upstream.bulkhead.max_concurrent` | `UPSTREAM_BULKHEAD_MAX_CONCURRENT` | `32` |
| `UpstreamBulkheadAcquireTimeout` | `upstream.bulkhead.acquire_timeout` | `UPSTREAM_BULKHEAD_ACQUIRE_TIMEOUT` | `1s` |

默认值说明：窗口 60s / 错误率 50% / 最少 10 请求，避免小流量下误熔断；open_timeout 30s 与上游常见故障恢复时间匹配；并发 32 是保守上限，SSE 流式长连接场景下可防单上游占满全局连接池。

### 5. 与现有机制的关系

| 机制 | 关系 |
|------|------|
| `SendUpstreamWithRetry` | 熔断/bulkhead 在其**外层**拦截；通过后重试照旧。重试内的每次失败都 report 给熔断器——上游故障被重试掩盖（最终成功）不熔断，重试耗尽（最终失败）计入 |
| `IsRetryableError`（含 429 可重试） | 不变。429 仍重试但不熔断（熔断计数与重试判定是两个独立维度） |
| token 维度限流、请求限流 | 服务自身配额控制，独立于上游容错，互不影响 |
| `inflight.Tracker` 优雅退出 | 独立。熔断在 `CancelOnDrain(ctx)` 之后执行，退出期间新请求同样被熔断拦截，语义一致 |
| 全局 httpclient 超时 | 不变（`HTTP_CLIENT_TIMEOUT` 5min / `ResponseHeaderTimeout` 30s）。per-endpoint 超时需扩展数据模型（`UpstreamEndpoint` 加字段），收益低，不在本期范围 |

### 6. 内存与生命周期

- 熔断器/信号量 registry 按 key 懒创建，常驻进程生命周期。key 数量 = 配置的模型 endpoint 数量（个位数~几十），增长可控，不做 idle 清理（`// ponytail: registry, 如出现 endpoint 动态增删场景再做 GC`）。
- **依赖注入**：通用组件保持纯参数化（不读 config、不依赖领域类型）。proxy 适配层在 `internal/infrastructure/transport/guard.go` 提供 `NewEndpointGuard(registry *prometheus.Registry)`（registry 复用 `metrics.NewRegistry` 的 fx 实例，nil 时跳过指标注册；构造时从 config 全局变量读取参数组装 `resilience.Guard`）。`NewOpenAIProxy`/`NewAnthropicProxy` 签名追加 `guard *transport.EndpointGuard` 参数，在 `internal/bootstrap/modules/repository.go` 中接线。指标随 guard 构造注册。

## 测试策略

### 单元测试（`test/unit/resilience/`）

**熔断器**：
- 状态转换全路径：Closed→Open（错误率超阈且请求数达标）、Open→HalfOpen（计时到）、HalfOpen→Closed（探测成功）、HalfOpen→Open（探测失败重计时）
- 窗口滑动：旧窗口错误不影响当前判定（时间桶滚动）
- min_requests 保护：请求数不足不打开
- 错误分类：429 不计数、5xx/连接错误计数
- 半开探测数限制：HalfOpen 期间只放行 `halfopen_max_requests` 个

**bulkhead**：
- 并发上限：N 个并发获取，第 N+1 个等待；释放后可获取
- 等待超时：超时返回 BulkheadFullError
- ctx 取消：等待中被取消返回 ctx.Err()
- release 幂等：双 release 不 panic

### 集成测试（httptest mock 上游）

- 熔断打开后请求不达上游（mock 调用计数不变），返回 503 + Retry-After
- 重试耗尽计入熔断、重试成功不计入（失败 3 次后成功，窗口只记 3 次失败）
- 半开探测成功恢复（mock 恢复 200，后续请求正常放行）
- bulkhead 满载快速失败、proxy 适配层 `Allow`/`Report` 与重试组合行为

### E2E（`test/e2e/<topic>/`）

- 用可编程 mock 上游（或本地假 HTTP 服务）验证：持续 5xx → 熔断打开 → 请求 503 → 恢复 200 → 半开探测通过 → 全量恢复。E2E 用例沉淀到仓库，部署后运行。

## 影响范围

### 新增文件

- `internal/common/resilience/circuit_breaker.go` — 通用熔断器（状态机 + 滑动窗口 + key registry）
- `internal/common/resilience/bulkhead.go` — 通用信号量 bulkhead
- `internal/common/resilience/guard.go` — 通用 `Guard`（`Allow(ctx, key)` + `Report(key, success)` 组合）
- `internal/infrastructure/transport/guard.go` — proxy 适配层（`endpointKey`、`isCircuitError`、`NewEndpointGuard`）
- `test/unit/resilience/circuit_breaker_test.go`、`test/unit/resilience/bulkhead_test.go`、`test/unit/resilience/guard_test.go`
- `docs/superpowers/specs/2026-08-20-api-resilience-design.md` — 本设计文档
- E2E 用例目录 `test/e2e/upstream-resilience/`

### 修改文件

- `internal/common/model/error.go` — 新增 `CircuitOpenError`、`BulkheadFullError`
- `internal/config/config.go` — 新增 9 个配置项
- `internal/infrastructure/transport/openai.go` — `doUpstreamRequest` 接入 `Guard`
- `internal/infrastructure/transport/anthropic.go` — `sendRequest` 接入 `Guard`
- `internal/infrastructure/transport/retry.go` — 不变（Guard 在其外层）
- `internal/application/llmproxy/usecase/common.go` — `upstreamProxyError` 映射新错误 → 503 + Retry-After
- `internal/bootstrap/modules/repository.go` — fx 提供 `NewEndpointGuard`，`NewOpenAIProxy`/`NewAnthropicProxy` 接线 guard 参数

### 不变

- 所有 Forward 方法签名、`vo.UpstreamEndpoint` 数据模型、resolver、handler 层、SSE 流式读取
- 重试机制、限流、inflight、httpclient 全局配置

## 风险与边界

- **误熔断**：min_requests=10 + 阈值 50% 的组合已偏保守；若单 endpoint 流量极小（<10 请求/60s）则永不熔断——这是刻意的（样本不足时不做判断）。
- **bulkhead 拒绝体验**：并发 32 的流式长连接场景，等待 1s 超时后 503；用户可重试。数值均可配。
- **429 不熔断**：配额型上游若持续 429，现有重试会放大请求量（最多 6 次）；但 429 有 Retry-After 语义，属于客户端可感知的限流，不归属"故障"范畴。如需保护可在后续加"429 快速失败"策略，不在本期。
