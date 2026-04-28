// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
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

// SubmitMessageStoreTask 提交消息存储任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	log := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	return pm.storePool.Go(func() {
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
				CheckSum: vo.ComputeMessageChecksum(m, toolSchemas),
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

			toolIDs, err := pm.deduplicateAndStoreTools(tx, tools)
			if err != nil {
				log.Error("[StorePool] Failed to store tools", zap.Error(err))
				return err
			}

			session := &dbmodel.Session{
				APIKeyName: task.APIKeyName,
				MessageIDs: messageIDs,
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
	})
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
	db := database.GetDBInstance(task.Ctx)

	return pm.storePool.Go(func() {
		audit := &dbmodel.ModelCallAudit{
			APIKeyID:                 util.CtxValueUint(task.Ctx, constant.CtxKeyAPIKeyID),
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
