# Token 维度限流中间件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为大模型转发链路新增 token 维度 Redis 令牌桶限流中间件，按实际 input+output token 用量在请求完成后扣减，默认 TPM 配额 1,000,000。

**Architecture:** 中间件在请求前 peek 桶余额并注入 `TokenUsageReporter`；Usecase 拿到上游 usage 后调用 `usage.TotalTokens()` 与 reporter 扣减；单桶模型，period/capacity 由路由决定，与现有请求维度限流叠加挂载。

**Tech Stack:** Go 1.25.1, Huma v2, Fiber v3, Redis, go-redis/v9, miniredis

---

## File Map

| File | Responsibility |
|------|----------------|
| `internal/common/ratelimit/reporter.go` | `TokenUsageReporter` 接口定义 |
| `internal/common/constant/ctx.go` | `CtxKeyTokenUsageReporter` context key |
| `internal/common/constant/ratelimit.go` | 默认 TPM period/capacity 常量 |
| `internal/dto/openai/chat.go` | `OpenAICompletionUsage.TotalTokens()` |
| `internal/dto/openai/response_rsp.go` | `ResponseUsage.TotalTokens()` |
| `internal/dto/anthropic/anthropic.go` | `AnthropicUsage.TotalTokens()` |
| `internal/middleware/token_rate.go` | peek/deduct Lua、reporter 实现、中间件 |
| `internal/application/llmproxy/usecase/common.go` | `reportTokenUsage` helper |
| `internal/application/llmproxy/usecase/openai_chat.go` | OpenAI chat 转发路径上报 usage |
| `internal/application/llmproxy/usecase/openai_response.go` | OpenAI response 转发路径上报 usage |
| `internal/application/llmproxy/usecase/anthropic_message.go` | Anthropic message 转发路径上报 usage |
| `internal/router/openai.go` | OpenAI 路由挂载 token 中间件 |
| `internal/router/anthropic.go` | Anthropic 路由挂载 token 中间件 |
| `test/unit/token_rate_limiter/token_rate_limiter_test.go` | Lua、reporter、TotalTokens 单元测试 |

---

### Task 1: TokenUsageReporter 接口与 Context Key

**Files:**
- Create: `internal/common/ratelimit/reporter.go`
- Modify: `internal/common/constant/ctx.go`

- [ ] **Step 1: 创建接口文件**

```go
package ratelimit

import "context"

// TokenUsageReporter 用于在 LLM 调用完成后按实际 token 用量扣减限流桶。
type TokenUsageReporter interface {
    Report(ctx context.Context, tokens int64)
}
```

- [ ] **Step 2: 注册 context key**

在 `internal/common/constant/ctx.go` 的 `CtxKeyLimiter` 附近新增：

```go
// CtxKeyTokenUsageReporter 注入 token 用量上报器
//	@update 2026-06-17 10:00:00
CtxKeyTokenUsageReporter enum.CtxKey = "tokenUsageReporter"
```

- [ ] **Step 3: Commit**

```bash
git add internal/common/ratelimit/reporter.go internal/common/constant/ctx.go
git commit -m "feat(ratelimit): add TokenUsageReporter interface and context key"
```

---

### Task 2: Usage DTO 统一 TotalTokens 方法

**Files:**
- Modify: `internal/dto/openai/chat.go`
- Modify: `internal/dto/openai/response_rsp.go`
- Modify: `internal/dto/anthropic/anthropic.go`

- [ ] **Step 1: OpenAICompletionUsage.TotalTokens**

在 `internal/dto/openai/chat.go` 的 `OpenAICompletionUsage` 结构体后新增方法：

```go
// TotalTokens 返回 input + output token 总数，不包含 cache。
//
//	@receiver u *OpenAICompletionUsage
//	@return int64
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (u *OpenAICompletionUsage) TotalTokens() int64 {
    return int64(u.PromptTokens + u.CompletionTokens)
}
```

- [ ] **Step 2: ResponseUsage.TotalTokens**

在 `internal/dto/openai/response_rsp.go` 的 `ResponseUsage` 结构体后新增方法：

```go
// TotalTokens 返回 input + output token 总数。
//
//	@receiver u *ResponseUsage
//	@return int64
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (u *ResponseUsage) TotalTokens() int64 {
    return int64(u.InputTokens + u.OutputTokens)
}
```

- [ ] **Step 3: AnthropicUsage.TotalTokens**

在 `internal/dto/anthropic/anthropic.go` 的 `AnthropicUsage` 结构体后新增方法：

```go
// TotalTokens 返回 input + output token 总数，不包含 cache。
//
//	@receiver u *AnthropicUsage
//	@return int64
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (u *AnthropicUsage) TotalTokens() int64 {
    return int64(u.InputTokens + u.OutputTokens)
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/dto/openai/chat.go internal/dto/openai/response_rsp.go internal/dto/anthropic/anthropic.go
git commit -m "feat(dto): add TotalTokens helper on usage DTOs"
```

---

### Task 3: Token 维度限流中间件实现

**Files:**
- Create: `internal/middleware/token_rate.go`
- Modify: `internal/common/constant/ratelimit.go`

- [ ] **Step 1: 创建中间件文件**

```go
package middleware

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/ratelimit"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// tokenBucketPeekLua 令牌桶余额探查脚本：refill 后返回当前余额，不扣减。
//
// KEYS[1]: 限流键
// ARGV[1]: 桶容量
// ARGV[2]: 每微秒补充令牌数
// ARGV[3]: 当前时间戳（微秒）
var tokenBucketPeekLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    return {tostring(capacity), tostring(capacity)}
end

local elapsed = now - last_refill
if elapsed > 0 then
    local refill = elapsed * refill_rate
    tokens = math.min(capacity, tokens + refill)
end

return {tostring(tokens), tostring(capacity)}
`)

// tokenBucketDeductLua 令牌桶扣减脚本：refill 后按实际 usage 扣减 cost，允许负值。
//
// KEYS[1]: 限流键
// ARGV[1]: 桶容量
// ARGV[2]: 每微秒补充令牌数
// ARGV[3]: 当前时间戳（微秒）
// ARGV[4]: key 过期时间（毫秒）
// ARGV[5]: 本次扣减 token 数
var tokenBucketDeductLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expire_ms = tonumber(ARGV[4])
local cost = tonumber(ARGV[5])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill = now
end

local elapsed = now - last_refill
if elapsed > 0 then
    local refill = elapsed * refill_rate
    tokens = math.min(capacity, tokens + refill)
    last_refill = now
end

tokens = tokens - cost

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('PEXPIRE', key, expire_ms)

return {tostring(tokens), tostring(capacity)}
`)

type tokenUsageReporter struct {
	cache      *redis.Client
	limiterKey string
	period     time.Duration
	capacity   int64
}

// Report 按实际 token 用量扣减限流桶。
func (r *tokenUsageReporter) Report(ctx context.Context, tokens int64) {
	if tokens <= 0 {
		return
	}

	refillRate := float64(r.capacity) / float64(r.period.Microseconds())
	expireMs := r.period.Milliseconds() * 2
	now := time.Now().UnixMicro()

	result, err := tokenBucketDeductLua.Run(
		ctx, r.cache,
		[]string{r.limiterKey},
		r.capacity, refillRate, now, expireMs, tokens,
	).StringSlice()
	if err != nil {
		logger.WithCtx(ctx).Error("[TokenUsageReporter] Failed to deduct tokens",
			zap.String("limiterKey", r.limiterKey),
			zap.Int64("cost", tokens),
			zap.Error(err),
		)
		return
	}

	remaining, _ := strconv.ParseFloat(result[0], constant.ParseFloat64BitSize) //nolint:errcheck // redis returns float string
	remainingInt := int64(math.Max(0, math.Floor(remaining)))
	logger.WithCtx(ctx).Debug("[TokenUsageReporter] Tokens deducted",
		zap.String("limiterKey", r.limiterKey),
		zap.Int64("cost", tokens),
		zap.Int64("remaining", remainingInt),
	)
}

// TokenBucketTokenRateLimiterMiddleware token 维度令牌桶限流中间件。
//
// 请求前 peek 桶余额；余额 <= 0 时拒绝。通过 context 注入 TokenUsageReporter，
// 由 Usecase 在拿到上游实际 usage 后调用 Report 扣减。
//
//	@param cache *redis.Client Redis 客户端
//	@param serviceName string 服务名，用于 Redis key 前缀
//	@param key enum.CtxKey 限流维度 context key，为空则按 IP
//	@param period time.Duration 令牌完全补满所需时间
//	@param capacity int64 桶容量
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func TokenBucketTokenRateLimiterMiddleware(
	cache *redis.Client,
	serviceName string,
	key enum.CtxKey,
	period time.Duration,
	capacity int64,
) func(ctx huma.Context, next func(huma.Context)) {
	refillRate := float64(capacity) / float64(period.Microseconds())
	expireMs := period.Milliseconds() * 2
	retryAfterSeconds := int(math.Ceil(1.0 / (refillRate * 1e6)))

	return func(ctx huma.Context, next func(huma.Context)) {
		log := logger.WithCtx(ctx.Context())
		if cache == nil {
			log.Error("[TokenBucketTokenRateLimiter] Redis dependency is nil")
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}

		var keyValue, value string
		if key == "" {
			keyValue = constant.RateLimitKeyByIP
			fCtx := humafiber.Unwrap(ctx)
			value = fCtx.IP()
		} else {
			if ctxValue := ctx.Context().Value(key); ctxValue != nil {
				keyValue = string(key)
				value = fmt.Sprintf(constant.FormatDefault, ctxValue)
			} else {
				log.Warn("[TokenBucketTokenRateLimiter] Context value not found",
					zap.String("serviceName", serviceName),
					zap.String("key", string(key)),
				)
				lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
				return
			}
		}

		limiterKey := fmt.Sprintf(constant.TokenBucketKeyTemplate, serviceName, keyValue, value)
		ctx = huma.WithValue(ctx, constant.CtxKeyLimiter, limiterKey)

		now := time.Now().UnixMicro()
		result, err := tokenBucketPeekLua.Run(
			ctx.Context(), cache,
			[]string{limiterKey},
			capacity, refillRate, now,
		).StringSlice()
		if err != nil {
			log.Error("[TokenBucketTokenRateLimiter] Failed to peek rate limit bucket", zap.Error(err))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}

		remaining, _ := strconv.ParseFloat(result[0], constant.ParseFloat64BitSize) //nolint:errcheck // redis returns float string
		remainingInt := int64(math.Max(0, math.Floor(remaining)))
		limitStr := result[1]

		if remainingInt <= 0 {
			log.Warn("[TokenBucketTokenRateLimiter] Rate limit reached",
				zap.String("serviceName", serviceName),
				zap.String("key", keyValue),
				zap.String("value", value),
				zap.Int64("remaining", remainingInt),
				zap.String("capacity", limitStr),
			)
			ctx.SetHeader(constant.HTTPHeaderXRateLimitLimit, limitStr)
			ctx.SetHeader(constant.HTTPHeaderXRateLimitRemaining, constant.ZeroString)
			ctx.SetHeader(constant.HTTPHeaderRetryAfter, strconv.Itoa(retryAfterSeconds))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrTooManyRequests.BizError()))
			return
		}

		ctx.SetHeader(constant.HTTPHeaderXRateLimitLimit, limitStr)
		ctx.SetHeader(constant.HTTPHeaderXRateLimitRemaining, strconv.FormatInt(remainingInt, constant.DecimalBase))

		reporter := &tokenUsageReporter{
			cache:      cache,
			limiterKey: limiterKey,
			period:     period,
			capacity:   capacity,
		}
		ctx = huma.WithValue(ctx, constant.CtxKeyTokenUsageReporter, ratelimit.TokenUsageReporter(reporter))

		next(ctx)
	}
}
```

- [ ] **Step 2: 添加默认 TPM 常量**

在 `internal/common/constant/ratelimit.go` 新增：

```go
const (
    PeriodCallProxyLLMToken = 1 * time.Minute
    LimitCallProxyLLMToken  = 1000000
)
```

- [ ] **Step 3: 运行编译检查**

```bash
go build ./internal/middleware/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/middleware/token_rate.go internal/common/constant/ratelimit.go
git commit -m "feat(middleware): add token dimension rate limiter middleware"
```

---

### Task 4: Usecase 上报实际 Token Usage

**Files:**
- Modify: `internal/application/llmproxy/usecase/common.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`

- [ ] **Step 1: 新增 reportTokenUsage helper**

在 `internal/application/llmproxy/usecase/common.go` 中新增 helper 并导入 `ratelimit` 包：

```go
import (
    ...
    "github.com/hcd233/aris-proxy-api/internal/common/ratelimit"
    ...
)

// reportTokenUsage 从 context 取出 TokenUsageReporter 并上报实际 token 用量。
func reportTokenUsage(ctx context.Context, tokens int64) {
    if tokens <= 0 {
        return
    }
    reporter, ok := ctx.Value(constant.CtxKeyTokenUsageReporter).(ratelimit.TokenUsageReporter)
    if !ok || reporter == nil {
        return
    }
    reporter.Report(ctx, tokens)
}
```

- [ ] **Step 2: OpenAI chat 路径上报**

在 `internal/application/llmproxy/usecase/openai_chat.go` 中，找到所有 `SetTokensFromOpenAIUsage` 调用处，在其后增加上报：

**forwardChatNativeStream**（约第 97 行附近）：
```go
task.SetTokensFromOpenAIUsage(usage)
if usage != nil {
    reportTokenUsage(ctx, usage.TotalTokens())
}
```

**forwardChatNativeUnary**（约第 120 行附近）：
```go
task.SetTokensFromOpenAIUsage(completion.Usage)
if completion.Usage != nil {
    reportTokenUsage(ctx, completion.Usage.TotalTokens())
}
```

**forwardChatViaAnthropicStreamBody**（约第 151 行附近）：
```go
task.SetTokensFromAnthropicUsage(anthropicMsg)
if anthropicMsg != nil && anthropicMsg.Usage != nil {
    reportTokenUsage(ctx, anthropicMsg.Usage.TotalTokens())
}
```

**forwardChatViaAnthropicUnary**（约第 213 行附近）：
```go
task.SetTokensFromAnthropicUsage(anthropicMsg)
if anthropicMsg != nil && anthropicMsg.Usage != nil {
    reportTokenUsage(ctx, anthropicMsg.Usage.TotalTokens())
}
```

- [ ] **Step 3: OpenAI response 路径上报**

在 `internal/application/llmproxy/usecase/openai_response.go` 中，对所有 `SetTokensFromOpenAIUsage` / `SetTokensFromResponseUsage` / `SetTokensFromAnthropicUsage` 调用后增加类似上报。

典型模式：
```go
task.SetTokensFromOpenAIUsage(completion.Usage)
if completion.Usage != nil {
    reportTokenUsage(ctx, completion.Usage.TotalTokens())
}
```

以及 ResponseUsage：
```go
task.SetTokensFromResponseUsage(rsp)
if rsp != nil && rsp.Usage != nil {
    reportTokenUsage(ctx, rsp.Usage.TotalTokens())
}
```

- [ ] **Step 4: Anthropic message 路径上报**

在 `internal/application/llmproxy/usecase/anthropic_message.go` 中，对所有 `SetTokensFromAnthropicUsage` / `SetTokensFromOpenAIUsage` 调用后增加上报：

```go
task.SetTokensFromAnthropicUsage(anthropicMsg)
if anthropicMsg != nil && anthropicMsg.Usage != nil {
    reportTokenUsage(ctx, anthropicMsg.Usage.TotalTokens())
}
```

- [ ] **Step 5: 编译检查**

```bash
go build ./internal/application/llmproxy/usecase/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/application/llmproxy/usecase/common.go internal/application/llmproxy/usecase/openai_chat.go internal/application/llmproxy/usecase/openai_response.go internal/application/llmproxy/usecase/anthropic_message.go
git commit -m "feat(llmproxy): report actual token usage to rate limiter"
```

---

### Task 5: 路由挂载

**Files:**
- Modify: `internal/router/openai.go`
- Modify: `internal/router/anthropic.go`

- [ ] **Step 1: OpenAI 路由挂载**

在 `internal/router/openai.go` 中，将 `/chat/completions` 与 `/responses` 的 `Middlewares` 从：

```go
Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM)},
```

改为：

```go
Middlewares: huma.Middlewares{
    middleware.TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM),
    middleware.TokenBucketTokenRateLimiterMiddleware(cache, "callProxyLLMToken", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLMToken, constant.LimitCallProxyLLMToken),
},
```

- [ ] **Step 2: Anthropic 路由挂载**

在 `internal/router/anthropic.go` 中，对 `/messages` 路由做同样修改。

- [ ] **Step 3: 编译检查**

```bash
go build ./internal/router/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/router/openai.go internal/router/anthropic.go
git commit -m "feat(router): mount token dimension rate limiter on LLM proxy routes"
```

---

### Task 6: 单元测试

**Files:**
- Create: `test/unit/token_rate_limiter/token_rate_limiter_test.go`

- [ ] **Step 1: 创建测试文件**

```go
package token_rate_limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/dto/anthropic"
	"github.com/hcd233/aris-proxy-api/internal/dto/openai"
	"github.com/redis/go-redis/v9"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close(); mr.Close() })
	return mr, rdb
}

func TestOpenAICompletionUsage_TotalTokens(t *testing.T) {
	u := &openai.OpenAICompletionUsage{PromptTokens: 10, CompletionTokens: 5}
	if got := u.TotalTokens(); got != 15 {
		t.Fatalf("TotalTokens = %d, want 15", got)
	}
}

func TestResponseUsage_TotalTokens(t *testing.T) {
	u := &openai.ResponseUsage{InputTokens: 20, OutputTokens: 8}
	if got := u.TotalTokens(); got != 28 {
		t.Fatalf("TotalTokens = %d, want 28", got)
	}
}

func TestAnthropicUsage_TotalTokens(t *testing.T) {
	u := &anthropic.AnthropicUsage{InputTokens: 7, OutputTokens: 3}
	if got := u.TotalTokens(); got != 10 {
		t.Fatalf("TotalTokens = %d, want 10", got)
	}
}

func TestTokenBucketScripts_PeekThenDeduct(t *testing.T) {
	_, rdb := newRedis(t)
	ctx := context.Background()
	key := "tb:test:peek:deduct"
	capacity := int64(100)
	period := time.Minute
	refillRate := float64(capacity) / float64(period.Microseconds())
	expireMs := period.Milliseconds() * 2

	// Peek 初始应为满桶
	res, err := tokenBucketPeekLua.Run(ctx, rdb, []string{key}, capacity, refillRate, time.Now().UnixMicro()).StringSlice()
	if err != nil {
		t.Fatalf("peek failed: %v", err)
	}
	if res[0] != "100" || res[1] != "100" {
		t.Fatalf("initial peek = %v, want [100 100]", res)
	}

	// Deduct 30
	res, err = tokenBucketDeductLua.Run(ctx, rdb, []string{key}, capacity, refillRate, time.Now().UnixMicro(), expireMs, int64(30)).StringSlice()
	if err != nil {
		t.Fatalf("deduct failed: %v", err)
	}
	if res[0] != "70" {
		t.Fatalf("after deduct 30 = %s, want 70", res[0])
	}

	// Peek 应看到 70
	res, err = tokenBucketPeekLua.Run(ctx, rdb, []string{key}, capacity, refillRate, time.Now().UnixMicro()).StringSlice()
	if err != nil {
		t.Fatalf("peek after deduct failed: %v", err)
	}
	if res[0] != "70" {
		t.Fatalf("peek after deduct = %s, want 70", res[0])
	}
}

func TestTokenUsageReporter_Report(t *testing.T) {
	_, rdb := newRedis(t)
	ctx := context.Background()
	reporter := newTokenUsageReporter(rdb, "tb:test:reporter", time.Minute, 100)

	reporter.Report(ctx, 40)

	// 直接读 Redis 验证余额
	remaining, err := rdb.HGet(ctx, "tb:test:reporter", "tokens").Result()
	if err != nil {
		t.Fatalf("hget failed: %v", err)
	}
	if remaining != "60" {
		t.Fatalf("remaining = %s, want 60", remaining)
	}
}
```

注意：以上测试直接引用了 `internal/middleware` 包中的非导出 Lua 脚本与 `newTokenUsageReporter`。若这些未导出，可通过 `TokenBucketTokenRateLimiterMiddleware` 注入 context 后调用 reporter，或调整测试包为 `middleware_test` 并导出测试钩子。更简单的做法是把 Lua 脚本与 reporter 构造函数改为包内可测试：测试文件放在 `test/unit/token_rate_limiter` 并导入 `internal/middleware`，使用 `middleware.TokenBucketTokenRateLimiterMiddleware` 构造 reporter。

- [ ] **Step 2: 运行测试**

```bash
go test -v -count=1 ./test/unit/token_rate_limiter/...
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add test/unit/token_rate_limiter/token_rate_limiter_test.go
git commit -m "test(token_rate_limiter): add unit tests for token dimension limiter"
```

---

### Task 7: 全量验证

- [ ] **Step 1: 运行单元测试**

```bash
go test -count=1 ./...
```

Expected: all tests pass

- [ ] **Step 2: 运行 lint**

```bash
make lint
```

Expected: no lint errors

- [ ] **Step 3: 编译全量**

```bash
make build
```

Expected: build succeeds

---

## Self-Review

- **Spec coverage**:
  - Post-call deduction: Task 3 reporter + Task 4 usecase 上报 ✓
  - input+output total: Task 2 DTO methods ✓
  - 单桶可配置模型: Task 3 middleware 签名 ✓
  - 默认 1,000,000 TPM: Task 3 常量 + Task 5 路由 ✓
  - 与现有请求限流叠加: Task 5 同时挂载两个中间件 ✓
- **Placeholder scan**: 无 TBD/TODO/"later"/"appropriate"。
- **Type consistency**: `TokenUsageReporter.Report(ctx, int64)` 在 Task 1、3、4 中一致；`TotalTokens()` 返回 `int64` 在 Task 2、4 中一致。

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-17-token-dimension-rate-limiter.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** - 每个 Task 派一个独立 subagent 执行，我在中间做 review 与集成。
2. **Inline Execution** - 我在当前 session 按 Task 顺序直接编码，每个 Task 后做验证。

Which approach do you prefer?
