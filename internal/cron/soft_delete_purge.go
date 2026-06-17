// Package cron 软删除数据清理定时任务
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
package cron

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SoftDeletePurgeCron 软删除数据清理定时任务，每周硬删除所有已软删除的Message、Session、Tool记录
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SoftDeletePurgeCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     lock.Locker
	messageDAO *dao.MessageDAO
	sessionDAO *dao.SessionDAO
	toolDAO    *dao.ToolDAO
}

// NewSoftDeletePurgeCron 创建软删除数据清理定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSoftDeletePurgeCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SoftDeletePurgeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSoftDeletePurge)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		messageDAO: dao.GetMessageDAO(),
		sessionDAO: dao.GetSessionDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}

// Stop 停止软删除数据清理定时任务
//
//	@receiver c *SoftDeletePurgeCron
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func (c *SoftDeletePurgeCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// Start 启动软删除数据清理定时任务
//
//	@receiver c *SoftDeletePurgeCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SoftDeletePurgeCron) Start() error {
	// 每周日凌晨4:00执行，确保所有任务完成后再清理
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSoftDeletePurge)
	entryID, err := c.cron.AddFunc(constant.CronSpecSoftDeletePurge, wrapCronFunc(constant.CronModuleSoftDeletePurge, c.locker, key, LockOptions{}, c.purge))
	if err != nil {
		logger.Logger().Error("[SoftDeletePurgeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SoftDeletePurgeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// purge 执行硬删除逻辑，只删除未被任何活跃 Session 引用的 Message 和 Tool
//
//	@receiver c *SoftDeletePurgeCron
//	@author centonhuang
//	@update 2026-06-09 10:00:00
func (c *SoftDeletePurgeCron) purge(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	// 1. 查询所有被软删除的 session
	softDeletedSessions, err := c.sessionDAO.FindAllForPurge(db, true)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to find soft deleted sessions", zap.Error(err))
		return
	}

	if len(softDeletedSessions) == 0 {
		log.Info("[SoftDeletePurgeCron] No soft deleted sessions found")
		return
	}

	// 2. 从被软删除的 session 中提取 message_ids 和 tool_ids 并去重
	candidateMessageIDs := lo.Uniq(lo.Flatten(lo.Map(softDeletedSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.MessageIDs
	})))
	candidateToolIDs := lo.Uniq(lo.Flatten(lo.Map(softDeletedSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.ToolIDs
	})))

	// 3. 查询所有未删除的 session，收集引用的 message_ids 和 tool_ids
	activeSessions, err := c.sessionDAO.FindAllForPurge(db, false)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to find active sessions", zap.Error(err))
		return
	}

	usedMessageIDs := lo.Uniq(lo.Flatten(lo.Map(activeSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.MessageIDs
	})))
	usedToolIDs := lo.Uniq(lo.Flatten(lo.Map(activeSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.ToolIDs
	})))

	// 4. 计算差集：未被引用的 = 候选 - 已使用
	orphanMessageIDs, _ := lo.Difference(candidateMessageIDs, usedMessageIDs)
	orphanToolIDs, _ := lo.Difference(candidateToolIDs, usedToolIDs)

	// 5. 批量硬删除未被引用的 message 和 tool
	var msgCount, toolCount int64
	if len(orphanMessageIDs) > 0 {
		msgCount, err = c.messageDAO.HardDeleteByIDs(db, orphanMessageIDs)
		if err != nil {
			log.Error("[SoftDeletePurgeCron] Failed to purge messages", zap.Error(err))
			return
		}
	}

	if len(orphanToolIDs) > 0 {
		toolCount, err = c.toolDAO.HardDeleteByIDs(db, orphanToolIDs)
		if err != nil {
			log.Error("[SoftDeletePurgeCron] Failed to purge tools", zap.Error(err))
			return
		}
	}

	// 6. 硬删除被软删除的 session
	sessionCount, err := c.sessionDAO.HardDeleteSoftDeleted(db)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to purge sessions", zap.Error(err))
		return
	}

	log.Info("[SoftDeletePurgeCron] Purge completed",
		zap.Int64("sessionsDeleted", sessionCount),
		zap.Int64("messagesDeleted", msgCount),
		zap.Int64("toolsDeleted", toolCount))
}
