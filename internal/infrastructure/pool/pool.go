// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"context"

	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// PoolManager 全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolManager struct {
	db        *gorm.DB
	storePool pond.Pool
	agentPool pond.Pool
}

// NewPoolManager 创建协程池管理器。
//
//	@param db *gorm.DB
//	@return *PoolManager
//	@author centonhuang
//	@update 2026-05-12 20:30:00
func NewPoolManager(db *gorm.DB) *PoolManager {
	return &PoolManager{
		db:        db,
		storePool: pond.NewPool(config.Pool.Store.Workers, pond.WithQueueSize(config.Pool.Store.QueueSize)),
		agentPool: pond.NewPool(config.Pool.Agent.Workers, pond.WithQueueSize(config.Pool.Agent.QueueSize)),
	}
}

func (pm *PoolManager) StopWithContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		pm.Stop()
	}()
	select {
	case <-done:
		logger.Logger().Info("[Pool] Pool manager stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
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

	checksums := lo.Map(messages, func(m *dbmodel.Message, _ int) string { return m.CheckSum })

	messageDAO := dao.GetMessageDAO()
	existingMessages, err := messageDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.MessageRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}

	existingMap := lo.SliceToMap(existingMessages, func(m *dbmodel.Message) (string, uint) { return m.CheckSum, m.ID })

	newMessages := lo.Filter(messages, func(m *dbmodel.Message, _ int) bool {
		_, exists := existingMap[m.CheckSum]
		return !exists
	})

	if len(newMessages) > 0 {
		if err := messageDAO.BatchCreate(tx, newMessages); err != nil {
			// unique constraint 冲突（并发去重）：重新查询已存在的记录
			existingMessages, retryErr := messageDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.MessageRepoFieldsChecksum)
			if retryErr != nil {
				return nil, retryErr
			}
			existingMap = lo.SliceToMap(existingMessages, func(m *dbmodel.Message) (string, uint) { return m.CheckSum, m.ID })
			// 用 existingMap 重新填充结果即可，无需再插入
			_ = err
		} else {
			for _, nm := range newMessages {
				existingMap[nm.CheckSum] = nm.ID
			}
		}
	}

	messageIDs := lo.Map(messages, func(m *dbmodel.Message, _ int) uint { return existingMap[m.CheckSum] })

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

	checksums := lo.Map(tools, func(t *dbmodel.Tool, _ int) string { return t.CheckSum })

	toolDAO := dao.GetToolDAO()
	existingTools, err := toolDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.ToolRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}

	existingMap := lo.SliceToMap(existingTools, func(t *dbmodel.Tool) (string, uint) { return t.CheckSum, t.ID })

	newTools := lo.Filter(tools, func(t *dbmodel.Tool, _ int) bool {
		_, exists := existingMap[t.CheckSum]
		return !exists
	})

	if len(newTools) > 0 {
		if err := toolDAO.BatchCreate(tx, newTools); err != nil {
			// unique constraint 冲突（并发去重）：重新查询已存在的记录
			existingTools, retryErr := toolDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.ToolRepoFieldsChecksum)
			if retryErr != nil {
				return nil, retryErr
			}
			existingMap = lo.SliceToMap(existingTools, func(t *dbmodel.Tool) (string, uint) { return t.CheckSum, t.ID })
			_ = err
		} else {
			for _, nt := range newTools {
				existingMap[nt.CheckSum] = nt.ID
			}
		}
	}

	toolIDs := lo.Map(tools, func(t *dbmodel.Tool, _ int) uint { return existingMap[t.CheckSum] })

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
