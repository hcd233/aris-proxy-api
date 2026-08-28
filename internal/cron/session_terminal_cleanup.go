// Package cron Session终态清理定时任务
//
// 扫描最近 24 小时内末条消息为 assistant+tool_calls（对话中断于工具调用处）的
// 会话并软删。前缀去重已迁移至 session 插入路径实时执行
// （internal/infrastructure/repository/session_dedup.go），本任务不再做前缀解析。
//
// 稳态语义与旧算法等价：旧实现中 absorbed 保护仅使 merge target 多存活一个
// cron 周期（下一轮成单例组后仍被删除）。
//
//	author centonhuang
//	update 2026-08-29 10:00:00
package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionTerminalCleanupCron Session终态清理定时任务，删除最近 24h 内中断于 assistant tool_call 的会话
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
type SessionTerminalCleanupCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     *lock.RedisLocker
	lockKey    string
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionTerminalCleanupCron 创建Session终态清理定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func NewSessionTerminalCleanupCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SessionTerminalCleanupCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionTerminalCleanup)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}

// Stop 停止Session终态清理定时任务
//
//	@receiver c *SessionTerminalCleanupCron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *SessionTerminalCleanupCron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

// Start 启动Session终态清理定时任务
//
//	@receiver c *SessionTerminalCleanupCron
//	@param spec string cron 表达式
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionTerminalCleanup)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleSessionTerminalCleanup, c.locker, c.lockKey, LockOptions{}, c.cleanup, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[SessionTerminalCleanupCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionTerminalCleanupCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// Trigger 手动触发一次 Session 终态清理
//
//	@receiver c *SessionTerminalCleanupCron
//	@return bool
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleSessionTerminalCleanup, c.locker, c.lockKey, LockOptions{}, c.cleanup)
}

// cleanup 执行Session终态清理
//
//	@receiver c *SessionTerminalCleanupCron
//	@param ctx context.Context
//	@return *commonmodel.CronCallAuditMetadata
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) cleanup(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.FindCreatedSince(db, time.Now().UTC().Add(-constant.CronTerminalCleanupWindow))
	if err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to load recent sessions", zap.Error(err))
		return nil, err
	}
	checkedCount := int64(len(sessions))

	lastMsgIDs := lo.FilterMap(sessions, func(s dao.SessionTerminalScanView, _ int) (uint, bool) {
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		return s.MessageIDs[len(s.MessageIDs)-1], true
	})
	terminalMsgIDs, err := c.messageDAO.FilterTerminalToolCallIDs(db, lo.Uniq(lastMsgIDs))
	if err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to filter terminal tool call message ids", zap.Error(err))
		return nil, err
	}

	victimIDs := PickTerminalStuckSessions(sessions, terminalMsgIDs)
	if len(victimIDs) == 0 {
		log.Info("[SessionTerminalCleanupCron] No terminal stuck sessions", zap.Int64("checked", checkedCount))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	if err := c.sessionDAO.BatchDeleteByField(db, constant.WhereFieldID, victimIDs); err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to delete terminal stuck sessions", zap.Error(err))
		return nil, err
	}

	log.Info("[SessionTerminalCleanupCron] Terminal cleanup completed",
		zap.Int64("checked", checkedCount),
		zap.Int("deleted", len(victimIDs)))

	return &commonmodel.CronCallAuditMetadata{
		CheckedSessions: checkedCount,
		DedupedSessions: int64(len(victimIDs)),
	}, nil
}

// PickTerminalStuckSessions 取末条消息命中 terminalMsgIDs 的会话 ID
//
//	导出以便外部测试包验证。
//
//	@param sessions []dao.SessionTerminalScanView
//	@param terminalMsgIDs []uint 已由 SQL 判定为 assistant+tool_calls 的 message ID
//	@return []uint
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func PickTerminalStuckSessions(sessions []dao.SessionTerminalScanView, terminalMsgIDs []uint) []uint {
	terminalSet := lo.SliceToMap(terminalMsgIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	return lo.FilterMap(sessions, func(s dao.SessionTerminalScanView, _ int) (uint, bool) {
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		if _, ok := terminalSet[s.MessageIDs[len(s.MessageIDs)-1]]; !ok {
			return 0, false
		}
		return s.ID, true
	})
}
