// Package cron 软删除数据清理定时任务
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
package cron

import (
	"context"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// SoftDeletePurgeCron 软删除数据清理定时任务，每周硬删除所有已软删除的Message、Session、Tool记录
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
type SoftDeletePurgeCron struct {
	cron       *cron.Cron
	messageDAO *dao.MessageDAO
	sessionDAO *dao.SessionDAO
	toolDAO    *dao.ToolDAO
}

// NewSoftDeletePurgeCron 创建软删除数据清理定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func NewSoftDeletePurgeCron() Cron {
	return &SoftDeletePurgeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter("SoftDeletePurgeCron", logger.Logger())),
		),
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
//	@update 2026-04-03 10:00:00
func (c *SoftDeletePurgeCron) Start() error {
	// 每周日凌晨4:00执行，确保所有任务完成后再清理
	entryID, err := c.cron.AddFunc("0 4 * * 0", c.purge)
	if err != nil {
		logger.Logger().Error("[SoftDeletePurgeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SoftDeletePurgeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// purge 执行硬删除逻辑，依次清理Message、Session、Tool中所有已软删除的记录
//
//	@receiver c *SoftDeletePurgeCron
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func (c *SoftDeletePurgeCron) purge() {
	ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	msgCount, err := c.messageDAO.HardDeleteSoftDeleted(db)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to purge messages", zap.Error(err))
		return
	}

	sessionCount, err := c.sessionDAO.HardDeleteSoftDeleted(db)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to purge sessions", zap.Error(err))
		return
	}

	toolCount, err := c.toolDAO.HardDeleteSoftDeleted(db)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to purge tools", zap.Error(err))
		return
	}

	log.Info("[SoftDeletePurgeCron] Purge completed",
		zap.Int64("messagesDeleted", msgCount),
		zap.Int64("sessionsDeleted", sessionCount),
		zap.Int64("toolsDeleted", toolCount))
}
