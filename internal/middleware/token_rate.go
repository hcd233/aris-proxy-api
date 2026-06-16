package middleware

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
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
	retryAfterSeconds := int(math.Ceil(1.0 / (refillRate * 1e6)))

	return func(ctx huma.Context, next func(huma.Context)) {
		logger := logger.WithCtx(ctx.Context())
		if cache == nil {
			logger.Error("[TokenBucketTokenRateLimiter] Redis dependency is nil")
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
				logger.Warn("[TokenBucketTokenRateLimiter] Context value not found",
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
			logger.Error("[TokenBucketTokenRateLimiter] Failed to peek rate limit bucket", zap.Error(err))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}

		remaining, _ := strconv.ParseFloat(result[0], constant.ParseFloat64BitSize) //nolint:errcheck // redis returns float string
		remainingInt := int64(math.Max(0, math.Floor(remaining)))
		limitStr := result[1]

		if remainingInt <= 0 {
			logger.Warn("[TokenBucketTokenRateLimiter] Rate limit reached",
				zap.String("serviceName", serviceName),
				zap.String("key", keyValue),
				zap.String("value", value),
				zap.Int64("remaining", remainingInt),
				zap.String("capacity", limitStr),
			)
			ctx.SetHeader(constant.HTTPHeaderXRateLimitLimit, limitStr)
			ctx.SetHeader(constant.HTTPHeaderXRateLimitRemaining, constant.ZeroString)
			ctx.SetHeader(constant.HTTPHeaderRetryAfter, strconv.Itoa(retryAfterSeconds))
			ctx.SetStatus(fiber.StatusTooManyRequests)
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
