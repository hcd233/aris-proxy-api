// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"
)

// PoolManager 全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolManager struct {
	storePool pond.Pool
	agentPool pond.Pool
}

var poolManager *PoolManager

// InitPoolManager 初始化全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func InitPoolManager() {
	poolManager = &PoolManager{
		storePool: pond.NewPool(config.Pool.Store.Workers, pond.WithQueueSize(config.Pool.Store.QueueSize)),
		agentPool: pond.NewPool(config.Pool.Agent.Workers, pond.WithQueueSize(config.Pool.Agent.QueueSize)),
	}
}

// GetPoolManager 获取全局协程池管理器实例
//
//	@return *PoolManager
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func GetPoolManager() *PoolManager {
	return poolManager
}

// StopPoolManager 停止全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func StopPoolManager() {
	if poolManager != nil {
		poolManager.Stop()
	}
}

// deduplicateAndStoreMessages 批量去重并存储消息
//
//	使用 IN 查询一次性获取已存在的消息，批量创建不存在的消息，保持原始顺序返回 ID 列表
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param messages []*dbmodel.Message
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) deduplicateAndStoreMessages(tx *gorm.DB, messages []*dbmodel.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	checksums := make([]string, len(messages))
	for i, m := range messages {
		checksums[i] = m.CheckSum
	}

	messageDAO := dao.GetMessageDAO()
	existingMessages, err := messageDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.MessageRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}

	existingMap := make(map[string]uint, len(existingMessages))
	for _, m := range existingMessages {
		existingMap[m.CheckSum] = m.ID
	}

	newMessages := make([]*dbmodel.Message, 0)
	for _, m := range messages {
		if _, exists := existingMap[m.CheckSum]; !exists {
			newMessages = append(newMessages, m)
		}
	}

	if len(newMessages) > 0 {
		if err := messageDAO.BatchCreate(tx, newMessages); err != nil {
			return nil, err
		}
		// BatchCreate 后 GORM 已填充 ID，更新 map
		for _, nm := range newMessages {
			existingMap[nm.CheckSum] = nm.ID
		}
	}

	messageIDs := make([]uint, len(messages))
	for i, m := range messages {
		messageIDs[i] = existingMap[m.CheckSum]
	}

	return messageIDs, nil
}

// deduplicateAndStoreTools 批量去重并存储工具
//
//	使用 IN 查询一次性获取已存在的工具，批量创建不存在的工具，保持原始顺序返回 ID 列表
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param tools []*dbmodel.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) deduplicateAndStoreTools(tx *gorm.DB, tools []*dbmodel.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	checksums := make([]string, len(tools))
	for i, t := range tools {
		checksums[i] = t.CheckSum
	}

	toolDAO := dao.GetToolDAO()
	existingTools, err := toolDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.ToolRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}

	existingMap := make(map[string]uint, len(existingTools))
	for _, t := range existingTools {
		existingMap[t.CheckSum] = t.ID
	}

	newTools := make([]*dbmodel.Tool, 0)
	for _, t := range tools {
		if _, exists := existingMap[t.CheckSum]; !exists {
			newTools = append(newTools, t)
		}
	}

	if len(newTools) > 0 {
		if err := toolDAO.BatchCreate(tx, newTools); err != nil {
			return nil, err
		}
		// BatchCreate 后 GORM 已填充 ID，更新 map
		for _, nt := range newTools {
			existingMap[nt.CheckSum] = nt.ID
		}
	}

	toolIDs := make([]uint, len(tools))
	for i, t := range tools {
		toolIDs[i] = existingMap[t.CheckSum]
	}

	return toolIDs, nil
}

// Stop 停止所有协程池
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) Stop() {
	if pm.storePool != nil {
		pm.storePool.Stop()
	}
	if pm.agentPool != nil {
		pm.agentPool.Stop()
	}
}
