// Package cron Session评分定时任务
//
//	author centonhuang
//	update 2026-04-02 10:00:00
package cron

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// SessionScoreCron Session评分定时任务
//
//	@author centonhuang
//	@update 2026-04-02 10:00:00
type SessionScoreCron struct {
	cron       *cron.Cron
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionScoreCron 创建Session评分定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func NewSessionScoreCron() Cron {
	return &SessionScoreCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter("SessionScoreCron", logger.Logger())),
		),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}

// Stop 停止Session评分定时任务
//
//	@receiver c *SessionScoreCron
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (c *SessionScoreCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// Start 启动Session评分定时任务
//
//	@receiver c *SessionScoreCron
//	@return error
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (c *SessionScoreCron) Start() error {
	// 每周一凌晨2:00运行
	entryID, err := c.cron.AddFunc("0 2 * * 1", c.score)
	if err != nil {
		logger.Logger().Error("[SessionScoreCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionScoreCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// score 执行Session评分逻辑
//
//	@receiver c *SessionScoreCron
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (c *SessionScoreCron) score() {
	ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)
	poolManager := pool.GetPoolManager()

	// 获取未评分且未删除的session（score_version为空字符串）
	sessions, err := c.sessionDAO.BatchGetByField(db, "score_version", []string{""}, []string{"id", "message_ids"})
	if err != nil {
		log.Error("[SessionScoreCron] Failed to get unscored sessions", zap.Error(err))
		return
	}

	if len(sessions) == 0 {
		log.Info("[SessionScoreCron] No sessions to score")
		return
	}

	log.Info("[SessionScoreCron] Starting scoring", zap.Int("count", len(sessions)))

	for _, session := range sessions {
		content, err := c.getSessionContent(ctx, session)
		if err != nil {
			log.Error("[SessionScoreCron] Failed to get session content",
				zap.Uint("sessionID", session.ID),
				zap.Error(err))
			continue
		}

		task := &dto.ScoreTask{
			Ctx:       ctx,
			SessionID: session.ID,
			Content:   content,
		}

		if err := poolManager.SubmitScoreTask(task); err != nil {
			log.Error("[SessionScoreCron] Failed to submit score task",
				zap.Uint("sessionID", session.ID),
				zap.Error(err))
		}
	}

	log.Info("[SessionScoreCron] All score tasks submitted")
}

// getSessionContent 获取Session的消息内容
//
//	@receiver c *SessionScoreCron
//	@param ctx context.Context
//	@param session *dbmodel.Session
//	@return string 消息内容
//	@return error
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (c *SessionScoreCron) getSessionContent(ctx context.Context, session *dbmodel.Session) (string, error) {
	if len(session.MessageIDs) == 0 {
		return "", nil
	}

	messages, err := c.messageDAO.BatchGetByField(database.GetDBInstance(ctx), "id", session.MessageIDs, []string{"id", "message"})
	if err != nil {
		return "", err
	}

	messageMap := lo.SliceToMap(messages, func(m *dbmodel.Message) (uint, *dbmodel.Message) {
		return m.ID, m
	})

	var contentParts []string
	for _, msgID := range session.MessageIDs {
		if msg, ok := messageMap[msgID]; ok && msg.Message != nil {
			formatted := formatMessage(msg.Message)
			if formatted != "" {
				contentParts = append(contentParts, formatted)
			}
		}
	}

	return strings.Join(contentParts, "\n"), nil
}
