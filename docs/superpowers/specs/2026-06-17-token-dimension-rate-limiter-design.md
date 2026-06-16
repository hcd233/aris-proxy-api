# Token 维度限流中间件设计

## 背景与目标

当前 `internal/middleware/rate.go` 已实现基于 Redis 令牌桶的请求维度限流，每个 LLM 转发请求固定消耗 1 个令牌。本设计参考其 Lua 脚本与中间件结构，新增 **token 维度限流中间件**，对 OpenAI/Anthropic 大模型转发链路按实际 token 用量做限流，默认按 **1,000,000 TPM** 生效。

## 关键决策

| 决策项 | 选择 | 说明 |
|--------|------|------|
| 扣减时机 | 请求后扣减 | 实际 token 用量必须等上游响应返回后才能拿到。 |
| usage 采集 | Usecase 通过 context 上报 | 中间件注入 `TokenUsageReporter`，Usecase 在拿到 usage 后调用 `Report`；准确、稳定、易覆盖流式与协议转换。 |
| 桶模型 | 与现有 rate 中间件一致，单桶可配置 | 不区分 TPS/TPM，仅通过 `period` 与 `capacity` 区分。路由注册时决定窗口。 |
| total token 计算 | input + output | 不包含 cache 相关 token；由 usage DTO 提供统一的 `TotalTokens()` 方法。 |
| 默认配额 | 1,000,000 TPM | 对 `/chat/completions`、`/responses`、`/messages` 三个转发接口默认生效。 |
| 与现有请求限流关系 | 叠加 | 保留现有 `TokenBucketRateLimiterMiddleware`，新增 token 维度中间件同时挂载。 |

## 组件设计

### 1. TokenUsageReporter 接口

新增 `internal/common/ratelimit/reporter.go`：

```go
package ratelimit

import "context"

type TokenUsageReporter interface {
    Report(ctx context.Context, tokens int64)
}
```

`internal/common/constant/ctx.go` 新增 context key：

```go
CtxKeyTokenUsageReporter enum.CtxKey = "tokenUsageReporter"
```

### 2. Token 维度限流中间件

新增 `internal/middleware/token_rate.go`，签名与现有请求维度限流保持一致：

```go
func TokenBucketTokenRateLimiterMiddleware(
    cache *redis.Client,
    serviceName string,
    key enum.CtxKey,
    period time.Duration,
    capacity int64,
) func(ctx huma.Context, next func(huma.Context))
```

行为：

1. 解析限流维度（同现有中间件：key 为空按 IP，否则从 context 取）。
2. Redis key 沿用 `tb:{serviceName}:{keyValue}:{value}` 模板。
3. 请求前执行 **peek Lua 脚本**：读取当前桶内令牌数并 refill，不扣减。
   - 若 `tokens <= 0`：返回 `429 Too Many Requests`，设置 `Retry-After`。
   - 否则继续，设置 `X-RateLimit-Limit` / `X-RateLimit-Remaining` 响应头。
4. 创建 `tokenUsageReporter` 实例并通过 `huma.WithValue` 注入 context。
5. 调用 `next(ctx)`。

提供两个 Lua 脚本：

- `tokenBucketPeekLua`：原子性地 refill 并返回当前令牌数，不修改数量。
- `tokenBucketDeductLua`：原子性地 refill 并按实际 usage 扣减 `cost` 个令牌，允许桶 transient 为负值。

### 3. Usage DTO 统一 TotalTokens 方法

为三种 usage 结构体各新增 `TotalTokens() int64`，统一按 input + output 计算：

- `internal/dto/openai/chat.go`：`OpenAICompletionUsage.TotalTokens()` = `PromptTokens + CompletionTokens`
- `internal/dto/openai/response_rsp.go`：`ResponseUsage.TotalTokens()` = `InputTokens + OutputTokens`
- `internal/dto/anthropic/anthropic.go`：`AnthropicUsage.TotalTokens()` = `InputTokens + OutputTokens`

审计任务与限流 reporter 都使用这些 DTO 方法，避免从 audit task 反向推导 total。

### 4. Usecase 上报点

在 `internal/application/llmproxy/usecase` 各 forward 路径中，拿到上游 usage 后、提交 audit task 前调用：

```go
reportTokenUsage(ctx, usage.TotalTokens())
```

新增 helper：

```go
func reportTokenUsage(ctx context.Context, tokens int64) {
    reporter, ok := ctx.Value(constant.CtxKeyTokenUsageReporter).(ratelimit.TokenUsageReporter)
    if !ok || reporter == nil || tokens <= 0 {
        return
    }
    reporter.Report(ctx, tokens)
}
```

覆盖路径：

- `openai_chat.go`：native stream/unary、via-anthropic stream/unary
- `openai_response.go`：native stream/unary、via-chat、via-anthropic
- `anthropic_message.go`：native stream/unary、via-openai-chat、via-openai-response

### 5. 路由挂载

在 `internal/router/openai.go` 与 `internal/router/anthropic.go` 的转发路由上叠加新的中间件，默认 TPM：

```go
Middlewares: huma.Middlewares{
    middleware.TokenBucketRateLimiterMiddleware(
        cache,
        "callProxyLLMToken",
        constant.CtxKeyAPIKeyID,
        constant.PeriodCallProxyLLMToken,
        constant.LimitCallProxyLLMToken,
    ),
},
```

新增常量：

```go
const (
    PeriodCallProxyLLMToken = 1 * time.Minute
    LimitCallProxyLLMToken  = 1000000
)
```

## 数据流

```
客户端请求
    ↓
APIKeyMiddleware / HeaderPassthroughMiddleware
    ↓
TokenBucketTokenRateLimiterMiddleware
    ├─ peek 桶：depleted → 429
    ├─ ok → 注入 TokenUsageReporter → next(ctx)
    ↓
Handler → Usecase → 上游 LLM
    ↓
Usecase 拿到 usage
    ├─ usage.TotalTokens() → reportTokenUsage(ctx, total)
    │       └─ reporter.Report → Redis deduct Lua
    └─ SubmitModelCallAuditTask
    ↓
返回响应
```

## 边界与容错

- **桶可 transient 为负**：post-call 扣减无法阻止单次调用超量，但会让后续请求被拒，直到 refill。
- **Reporter 上报失败**：按 best-effort 处理，不阻塞已完成的响应返回。
- **无 reporter 或 tokens <= 0**：跳过扣减。
- **Retry-After**：按 `ceil(1 / refillRate)` 秒计算。
- **与现有请求限流共存**：两个中间件各自维护独立的 Redis key，互不影响。

## 测试计划

新增 `test/unit/token_rate_limiter/token_rate_limiter_test.go`：

- 基于 `miniredis` 构造 Redis。
- 验证 peek/deduct Lua 脚本的 refill 与扣减逻辑。
- 验证桶耗尽后返回 429。
- 验证 reporter 扣减后桶余额正确。
- 验证 `TotalTokens()` 方法在三种 usage DTO 上按 input + output 计算。

## 影响文件

- 新增
  - `internal/common/ratelimit/reporter.go`
  - `internal/middleware/token_rate.go`
  - `test/unit/token_rate_limiter/token_rate_limiter_test.go`
- 修改
  - `internal/common/constant/ctx.go`
  - `internal/common/constant/ratelimit.go`
  - `internal/dto/openai/chat.go`
  - `internal/dto/openai/response_rsp.go`
  - `internal/dto/anthropic/anthropic.go`
  - `internal/application/llmproxy/usecase/common.go`
  - `internal/application/llmproxy/usecase/openai_chat.go`
  - `internal/application/llmproxy/usecase/openai_response.go`
  - `internal/application/llmproxy/usecase/anthropic_message.go`
  - `internal/router/openai.go`
  - `internal/router/anthropic.go`
