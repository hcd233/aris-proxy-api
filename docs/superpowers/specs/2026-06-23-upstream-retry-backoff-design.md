# 上游请求重试/退避机制设计

## 背景与动机

### 线上问题

traceId `dad63e23-0cb8-4a11-af31-a32926ce4a5d` 记录了一次典型的上游瞬时性故障：

- **请求**：OpenAI 协议流式请求，转发到 `opencode.ai/zen/go` 的 `glm-5.2` 模型
- **错误**：上游 Cloudflare 返回 **522 Connection timed out**（源站连接超时），等待约 21 秒
- **当前行为**：`doUpstreamRequest` 收到非 200 状态码后直接返回 `*model.UpstreamError`，无重试、无退避、无降级
- **影响**：流式响应的 200 头部已提前提交，无法改状态码，只能把 522 的 HTML 错误页作为 SSE data 帧写回流里，用户看到错误中断

### 问题根因

`internal/infrastructure/transport/openai.go` 的 `doUpstreamRequest` 和 `internal/infrastructure/transport/anthropic.go` 的 `sendRequest` 在收到非 200 状态码时直接返回错误，没有任何重试逻辑。全代码库扫描未发现任何上游请求重试机制。

522 这类 5xx 错误是瞬时性的——上游源站可能只是临时不可达，重试一次很可能成功。但当前代码放弃了这个机会。

## 需求

1. 对上游 5xx 瞬时错误和网络层错误实现自动重试
2. 仅在连接建立阶段重试（`doUpstreamRequest`/`sendRequest` 层面），不重试流式读取阶段
3. 指数退避 + jitter，参数可配置
4. OpenAI 和 Anthropic 两个 provider 都需要

## 设计

### 架构

在 `internal/infrastructure/transport/` 包内新增重试模块。将 `doUpstreamRequest`（OpenAI）和 `sendRequest`（Anthropic）的单次发送逻辑拆为独立的单次发送函数，外层包裹公共的 `sendUpstreamWithRetry` 重试循环。所有 Forward 方法签名不变，自动受益。

```
ForwardChatCompletionStream / ForwardChatCompletion / ...
  → doUpstreamRequest / sendRequest  (签名不变)
    → sendUpstreamWithRetry  (新增，重试循环)
      → 循环 attempt 0..maxAttempts:
         sendFn() → sendUpstreamRequestOnce / sendRequestOnce
           → 构建 req（bytes.NewReader(body) 每次新建）
           → 设置请求头（provider 差异在此）
           → httpclient.Do(req)
           → 网络错误 → UpstreamConnectionError
           → 非200 → UpstreamError
           → 200 → 存透传头 → return resp
         → 成功? return resp
         → isRetryable? no → return err
         → isRetryable? yes → log.Warn → backoff → 下一次
      → 耗尽 → return lastErr
```

### 组件

#### 1. 配置项（`internal/config/config.go`）

遵循现有 viper 模式，新增 4 个全局变量和对应的 `SetDefault` + 读取赋值：

| 全局变量 | viper key | 环境变量 | 类型 | 默认值 |
|---------|-----------|---------|------|--------|
| `UpstreamRetryMaxAttempts` | `upstream.retry.max_attempts` | `UPSTREAM_RETRY_MAX_ATTEMPTS` | int | `2` |
| `UpstreamRetryInitialBackoff` | `upstream.retry.initial_backoff` | `UPSTREAM_RETRY_INITIAL_BACKOFF` | time.Duration | `500ms` |
| `UpstreamRetryMaxBackoff` | `upstream.retry.max_backoff` | `UPSTREAM_RETRY_MAX_BACKOFF` | time.Duration | `2s` |
| `UpstreamRetryJitterFactor` | `upstream.retry.jitter_factor` | `UPSTREAM_RETRY_JITTER_FACTOR` | float64 | `0.3` |

在 `initEnvironment()` 中添加：
```go
config.SetDefault("upstream.retry.max_attempts", 2)
config.SetDefault("upstream.retry.initial_backoff", 500*time.Millisecond)
config.SetDefault("upstream.retry.max_backoff", 2*time.Second)
config.SetDefault("upstream.retry.jitter_factor", 0.3)
```

#### 2. 重试模块（`internal/infrastructure/transport/retry.go`）

**`sendUpstreamWithRetry`**

```go
func sendUpstreamWithRetry(
    ctx context.Context,
    module string,
    sendFn func() (*http.Response, error),
) (*http.Response, error)
```

- `module`：模块名（`"OpenAIProxy"` / `"AnthropicProxy"`），用于日志前缀
- `sendFn`：每次调用都重新构建请求并发送的闭包。`body []byte` 是原始字节数组，`bytes.NewReader(body)` 每次创建新 reader，所以闭包可重复调用
- 重试循环 `attempt 0..maxAttempts`（总共最多 `maxAttempts + 1` 次请求）
- 退避等待期间监听 `ctx.Done()`
- 重试耗尽后返回最后一次错误

**`isRetryableError`**

```go
func isRetryableError(err error) bool
```

判定逻辑：
- `*model.UpstreamConnectionError` → `true`（网络层错误：超时、连接拒绝等）
- `*model.UpstreamError` 且 `StatusCode >= 500` → `true`（5xx 瞬时错误）
- 其他 → `false`（4xx 永久错误、`ierr` 请求构建错误等）

**`calculateBackoff`**

```go
func calculateBackoff(
    attempt int,
    initial time.Duration,
    max time.Duration,
    jitterFactor float64,
) time.Duration
```

公式：
- `base = min(initial * 2^attempt, max)`
- `jitter = base * jitterFactor * (2 * rand() - 1)` 范围 `[-jitterFactor, +jitterFactor]`
- `backoff = base + jitter`

以默认值举例（initial=500ms, max=2s, jitter=0.3）：

| attempt | base | jitter 范围 | 实际 backoff 范围 |
|---------|------|------------|-----------------|
| 0 → 1 | 500ms | ±150ms | 350ms ~ 650ms |
| 1 → 2 | 1s | ±300ms | 700ms ~ 1.3s |
| 2 → 3 | 2s (capped) | ±600ms | 1.4s ~ 2.6s |

最大总等待时间（2 次重试）：约 650ms + 1.3s ≈ 2s（不含上游响应时间）。

#### 3. 改造现有方法

**`internal/infrastructure/transport/openai.go`**

- 原 `doUpstreamRequest` 的单次发送逻辑拆为 `sendUpstreamRequestOnce`（unexported method）
- 新 `doUpstreamRequest` 变为薄包装：
  ```go
  func (p *openAIProxy) doUpstreamRequest(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
      sendFn := func() (*http.Response, error) {
          return p.sendUpstreamRequestOnce(ctx, ep, body, pathSuffix)
      }
      return sendUpstreamWithRetry(ctx, "OpenAIProxy", sendFn)
  }
  ```
- `sendUpstreamRequestOnce` 内部移除非 200 时的 `log.Error`（改由 `sendUpstreamWithRetry` 统一记录），保留成功时的 `log.Info`

**`internal/infrastructure/transport/anthropic.go`**

- 同上模式：`sendRequest` → `sendRequestOnce` + 薄包装

### 错误处理

#### 可重试错误

| 错误类型 | 判定条件 | 示例 |
|---------|---------|------|
| `*model.UpstreamConnectionError` | 类型匹配 | 网络超时、连接拒绝、DNS 失败 |
| `*model.UpstreamError` | `StatusCode >= 500` | 502 Bad Gateway、503 Service Unavailable、522 Connection Timed Out |

#### 不可重试错误

| 错误类型 | 判定条件 | 示例 |
|---------|---------|------|
| `*model.UpstreamError` | `StatusCode < 500` | 400 Bad Request、401 Unauthorized、403 Forbidden、404 Not Found、429 Too Many Requests |
| `ierr.Wrap` 包装的错误 | 非 UpstreamError/UpstreamConnectionError | 请求构建失败 |

#### context 取消

退避等待期间监听 `ctx.Done()`。如果 context 被取消（如客户端断开连接或请求超时），立即中止重试，返回 `ctx.Err()`。

### 日志策略

| 事件 | 级别 | 消息格式 | 关键字段 |
|------|------|---------|---------|
| 首次发送 | Info | `[<module>] Send upstream request` | `upstreamURL`, `upstreamModel`, `upstreamAPIKey`, `upstreamHeaders`, `upstreamBody`（现有行为不变） |
| 可重试错误触发重试 | Warn | `[<module>] Upstream request failed, retrying` | `attempt`, `maxAttempts`, `backoff`, `error` |
| 退避等待被 context 取消 | Warn | `[<module>] Retry cancelled by context` | `attempt`, `error` |
| 重试耗尽（最终失败） | Error | `[<module>] Upstream returned non-200 status` | `upstreamURL`, `statusCode`, `responseBody`（最后一次的错误日志，保持现有格式） |
| 不可重试错误 | Error | `[<module>] Upstream returned non-200 status` | 同上，直接返回不重试 |

关键变更：**`sendUpstreamRequestOnce` / `sendRequestOnce` 内部不再记 Error 日志**（避免重试过程中产生多条 Error 日志），改为由 `sendUpstreamWithRetry` 在重试耗尽后统一记录最后一次的错误日志。

### context 超时考量

当前 HTTP Client 配置（`internal/infrastructure/httpclient/httpclient.go`）：
- `Client.Timeout`：5 分钟（LLM 流式响应总传输时长）
- `ResponseHeaderTimeout`：30 秒（仅约束首字节，不影响流式读取）

重试不修改这些超时。总重试时间受 context 控制——如果 context 在退避等待期被取消，立即返回 `ctx.Err()`。重试过程中的上游响应时间仍受 HTTP Client 的 Transport 超时约束。

## 测试策略

### 单元测试：`test/unit/transport/retry_test.go`

#### `isRetryableError` 测试

| 用例 | 输入 | 期望 |
|------|------|------|
| 522 可重试 | `&UpstreamError{StatusCode: 522}` | true |
| 502/503/504 可重试 | 各跑一遍 | true |
| 404 不可重试 | `&UpstreamError{StatusCode: 404}` | false |
| 429 不可重试 | `&UpstreamError{StatusCode: 429}` | false |
| 网络错误可重试 | `&UpstreamConnectionError{Cause: errors.New("timeout")}` | true |
| 其他错误不可重试 | `ierr.New(...)` | false |

#### `calculateBackoff` 测试

| 用例 | 验证点 |
|------|--------|
| 指数增长 | attempt 0 < 1 < 2 的 base 值 |
| max cap | 大 attempt 时不超过 maxBackoff |
| jitter 范围 | 多次采样，backoff 在 `[base*(1-j), base*(1+j)]` 范围内 |
| 零 jitter | 退避等于 base，无随机 |

#### `sendUpstreamWithRetry` 集成测试（用 `httptest.Server` mock 上游）

| 用例 | mock 行为 | 期望 |
|------|---------|------|
| 首次成功 | 200 | 返回 resp，无重试，调用 1 次 |
| 522 → 200 | 第一次 522，第二次 200 | 重试 1 次后成功，调用 2 次 |
| 522 × 3 → 耗尽 | 始终 522 | 返回最后 UpstreamError，调用 3 次（1 + maxAttempts=2） |
| 4xx 不重试 | 404 | 直接返回，无重试，调用 1 次 |
| 网络错误 → 200 | 第一次 server close，第二次 200 | 重试成功 |
| context 取消 | 522 + 可取消 ctx | 退避期取消 → 返回 ctx.Err() |

mock 上游用 `httptest.NewServer`，按调用次数返回不同状态码。用 `atomic.Int32` 计数 mock 调用次数，避免 `time.Sleep` 同步。退避参数在测试中设为极小值（如 `initial=1ms`）避免测试缓慢。

## 影响范围

### 新增文件

- `internal/infrastructure/transport/retry.go` — 重试模块
- `test/unit/transport/retry_test.go` — 单元测试
- `docs/superpowers/specs/2026-06-23-upstream-retry-backoff-design.md` — 本设计文档

### 修改文件

- `internal/config/config.go` — 新增 4 个重试配置项
- `internal/infrastructure/transport/openai.go` — 拆分 `doUpstreamRequest` 为薄包装 + `sendUpstreamRequestOnce`
- `internal/infrastructure/transport/anthropic.go` — 拆分 `sendRequest` 为薄包装 + `sendRequestOnce`

### 不变

- 所有 Forward 方法签名不变
- `*model.UpstreamError` 和 `*model.UpstreamConnectionError` 结构不变
- HTTP Client 配置不变
- `WriteUpstreamSSEError` 等上层错误处理不变
