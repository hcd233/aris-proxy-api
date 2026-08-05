package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
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

// RunWithLock 拿 Redis 分布式锁后执行 fn；执行期间 ticker 续期；fn 返回后停止续期，锁由 TTL 自然过期。
// 续期失败不中断 fn（业务任务均幂等）。
// 返回 true 表示 fn 被执行，false 表示未获取到锁（跳过）。
//
//	@author centonhuang
//	@update 2026-06-18 01:30:00
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
// source 为触发来源（scheduled/manual）。
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func wrapCronFunc(name string, locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error), source string) func() {
	return func() {
		ctx := context.WithValue(getBootstrapContext(), constant.CtxKeyTraceID, uuid.New().String())
		start := time.Now()
		var (
			metadata *commonmodel.CronCallAuditMetadata
			fnErr    error
		)
		defer func() {
			if r := recover(); r != nil {
				cronPanicHandler(ctx, name, r, source)
			}
		}()

		if source == constant.CronTriggerSourceScheduled && cronJobStore != nil {
			job, err := cronJobStore.Get(ctx, name)
			if err == nil && job != nil && !job.Enabled {
				logger.WithCtx(ctx).Info("[Cron] Cron job is disabled in DB, skip", zap.String("name", name))
				saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSkipped, 0, "", nil, source)
				return
			}
		}

		executed := RunWithLock(ctx, locker, key, opts, func(lockCtx context.Context) {
			metadata, fnErr = fn(lockCtx)
		})
		if !executed {
			return
		}
		durationMs := time.Since(start).Milliseconds()
		if fnErr != nil {
			saveCronCallAudit(ctx, name, constant.CronCallAuditStatusFailed, durationMs, fnErr.Error(), nil, source)
			return
		}
		saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSuccess, durationMs, "", metadata, source)
	}
}

// TriggerWithLock 手动触发：同步获取分布式锁，拿到锁后在后台 goroutine 执行 fn（含锁续期、
// panic 恢复与审计，source=manual），立即返回 true；拿不到锁或加锁失败返回 false，不产生任何记录。
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func TriggerWithLock(
	name string,
	locker lock.Locker,
	key string,
	opts LockOptions,
	fn func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error),
) bool {
	ctx := context.WithValue(getBootstrapContext(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = constant.CronLockDefaultTTL
	}
	renew := opts.RenewInterval
	if renew <= 0 {
		renew = ttl / constant.CronLockDefaultRenewDivisor
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	value := uuid.New().String()
	locked, err := locker.Lock(childCtx, key, value, ttl)
	if err != nil {
		log.Error("[CronTrigger] Lock acquire error", zap.String("key", key), zap.Error(err))
		return false
	}
	if !locked {
		log.Info("[CronTrigger] Lock held by another instance, skip manual trigger", zap.String("key", key))
		return false
	}

	go func() {
		start := time.Now()
		var (
			metadata *commonmodel.CronCallAuditMetadata
			fnErr    error
		)
		defer func() {
			if r := recover(); r != nil {
				cronPanicHandler(ctx, name, r, constant.CronTriggerSourceManual)
				return
			}
			durationMs := time.Since(start).Milliseconds()
			if fnErr != nil {
				saveCronCallAudit(ctx, name, constant.CronCallAuditStatusFailed, durationMs, fnErr.Error(), nil, constant.CronTriggerSourceManual)
				return
			}
			saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSuccess, durationMs, "", metadata, constant.CronTriggerSourceManual)
		}()
		go renewLoop(childCtx, locker, key, value, ttl, renew)
		metadata, fnErr = fn(childCtx)
	}()

	return true
}

func saveCronCallAudit(ctx context.Context, name, status string, durationMs int64, message string, metadata *commonmodel.CronCallAuditMetadata, source string) {
	if cronCallAuditStore == nil {
		return
	}
	now := time.Now().UTC()
	audit := &cronauditport.CronCallAuditView{
		CronName:      name,
		TraceID:       util.CtxValueString(ctx, constant.CtxKeyTraceID),
		StartedAt:     now.Add(-time.Duration(durationMs) * time.Millisecond),
		EndedAt:       now,
		DurationMs:    durationMs,
		Status:        status,
		TriggerSource: source,
		Message:       message,
		Metadata:      metadata,
	}
	if err := cronCallAuditStore.Save(ctx, audit); err != nil {
		logger.WithCtx(ctx).Error("[Cron] Save cron call audit failed",
			zap.String("name", name),
			zap.Error(err),
		)
	}
}

func cronPanicHandler(ctx context.Context, name string, r any, source string) {
	logger.WithCtx(ctx).Error("[Cron] Panic recovered",
		zap.String("name", name),
		zap.Any("panic", r),
		zap.Stack("stack"),
	)
	saveCronCallAudit(ctx, name, constant.CronCallAuditStatusPanic, 0, fmt.Sprintf(constant.CronPanicMessageTemplate, r), nil, source)
}
