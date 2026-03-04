package middleware

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// RedisLockMiddleware Redis锁中间件
//
//	@param serviceName string
//	@param key string
//	@param expire time.Duration
//	@return fiber.Handler
//	@author centonhuang
//	@update 2025-11-11 04:52:25
func RedisLockMiddleware(serviceName, key string, expire time.Duration) func(ctx huma.Context, next func(huma.Context)) {
	locker := lock.NewLocker()

	return func(ctx huma.Context, next func(huma.Context)) {
		logger := logger.WithCtx(ctx.Context())

		value := ctx.Context().Value(key)
		if value == nil {
			logger.Error("[RedisLockMiddleware] value is nil", zap.String("key", key))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrInternalError))
			return
		}

		lockKey := fmt.Sprintf(constant.LockKeyTemplateMiddleware, serviceName, key, value)
		lockValue := uuid.New().String()

		success, err := locker.Lock(ctx.Context(), lockKey, lockValue, expire)
		if err != nil {
			logger.Error("[RedisLockMiddleware] lock resource error", zap.Error(err))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrInternalError))
			return
		}
		if !success {
			logger.Info("[RedisLockMiddleware] lock resource is already locked", zap.String("lockKey", lockKey))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrTooManyRequests))
			return
		}

		defer func() {
			err = locker.Unlock(ctx.Context(), lockKey, lockValue)
			if err != nil {
				logger.Error("[RedisLockMiddleware] unlock resource error", zap.String("lockKey", lockKey), zap.Error(err))
			}
		}()
		next(ctx)
	}
}
