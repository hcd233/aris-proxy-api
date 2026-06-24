package cron

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
)

type blockedHitSyncCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	blockedRepo blocked.BlockedRepository
	hitCache    *cache.BlockedHitCache
	locker      lock.Locker
}

func NewBlockedHitSyncCron(db *gorm.DB, blockedRepo blocked.BlockedRepository, hitCache *cache.BlockedHitCache, redisClient *redis.Client) Cron {
	return &blockedHitSyncCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleBlockedHitSync)),
		),
		db:          db,
		blockedRepo: blockedRepo,
		hitCache:    hitCache,
		locker:      lock.NewLocker(redisClient),
	}
}

func (c *blockedHitSyncCron) Start(spec string) error {
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleBlockedHitSync)
	_, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleBlockedHitSync, c.locker, key, LockOptions{}, c.sync))
	if err != nil {
		return err
	}
	c.cron.Start()
	return nil
}

func (c *blockedHitSyncCron) Stop() {
	<-c.cron.Stop().Done()
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *blockedHitSyncCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *blockedHitSyncCron) StopGracefully() {
	c.cron.Stop()
}

func (c *blockedHitSyncCron) sync(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	hits, err := c.hitCache.PopAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to pop hit counts", zap.Error(err))
		return nil, err
	}
	if len(hits) == 0 {
		return &commonmodel.CronCallAuditMetadata{}, nil
	}
	err = c.blockedRepo.BatchIncrementHitCount(ctx, hits)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to batch increment hit counts", zap.Error(err))
		return nil, err
	}
	logger.WithCtx(ctx).Info("[BlockedHitSync] Synced hit counts",
		zap.Int("count", len(hits)))
	return &commonmodel.CronCallAuditMetadata{
		SyncedHits: int64(len(hits)),
	}, nil
}
