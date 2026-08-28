// Package cron Session前缀去重定时任务
//
// 去重算法与写回已提取至 internal/infrastructure/repository/session_dedup.go；
// terminal tool_call 清理由 session_terminal_cleanup.go（SessionTerminalCleanupCron）承担。
//
//	author centonhuang
//	update 2026-03-19 10:00:00
package cron

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionDeduplicateCron Session去重定时任务，清理MessageIDs被其他Session包含的冗余Session
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionDeduplicateCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     *lock.RedisLocker
	lockKey    string
	sessionDAO *dao.SessionDAO
}

// NewSessionDeduplicateCron 创建Session去重定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionDeduplicateCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SessionDeduplicateCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionDeduplicate)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		sessionDAO: dao.GetSessionDAO(),
	}
}

// Stop 停止Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

// Start 启动Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@param spec string cron 表达式
//	@return error
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionDeduplicate)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[SessionDeduplicateCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionDeduplicateCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// Trigger 手动触发一次 Session 去重
//
//	@receiver c *SessionDeduplicateCron
//	@return bool
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func (c *SessionDeduplicateCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate)
}

// deduplicate 执行Session去重逻辑
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-24 10:00:00
func (c *SessionDeduplicateCron) deduplicate(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, constant.SessionRepoFieldsDedup)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load sessions", zap.Error(err))
		return nil, err
	}

	checkedCount := int64(len(sessions))

	if len(sessions) < 2 {
		log.Info("[SessionDeduplicateCron] Skip deduplication, not enough sessions", zap.Int("count", len(sessions)))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	// 终端 tool_call 判定与 terminal 规则已随算法迁移拆分，此处仅做前缀去重。
	mergeResult := repository.FindRedundantSessions(sessions)

	if len(mergeResult.RedundantIDs) == 0 {
		log.Info("[SessionDeduplicateCron] No redundant sessions found", zap.Int("total", len(sessions)))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	mergedCount, err := repository.ApplyMergeResult(db, c.sessionDAO, mergeResult)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to apply deduplication", zap.Error(err))
		return nil, err
	}

	log.Info("[SessionDeduplicateCron] Deduplication completed",
		zap.Int("total", len(sessions)),
		zap.Int("deleted", len(mergeResult.RedundantIDs)),
		zap.Int("merged", mergedCount))

	return &commonmodel.CronCallAuditMetadata{
		CheckedSessions: checkedCount,
		DedupedSessions: int64(len(mergeResult.RedundantIDs)),
	}, nil
}
