package cron

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// LockOptions cron 锁的可选参数（0 → 走默认值）
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type LockOptions struct {
	TTL           time.Duration
	RenewInterval time.Duration
}

// RunWithLock 拿 Redis 分布式锁后执行 fn；执行期间 ticker 续期；返回前 defer 释放。
// 续期失败不中断 fn（业务任务均幂等）。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func RunWithLock(
	parentCtx context.Context,
	locker lock.Locker,
	key string,
	opts LockOptions,
	fn func(ctx context.Context),
) {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = constant.CronLockDefaultTTL
	}
	renew := opts.RenewInterval
	if renew <= 0 {
		renew = ttl / constant.CronLockDefaultRenewDivisor
	}

	childCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	log := logger.WithCtx(childCtx)

	value := uuid.New().String()
	locked, err := locker.Lock(childCtx, key, value, ttl)
	if err != nil {
		log.Error("[CronLock] Lock acquire error", zap.String("key", key), zap.Error(err))
		return
	}
	if !locked {
		log.Info("[CronLock] Lock held by another instance, skip this run", zap.String("key", key))
		return
	}
	defer func() {
		if err := locker.Unlock(childCtx, key, value); err != nil {
			log.Error("[CronLock] Unlock error", zap.String("key", key), zap.Error(err))
		}
	}()

	go renewLoop(childCtx, locker, key, value, ttl, renew)
	fn(childCtx)
}

func renewLoop(ctx context.Context, locker lock.Locker, key, value string, ttl, renew time.Duration) {
	t := time.NewTicker(renew)
	defer t.Stop()
	log := logger.WithCtx(ctx)
	failCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ok, err := locker.Refresh(ctx, key, value, ttl)
			switch {
			case err != nil:
				failCount++
				log.Warn("[CronLock] Refresh error",
					zap.String("key", key),
					zap.Int("consecutiveFailures", failCount),
					zap.Error(err))
				if failCount >= constant.CronLockMaxConsecutiveRenewFailures {
					log.Warn("[CronLock] Too many refresh failures, stop renewing",
						zap.String("key", key), zap.Int("failures", failCount))
					return
				}
			case !ok:
				log.Warn("[CronLock] Lock lost, stop renewing", zap.String("key", key))
				return
			default:
				failCount = 0
			}
		}
	}
}

// wrapCronFunc 把 cron fn 包成"注入 traceID + RunWithLock"的整体，供 AddFunc 使用。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func wrapCronFunc(locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context)) func() {
	return func() {
		ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
		RunWithLock(ctx, locker, key, opts, fn)
	}
}
