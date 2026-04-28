// Package cron Session总结定时任务
//
//	author centonhuang
//	update 2026-03-26 10:00:00
package cron

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// SessionSummarizeCron Session总结定时任务
//
//	@author centonhuang
//	@update 2026-03-26 10:00:00
type SessionSummarizeCron struct {
	cron       *cron.Cron
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionSummarizeCron 创建Session总结定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func NewSessionSummarizeCron() Cron {
	return &SessionSummarizeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionSummarize, logger.Logger())),
		),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
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
//	@update 2026-04-03 10:00:00
func (c *SessionSummarizeCron) Start() error {
	// 每天凌晨2:00执行，在去重任务完成后执行
	entryID, err := c.cron.AddFunc(constant.CronSpecSessionSummarize, c.summarize)
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
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) summarize() {
	ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)
	poolManager := pool.GetPoolManager()

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
		content, err := c.getSessionContent(ctx, session)
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
func (c *SessionSummarizeCron) getSessionContent(ctx context.Context, session *dbmodel.Session) (string, error) {
	if len(session.MessageIDs) == 0 {
		return "", nil
	}

	messages, err := c.messageDAO.BatchGetByField(database.GetDBInstance(ctx), constant.WhereFieldID, session.MessageIDs, constant.MessageRepoFieldsContent)
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
//	@param msg *dto.UnifiedMessage
//	@return string
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func formatMessage(msg *dto.UnifiedMessage) string {
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
