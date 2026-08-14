package cron

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
)

type triggerHitSyncCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	triggerRepo trigger.TriggerRepository
	hitCache    *cache.TriggerHitCache
	locker      *lock.RedisLocker
	lockKey     string
}

func NewTriggerHitSyncCron(db *gorm.DB, triggerRepo trigger.TriggerRepository, hitCache *cache.TriggerHitCache, redisClient *redis.Client) Cron {
	return &triggerHitSyncCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleTriggerHitSync)),
		),
		db:          db,
		triggerRepo: triggerRepo,
		hitCache:    hitCache,
		locker:      lock.NewLocker(redisClient),
	}
}

func (c *triggerHitSyncCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleTriggerHitSync)
	_, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleTriggerHitSync, c.locker, c.lockKey, LockOptions{}, c.sync, constant.CronTriggerSourceScheduled))
	if err != nil {
		return err
	}
	c.cron.Start()
	return nil
}

// Trigger 手动触发一次触发词命中同步
//
//	@receiver c *triggerHitSyncCron
//	@return bool
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func (c *triggerHitSyncCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleTriggerHitSync, c.locker, c.lockKey, LockOptions{}, c.sync)
}

func (c *triggerHitSyncCron) Stop() {
	<-c.cron.Stop().Done()
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *triggerHitSyncCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *triggerHitSyncCron) StopGracefully() {
	c.cron.Stop()
}

func (c *triggerHitSyncCron) sync(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	hits, err := c.hitCache.PopAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHitSync] Failed to pop hit counts", zap.Error(err))
		return nil, err
	}
	if len(hits) == 0 {
		return &commonmodel.CronCallAuditMetadata{}, nil
	}
	err = c.triggerRepo.BatchIncrementHitCount(ctx, hits)
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHitSync] Failed to batch increment hit counts", zap.Error(err))
		return nil, err
	}
	logger.WithCtx(ctx).Info("[TriggerHitSync] Synced hit counts",
		zap.Int("count", len(hits)))
	return &commonmodel.CronCallAuditMetadata{
		SyncedHits: int64(len(hits)),
	}, nil
}
