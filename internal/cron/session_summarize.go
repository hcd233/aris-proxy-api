// Package cron Session总结定时任务
//
//	author centonhuang
//	update 2026-03-26 10:00:00
package cron

import (
	"context"
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionSummarizeCron Session总结定时任务
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionSummarizeCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	poolManager *pool.PoolManager
	locker      lock.Locker
	sessionDAO  *dao.SessionDAO
	messageDAO  *dao.MessageDAO
}

// NewSessionSummarizeCron 创建Session总结定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionSummarizeCron(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) Cron {
	return &SessionSummarizeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionSummarize)),
		),
		db:          db,
		poolManager: poolManager,
		locker:      lock.NewLocker(cache),
		sessionDAO:  dao.GetSessionDAO(),
		messageDAO:  dao.GetMessageDAO(),
	}
}

// Stop 停止Session总结定时任务
//
//	@receiver c *SessionSummarizeCron
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// Start 启动Session总结定时任务
//
//	@receiver c *SessionSummarizeCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionSummarizeCron) Start() error {
	// 每天凌晨2:00执行，在去重任务完成后执行
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionSummarize)
	entryID, err := c.cron.AddFunc(constant.CronSpecSessionSummarize, wrapCronFunc(c.locker, key, LockOptions{}, c.summarize))
	if err != nil {
		logger.Logger().Error("[SessionSummarizeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionSummarizeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// summarize 执行Session总结逻辑
//
//	@receiver c *SessionSummarizeCron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionSummarizeCron) summarize(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)
	poolManager := c.poolManager

	sessions, err := c.sessionDAO.BatchGetByField(db, constant.WhereFieldSummary, []string{""}, constant.SessionRepoFieldsSummarize)
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to get unsummarized sessions", zap.Error(err))
		return
	}

	if len(sessions) == 0 {
		log.Info("[SessionSummarizeCron] No sessions to summarize")
		return
	}

	log.Info("[SessionSummarizeCron] Starting summarization", zap.Int("count", len(sessions)))

	for _, session := range sessions {
		content, err := c.getSessionContent(db, session)
		if err != nil {
			log.Error("[SessionSummarizeCron] Failed to get session content",
				zap.Uint("sessionID", session.ID),
				zap.Error(err))
			continue
		}

		task := &dto.SummarizeTask{
			Ctx:       ctx,
			SessionID: session.ID,
			Content:   content,
		}

		if err := poolManager.SubmitSummarizeTask(task); err != nil {
			log.Error("[SessionSummarizeCron] Failed to submit summarize task",
				zap.Uint("sessionID", session.ID),
				zap.Error(err))
		}
	}

	log.Info("[SessionSummarizeCron] All summarization tasks submitted")
}

// getSessionContent 获取Session的消息内容
//
//	@receiver c *SessionSummarizeCron
//	@param ctx context.Context
//	@param session *dbmodel.Session
//	@return string 消息内容
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) getSessionContent(db *gorm.DB, session *dbmodel.Session) (string, error) {
	if len(session.MessageIDs) == 0 {
		return "", nil
	}

	messages, err := c.messageDAO.BatchGetByField(db, constant.WhereFieldID, session.MessageIDs, constant.MessageRepoFieldsContent)
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

	return strings.Join(contentParts, constant.NewlineString), nil
}

// formatMessage 将UnifiedMessage格式化为字符串，包含所有字段
//
//	@param msg *vo.UnifiedMessage
//	@return string
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func formatMessage(msg *vo.UnifiedMessage) string {
	if msg == nil {
		return ""
	}

	var parts []string

	// Role
	parts = append(parts, fmt.Sprintf(constant.MessageFormatRole, msg.Role))

	// Name
	if msg.Name != "" {
		parts = append(parts, fmt.Sprintf(constant.MessageFormatName, msg.Name))
	}

	// Content
	if msg.Content != nil {
		if msg.Content.Text != "" {
			parts = append(parts, fmt.Sprintf(constant.MessageFormatContent, msg.Content.Text))
		}
		for _, p := range msg.Content.Parts {
			switch p.Type {
			case enum.ContentPartTypeText:
				if p.Text != "" {
					parts = append(parts, fmt.Sprintf(constant.MessageFormatContentText, p.Text))
				}
			case enum.ContentPartTypeImageURL:
				parts = append(parts, fmt.Sprintf(constant.MessageFormatContentImage, p.ImageURL))
			case enum.ContentPartTypeInputAudio:
				parts = append(parts, fmt.Sprintf(constant.MessageFormatContentAudio, p.AudioFormat))
			case enum.ContentPartTypeFile:
				parts = append(parts, fmt.Sprintf(constant.MessageFormatContentFile, p.Filename))
			case enum.ContentPartTypeRefusal:
				parts = append(parts, fmt.Sprintf(constant.MessageFormatContentRefusal, p.Text))
			}
		}
	}

	// ReasoningContent
	if msg.ReasoningContent != "" {
		parts = append(parts, fmt.Sprintf(constant.MessageFormatReasoning, msg.ReasoningContent))
	}

	// ToolCalls
	for _, tc := range msg.ToolCalls {
		parts = append(parts, fmt.Sprintf(constant.MessageFormatToolCall, tc.Name, tc.Arguments))
	}

	// ToolCallID
	if msg.ToolCallID != "" {
		parts = append(parts, fmt.Sprintf(constant.MessageFormatToolCallID, msg.ToolCallID))
	}

	// Refusal
	if msg.Refusal != "" {
		parts = append(parts, fmt.Sprintf(constant.MessageFormatRefusal, msg.Refusal))
	}

	return strings.Join(parts, constant.MessageContentSeparator)
}
