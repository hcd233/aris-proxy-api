package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
)

var (
	bootstrapCtx   context.Context
	bootstrapCtxMu sync.RWMutex
)

// SetBootstrapContext 设置 cron 任务的父 context（通常是 shutdown context）。
//
// InitCronJobs 会自动注入；测试代码可以手动调用以注入自定义 context。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func SetBootstrapContext(ctx context.Context) {
	bootstrapCtxMu.Lock()
	bootstrapCtx = ctx
	bootstrapCtxMu.Unlock()
}

func getBootstrapContext() context.Context {
	bootstrapCtxMu.RLock()
	defer bootstrapCtxMu.RUnlock()
	if bootstrapCtx == nil {
		return context.Background()
	}
	return bootstrapCtx
}

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
// 返回 true 表示 fn 被执行，false 表示未获取到锁（跳过）。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func RunWithLock(
	parentCtx context.Context,
	locker lock.Locker,
	key string,
	opts LockOptions,
	fn func(ctx context.Context),
) (executed bool) {
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
		return false
	}
	if !locked {
		log.Info("[CronLock] Lock held by another instance, skip this run", zap.String("key", key))
		return false
	}
	defer func() {
		if err := locker.Unlock(childCtx, key, value); err != nil {
			log.Error("[CronLock] Unlock error", zap.String("key", key), zap.Error(err))
		}
	}()

	go renewLoop(childCtx, locker, key, value, ttl, renew)
	fn(childCtx)
	return true
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

// wrapCronFunc 把 cron fn 包成"注入 traceID + 启用检查 + panic 恢复 + RunWithLock + 审计"的整体，供 AddFunc 使用。
//
// parentCtx 取自 SetBootstrapContext；未设置时退化为 context.Background()。
//
//	@author centonhuang
//	@update 2026-06-18 10:00:00
func wrapCronFunc(name string, locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context) map[string]any) func() {
	return func() {
		ctx := context.WithValue(getBootstrapContext(), constant.CtxKeyTraceID, uuid.New().String())
		start := time.Now()
		var metadata map[string]any
		defer func() {
			if r := recover(); r != nil {
				cronPanicHandler(ctx, name, r)
			}
		}()

		if cronJobStore != nil {
			job, err := cronJobStore.Get(ctx, name)
			if err == nil && job != nil && !job.Enabled {
				logger.WithCtx(ctx).Info("[Cron] Cron job is disabled in DB, skip", zap.String("name", name))
				saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSkipped, 0, "", nil)
				return
			}
		}

		if !RunWithLock(ctx, locker, key, opts, func(lockCtx context.Context) {
			metadata = fn(lockCtx)
		}) {
			return
		}
		durationMs := time.Since(start).Milliseconds()
		saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSuccess, durationMs, "", metadata)
	}
}

func saveCronCallAudit(ctx context.Context, name, status string, durationMs int64, message string, metadata map[string]any) {
	if cronCallAuditStore == nil {
		return
	}
	now := time.Now().UTC()
	audit := &cronauditport.CronCallAuditView{
		CronName:   name,
		TraceID:    util.CtxValueString(ctx, constant.CtxKeyTraceID),
		StartedAt:  now.Add(-time.Duration(durationMs) * time.Millisecond),
		EndedAt:    now,
		DurationMs: durationMs,
		Status:     status,
		Message:    message,
		Metadata:   metadata,
	}
	if err := cronCallAuditStore.Save(ctx, audit); err != nil {
		logger.WithCtx(ctx).Error("[Cron] Save cron call audit failed",
			zap.String("name", name),
			zap.Error(err),
		)
	}
}

func cronPanicHandler(ctx context.Context, name string, r any) {
	logger.WithCtx(ctx).Error("[Cron] Panic recovered",
		zap.String("name", name),
		zap.Any("panic", r),
		zap.Stack("stack"),
	)
	saveCronCallAudit(ctx, name, constant.CronCallAuditStatusPanic, 0, fmt.Sprintf(constant.CronPanicMessageTemplate, r), nil)
}
