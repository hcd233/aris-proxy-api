// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-02-04 16:10:57
package pool

import (
	"fmt"

	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/config"
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

// Manager 全局协程池管理器
//
//	author centonhuang
//	update 2026-03-18 10:00:00
type Manager struct {
	messageDAO       *dao.MessageDAO
	sessionDAO       *dao.SessionDAO
	toolDAO          *dao.ToolDAO
	pingPool         pond.Pool
	messageStorePool pond.Pool
}

var poolManager *Manager

// InitPoolManager 初始化全局协程池管理器
//
//	@author centonhuang
//	@update 2026-01-31 03:37:28
func InitPoolManager() {
	poolManager = &Manager{
		messageDAO:       dao.GetMessageDAO(),
		sessionDAO:       dao.GetSessionDAO(),
		toolDAO:          dao.GetToolDAO(),
		pingPool:         pond.NewPool(config.PoolWorkers, pond.WithQueueSize(config.PoolQueueSize)),
		messageStorePool: pond.NewPool(config.PoolWorkers, pond.WithQueueSize(config.PoolQueueSize)),
	}
}

// GetPoolManager 获取全局协程池管理器实例
//
//	return *PoolManager
//	author centonhuang
//	update 2026-01-31 16:00:00
func GetPoolManager() *Manager {
	return poolManager
}

// StopPoolManager 停止全局协程池管理器
//
//	@author centonhuang
//	@update 2026-01-31 03:47:43
func StopPoolManager() {
	if poolManager != nil {
		poolManager.Stop()
	}
}

// SubmitImageUploadTask InitImageUploadPool 初始化图片上传协程池
//
//	@receiver pm *PoolManager
//	@param task
//	@return error
//	@author centonhuang
//	@update 2026-02-04 16:10:57
func (pm *Manager) SubmitPingTask(task *dto.PingTask) error {
	logger := logger.WithCtx(task.Ctx)
	return pm.pingPool.Go(func() {
		logger.Info("[PoolManager] async ping success")
	})
}

// SubmitMessageStoreTask 提交消息存储任务到协程池
//
//	@receiver pm *Manager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func (pm *Manager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	logger := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)
	return pm.messageStorePool.Go(func() {
		messages := lo.Map(task.Messages, func(m *dto.UnifiedMessage, _ int) *dbmodel.Message {
			model := ""
			if lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role) {
				model = task.Model
			}
			return &dbmodel.Message{
				Model:    model,
				Message:  m,
				CheckSum: util.ComputeMessageChecksum(m),
			}
		})

		tools := lo.Map(task.Tools, func(t *dto.UnifiedTool, _ int) *dbmodel.Tool {
			return &dbmodel.Tool{
				Tool:     t,
				CheckSum: util.ComputeToolChecksum(t),
			}
		})

		err := db.Transaction(func(tx *gorm.DB) error {
			// 存储消息（批量IN查询去重）
			messageIDs, err := pm.deduplicateAndStoreMessages(tx, messages)
			if err != nil {
				logger.Error("[submitMessageStoreTask] failed to store messages", zap.Error(err))
				return err
			}

			// 存储工具（批量IN查询去重）
			toolIDs, err := pm.deduplicateAndStoreTools(tx, tools)
			if err != nil {
				logger.Error("[submitMessageStoreTask] failed to store tools", zap.Error(err))
				return err
			}

			session := &dbmodel.Session{
				APIKeyName: task.APIKeyName,
				MessageIDs: messageIDs,
				ToolIDs:    toolIDs,
			}
			if err := pm.sessionDAO.Create(tx, session); err != nil {
				logger.Error("[submitMessageStoreTask] failed to create session", zap.Error(err))
				return err
			}
			return nil
		})
		if err != nil {
			logger.Error("[submitMessageStoreTask] failed to store messages", zap.Error(err))
			return
		}
		logger.Info("[submitMessageStoreTask] messages stored successfully")
	})
}

// deduplicateAndStoreMessages 批量去重并存储消息
//
//	使用 IN 查询一次性获取已存在的消息，批量创建不存在的消息，保持原始顺序返回 ID 列表
//	@receiver pm *Manager
//	@param tx *gorm.DB
//	@param messages []*dbmodel.Message
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func (pm *Manager) deduplicateAndStoreMessages(tx *gorm.DB, messages []*dbmodel.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	// 提取所有 checksum，用 IN 一次性查询已存在的消息
	checksums := lo.Map(messages, func(m *dbmodel.Message, _ int) string {
		return m.CheckSum
	})

	existingMessages, err := pm.messageDAO.BatchGetByField(tx, "check_sum", checksums, []string{"id", "check_sum", "model"})
	if err != nil {
		return nil, err
	}

	// 构建 checksum+model -> ID 的映射，用于精确匹配
	existingMap := lo.SliceToMap(existingMessages, func(m *dbmodel.Message) (string, uint) {
		return fmt.Sprintf("%s:%s", m.CheckSum, m.Model), m.ID
	})

	// 分离已存在和需要新建的消息
	newMessages := lo.Filter(messages, func(m *dbmodel.Message, _ int) bool {
		key := fmt.Sprintf("%s:%s", m.CheckSum, m.Model)
		_, exists := existingMap[key]
		return !exists
	})

	// 批量创建不存在的消息
	if len(newMessages) > 0 {
		if err := pm.messageDAO.BatchCreate(tx, newMessages); err != nil {
			return nil, err
		}
		// 将新创建的消息加入映射
		for _, m := range newMessages {
			key := fmt.Sprintf("%s:%s", m.CheckSum, m.Model)
			existingMap[key] = m.ID
		}
	}

	// 按原始顺序收集所有消息 ID
	messageIDs := lo.Map(messages, func(m *dbmodel.Message, _ int) uint {
		key := fmt.Sprintf("%s:%s", m.CheckSum, m.Model)
		return existingMap[key]
	})

	return messageIDs, nil
}

// deduplicateAndStoreTools 批量去重并存储工具
//
//	使用 IN 查询一次性获取已存在的工具，批量创建不存在的工具，保持原始顺序返回 ID 列表
//	@receiver pm *Manager
//	@param tx *gorm.DB
//	@param tools []*dbmodel.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func (pm *Manager) deduplicateAndStoreTools(tx *gorm.DB, tools []*dbmodel.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	// 提取所有 checksum，用 IN 一次性查询已存在的工具
	checksums := lo.Map(tools, func(t *dbmodel.Tool, _ int) string {
		return t.CheckSum
	})

	existingTools, err := pm.toolDAO.BatchGetByField(tx, "check_sum", checksums, []string{"id", "check_sum"})
	if err != nil {
		return nil, err
	}

	// 构建 checksum -> ID 的映射
	existingMap := lo.SliceToMap(existingTools, func(t *dbmodel.Tool) (string, uint) {
		return t.CheckSum, t.ID
	})

	// 分离需要新建的工具
	newTools := lo.Filter(tools, func(t *dbmodel.Tool, _ int) bool {
		_, exists := existingMap[t.CheckSum]
		return !exists
	})

	// 批量创建不存在的工具
	if len(newTools) > 0 {
		if err := pm.toolDAO.BatchCreate(tx, newTools); err != nil {
			return nil, err
		}
		// 将新创建的工具加入映射
		for _, t := range newTools {
			existingMap[t.CheckSum] = t.ID
		}
	}

	// 按原始顺序收集所有工具 ID
	toolIDs := lo.Map(tools, func(t *dbmodel.Tool, _ int) uint {
		return existingMap[t.CheckSum]
	})

	return toolIDs, nil
}

// Stop 停止所有协程池
//
//	author centonhuang
//	update 2026-01-31 16:00:00
func (pm *Manager) Stop() {
	if pm.pingPool != nil {
		pm.pingPool.Stop()
	}
	if pm.messageStorePool != nil {
		pm.messageStorePool.Stop()
	}
}
