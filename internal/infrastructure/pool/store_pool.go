// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SubmitMessageStoreTask 提交消息存储任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	return pm.storePool.Go(func() {
		pm.runMessageStoreTask(task)
	})
}

func (pm *PoolManager) runMessageStoreTask(task *dto.MessageStoreTask) {
	log := logger.WithCtx(task.Ctx)
	db := pm.db.WithContext(task.Ctx)

	toolSchemas := vo.ToolSchemaMap{}
	for _, t := range task.Tools {
		if t.Parameters != nil {
			toolSchemas[t.Name] = t.Parameters
		}
	}

	messages := lo.Map(task.Messages, func(m *vo.UnifiedMessage, _ int) *dbmodel.Message {
		model := ""
		if lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role) {
			model = task.Model
		}
		return &dbmodel.Message{
			Model:    model,
			Message:  m,
			CheckSum: vo.ComputeMessageChecksum(m, model, toolSchemas),
		}
	})

	tools := lo.Map(task.Tools, func(t *vo.UnifiedTool, _ int) *dbmodel.Tool {
		return &dbmodel.Tool{
			Tool:     t,
			CheckSum: vo.ComputeToolChecksum(t),
		}
	})

	err := db.Transaction(func(tx *gorm.DB) error {
		messageIDs, err := pm.deduplicateAndStoreMessages(tx, messages)
		if err != nil {
			log.Error("[StorePool] Failed to store messages", zap.Error(err))
			return err
		}

		questions := extractQuestionIDs(messages, messageIDs)
		models := extractAssistantModels(messages)

		if err := pm.upgradeReasoningContent(tx, messages, messageIDs); err != nil {
			return err
		}

		toolIDs, err := pm.deduplicateAndStoreTools(tx, tools)
		if err != nil {
			log.Error("[StorePool] Failed to store tools", zap.Error(err))
			return err
		}

		session := &dbmodel.Session{
			APIKeyName: task.APIKeyName,
			MessageIDs: messageIDs,
			Questions:  questions,
			Models:     models,
			ToolIDs:    toolIDs,
			Metadata:   task.Metadata,
		}
		if err := dao.GetSessionDAO().Create(tx, session); err != nil {
			log.Error("[StorePool] Failed to create session", zap.Error(err))
			return err
		}
		return nil
	})
	if err != nil {
		log.Error("[StorePool] Transaction failed", zap.Error(err))
		return
	}
	log.Info("[StorePool] Messages stored successfully")
}

// extractQuestionIDs 从消息中提取用户提问的 message ID 列表
func extractQuestionIDs(messages []*dbmodel.Message, messageIDs []uint) []uint {
	return lo.FilterMap(messages, func(m *dbmodel.Message, i int) (uint, bool) {
		if m.Message.Role == enum.RoleUser && m.Message.ToolCallID == "" {
			return messageIDs[i], true
		}
		return 0, false
	})
}

// extractAssistantModels 从 assistant 消息中抽取去重后的模型名列表
func extractAssistantModels(messages []*dbmodel.Message) []string {
	candidates := lo.FilterMap(messages, func(m *dbmodel.Message, _ int) (string, bool) {
		if m.Message.Role == enum.RoleAssistant && m.Model != "" {
			return m.Model, true
		}
		return "", false
	})
	return lo.Uniq(candidates)
}

// upgradeReasoningContent 补充存量消息的 reasoning_content
//
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param messages []*dbmodel.Message
//	@param messageIDs []uint 与 messages 顺序对齐的 ID 列表
//	@return error
//	@author centonhuang
//	@update 2026-06-13 10:00:00
func (pm *PoolManager) upgradeReasoningContent(tx *gorm.DB, messages []*dbmodel.Message, messageIDs []uint) error {
	msgByID := make(map[uint]*vo.UnifiedMessage)
	needsUpgradeIDs := lo.FilterMap(messages, func(m *dbmodel.Message, i int) (uint, bool) {
		if m.Message.ReasoningContent != "" {
			msgByID[messageIDs[i]] = m.Message
			return messageIDs[i], true
		}
		return 0, false
	})
	if len(needsUpgradeIDs) == 0 {
		return nil
	}
	var missing []*dbmodel.Message
	if err := tx.Model(&dbmodel.Message{}).
		Where(constant.WhereFieldID+" IN ? AND ("+constant.FieldMessage+"::jsonb->>'reasoning_content' IS NULL OR "+constant.FieldMessage+"::jsonb->>'reasoning_content' = '')", needsUpgradeIDs).
		Select(constant.FieldID).
		Find(&missing).Error; err != nil {
		return err
	}
	for _, mr := range missing {
		if msg, ok := msgByID[mr.ID]; ok {
			if err := tx.Model(&dbmodel.Message{ID: mr.ID}).
				Select(constant.FieldMessage, constant.FieldUpdatedAt).
				Updates(map[string]any{
					constant.FieldMessage:   msg,
					constant.FieldUpdatedAt: time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
		}
	}
	if len(missing) > 0 {
		logger.Logger().Info("[StorePool] Upgraded reasoning_content for existing messages",
			zap.Int("count", len(missing)))
	}
	return nil
}

// SubmitModelCallAuditTask 提交模型调用审计任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.ModelCallAuditTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	l := logger.WithCtx(task.Ctx)
	db := pm.db.WithContext(task.Ctx)

	return pm.storePool.Go(func() {
		audit := &dbmodel.ModelCallAudit{
			APIKeyID:                 util.CtxValueUint(task.Ctx, constant.CtxKeyAPIKeyID),
			ModelID:                  task.ModelID,
			Model:                    task.Model,
			UpstreamProtocol:         task.UpstreamProtocol,
			APIProtocol:              task.APIProtocol,
			Endpoint:                 task.Endpoint,
			InputTokens:              task.InputTokens,
			OutputTokens:             task.OutputTokens,
			CacheCreationInputTokens: task.CacheCreationInputTokens,
			CacheReadInputTokens:     task.CacheReadInputTokens,
			FirstTokenLatencyMs:      task.FirstTokenLatencyMs,
			StreamDurationMs:         task.StreamDurationMs,
			UserAgent:                util.CtxValueString(task.Ctx, constant.CtxKeyClient),
			UpstreamStatusCode:       task.UpstreamStatusCode,
			ErrorMessage:             task.ErrorMessage,
			TraceID:                  util.CtxValueString(task.Ctx, constant.CtxKeyTraceID),
		}
		if err := dao.GetModelCallAuditDAO().Create(db, audit); err != nil {
			l.Error("[StorePool] Failed to store audit record", zap.Error(err))
			return
		}
		l.Info("[StorePool] Audit record stored successfully")
	})
}
