package middleware

import (
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/redis"
	"go.uber.org/zap"
)

// RateLimiterMiddleware 限频中间件
//
//	@param serviceName string
//	@param key string
//	@param period time.Duration
//	@param limit int64
//	@return ctx huma.Context
//	@return next func(huma.Context)
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2025-11-02 04:17:07
func RateLimiterMiddleware(serviceName, key string, period time.Duration, limit int64) func(ctx huma.Context, next func(huma.Context)) {
	rate := limiter.Rate{
		Period: period,
		Limit:  limit,
	}

	redisClient := cache.GetRedisClient()
	store := lo.Must1(redis.NewStoreWithOptions(redisClient, limiter.StoreOptions{
		Prefix: serviceName,
	}))

	instance := limiter.New(store, rate)

	return func(ctx huma.Context, next func(huma.Context)) {
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
				lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrUnauthorized))
				return
			}
		}

		limiterKey := fmt.Sprintf("%s:%v", keyValue, value)
		ctx = huma.WithValue(ctx, constant.CtxKeyLimiter, limiterKey)

		result, err := instance.Get(ctx.Context(), limiterKey)
		if err != nil {
			logger.WithCtx(ctx.Context()).Error("[RateLimiterMiddleware] failed to get rate limit", zap.Error(err))
			rsp := &dto.CommonRsp{Error: constant.ErrInternalError}
			_ = lo.Must1(ctx.BodyWriter().Write(lo.Must1(sonic.Marshal(rsp))))
			return
		}

		if result.Reached {
			fields := []zap.Field{zap.String("serviceName", serviceName)}
			if key == "" {
				fields = append(fields, zap.String("key", keyValue), zap.String("value", value))
			} else {
				fields = append(fields, zap.String("key", key), zap.String("value", value))
			}

			logger.WithCtx(ctx.Context()).Error("[RateLimiterMiddleware] rate limit reached", fields...)
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrTooManyRequests))
			return
		}
		next(ctx)
	}
}
