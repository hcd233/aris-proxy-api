package middleware

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// tokenBucketLua 令牌桶限流Lua脚本
//
// 基于 Redis Hash 实现：
//   - field "tokens": 当前剩余令牌数
//   - field "last_refill": 上次补充令牌的时间戳（微秒）
//
// 每次请求时：
//  1. 读取桶状态（tokens + last_refill）
//  2. 按照 elapsed time 计算应补充的令牌数，上限为 capacity
//  3. 尝试消耗 1 个令牌
//  4. 返回 [剩余令牌数, 是否被拒绝(0/1), 桶容量]
//
// KEYS[1]: 限流键
// ARGV[1]: 桶容量（最大令牌数）
// ARGV[2]: 每微秒补充的令牌数（refillRate = capacity / period_microseconds）
// ARGV[3]: 当前时间戳（微秒）
// ARGV[4]: 窗口时长（毫秒），用于设置 key 过期
var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expire_ms = tonumber(ARGV[4])

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

if tokens < 1 then
    redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('PEXPIRE', key, expire_ms)
    return {tostring(tokens), "1", tostring(capacity)}
end

tokens = tokens - 1
redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('PEXPIRE', key, expire_ms)

return {tostring(tokens), "0", tostring(capacity)}
`)

// TokenBucketRateLimiterMiddleware 令牌桶限流中间件
//
// 基于 Redis Hash + Lua 脚本实现令牌桶算法。桶以固定速率补充令牌，每个请求消耗 1 个令牌。
// 当桶为空时拒绝请求。相比固定窗口和滑动窗口，令牌桶允许一定程度的突发流量（最多 capacity 个），
// 同时保证长期平均速率不超过 capacity/period。
//
// 响应头说明：
//
//   - X-RateLimit-Limit: 桶容量（最大令牌数）
//
//   - X-RateLimit-Remaining: 当前剩余令牌数（向下取整）
//
//   - Retry-After: 被限流时，恢复 1 个令牌所需的秒数（仅在被拒绝时返回）
//
//     @param serviceName string 服务名称，用作 Redis key 前缀
//     @param key string 限流维度的 context key，为空则按 IP 限流
//     @param period time.Duration 令牌完全补满所需时间
//     @param capacity int64 桶容量（最大令牌数），同时也是突发上限
//     @return func(ctx huma.Context, next func(huma.Context))
//     @author centonhuang
//     @update 2026-03-20 10:00:00
func TokenBucketRateLimiterMiddleware(serviceName, key string, period time.Duration, capacity int64) func(ctx huma.Context, next func(huma.Context)) {
	redisClient := cache.GetRedisClient()
	prefix := fmt.Sprintf("tb:%s", serviceName)

	// 每微秒补充的令牌数
	refillRate := float64(capacity) / float64(period.Microseconds())
	expireMs := period.Milliseconds() * 2

	// 恢复 1 个令牌所需的秒数（用于 Retry-After 头）
	retryAfterSeconds := int(math.Ceil(1.0 / (refillRate * 1e6)))

	return func(ctx huma.Context, next func(huma.Context)) {
		logger := logger.WithCtx(ctx.Context())
		var keyValue, value string
		if key == "" {
			keyValue = "ip"
			value = ctx.Header("X-Forwarded-For")
			if value == "" {
				value = ctx.Header("X-Real-IP")
			}
			if value == "" {
				value = "unknown"
			}
		} else {
			if ctxValue := ctx.Context().Value(key); ctxValue != nil {
				keyValue = key
				value = fmt.Sprintf("%v", ctxValue)
			} else {
				lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
				return
			}
		}

		limiterKey := fmt.Sprintf("%s:%s:%v", prefix, keyValue, value)
		ctx = huma.WithValue(ctx, constant.CtxKeyLimiter, limiterKey)

		now := time.Now().UnixMicro()

		result, err := tokenBucketLua.Run(
			ctx.Context(), redisClient,
			[]string{limiterKey},
			capacity, refillRate, now, expireMs,
		).StringSlice()
		if err != nil {
			logger.Error("[TokenBucketRateLimiter] Failed to execute rate limit script", zap.Error(err))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}

		remaining, _ := strconv.ParseFloat(result[0], 64)
		remainingInt := int64(math.Max(0, math.Floor(remaining)))
		limitStr := result[2]

		rejected := result[1] == "1"
		if rejected {
			logger.Warn("[TokenBucketRateLimiter] Rate limit reached",
				zap.String("serviceName", serviceName),
				zap.String("key", keyValue),
				zap.String("value", value),
				zap.Int64("remaining", remainingInt),
				zap.String("capacity", limitStr),
			)
			ctx.SetHeader("X-RateLimit-Limit", limitStr)
			ctx.SetHeader("X-RateLimit-Remaining", "0")
			ctx.SetHeader("Retry-After", strconv.Itoa(retryAfterSeconds))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrTooManyRequests.BizError()))
			return
		}

		ctx.SetHeader("X-RateLimit-Limit", limitStr)
		ctx.SetHeader("X-RateLimit-Remaining", strconv.FormatInt(remainingInt, 10))

		next(ctx)
	}
}
