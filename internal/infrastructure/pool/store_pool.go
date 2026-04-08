// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// submitMessageStoreTask 提交消息存储任务到 Store 池
//
//	@param pm *PoolManager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) submitMessageStoreTask(task *dto.MessageStoreTask) error {
	logger := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	return pm.storePool.Go(func() {
		toolSchemas := util.ToolSchemaMap{}
		for _, t := range task.Tools {
			if t.Parameters != nil {
				toolSchemas[t.Name] = t.Parameters
			}
		}

		messages := lo.Map(task.Messages, func(m *dto.UnifiedMessage, _ int) *dbmodel.Message {
			model := ""
			if lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role) {
				model = task.Model
			}
			return &dbmodel.Message{
				Model:    model,
				Message:  m,
				CheckSum: util.ComputeMessageChecksum(m, toolSchemas),
			}
		})

		tools := lo.Map(task.Tools, func(t *dto.UnifiedTool, _ int) *dbmodel.Tool {
			return &dbmodel.Tool{
				Tool:     t,
				CheckSum: util.ComputeToolChecksum(t),
			}
		})

		err := db.Transaction(func(tx *gorm.DB) error {
			messageIDs, err := pm.deduplicateAndStoreMessages(tx, messages)
			if err != nil {
				logger.Error("[StorePool] Failed to store messages", zap.Error(err))
				return err
			}

			toolIDs, err := pm.deduplicateAndStoreTools(tx, tools)
			if err != nil {
				logger.Error("[StorePool] Failed to store tools", zap.Error(err))
				return err
			}

			session := &dbmodel.Session{
				APIKeyName: task.APIKeyName,
				MessageIDs: messageIDs,
				ToolIDs:    toolIDs,
				Client:     task.Client,
				Metadata:   task.Metadata,
			}
			if err := dao.GetSessionDAO().Create(tx, session); err != nil {
				logger.Error("[StorePool] Failed to create session", zap.Error(err))
				return err
			}
			return nil
		})
		if err != nil {
			logger.Error("[StorePool] Transaction failed", zap.Error(err))
			return
		}
		logger.Info("[StorePool] Messages stored successfully")
	})
}

// submitAuditTask 提交审计任务到 Store 池
//
//	@param pm *PoolManager
//	@param task *dto.ModelCallAuditTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) submitAuditTask(task *dto.ModelCallAuditTask) error {
	l := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	return pm.storePool.Go(func() {
		audit := &dbmodel.ModelCallAudit{
			APIKeyID:                 task.APIKeyID,
			ModelID:                  task.ModelID,
			Model:                    task.Model,
			UpstreamProvider:         task.UpstreamProvider,
			APIProvider:              task.APIProvider,
			InputTokens:              task.InputTokens,
			OutputTokens:             task.OutputTokens,
			CacheCreationInputTokens: task.CacheCreationInputTokens,
			CacheReadInputTokens:     task.CacheReadInputTokens,
			FirstTokenLatencyMs:      task.FirstTokenLatencyMs,
			StreamDurationMs:         task.StreamDurationMs,
			UserAgent:                task.UserAgent,
			UpstreamStatusCode:      task.UpstreamStatusCode,
			ErrorMessage:            task.ErrorMessage,
			TraceID:                task.TraceID,
		}
		if err := dao.GetModelCallAuditDAO().Create(db, audit); err != nil {
			l.Error("[StorePool] Failed to store audit record", zap.Error(err))
			return
		}
		l.Info("[StorePool] Audit record stored successfully")
	})
}
