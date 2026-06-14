package cron

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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

func (c *blockedHitSyncCron) Start() error {
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleBlockedHitSync)
	_, err := c.cron.AddFunc(constant.CronSpecBlockedHitSync, wrapCronFunc(c.locker, key, LockOptions{}, c.sync))
	if err != nil {
		return err
	}
	c.cron.Start()
	return nil
}

func (c *blockedHitSyncCron) Stop() {
	<-c.cron.Stop().Done()
}

func (c *blockedHitSyncCron) sync(ctx context.Context) {
	hits, err := c.hitCache.PopAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to pop hit counts", zap.Error(err))
		return
	}
	if len(hits) == 0 {
		return
	}
	err = c.blockedRepo.BatchIncrementHitCount(ctx, hits)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to batch increment hit counts", zap.Error(err))
		return
	}
	logger.WithCtx(ctx).Info("[BlockedHitSync] Synced hit counts",
		zap.Int("count", len(hits)))
}
