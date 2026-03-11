// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-02-04 16:10:57
package pool

import (
	"errors"

	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Manager 全局协程池管理器
//
//	author centonhuang
//	update 2026-01-31 16:00:00
type Manager struct {
	messageDAO       *dao.MessageDAO
	sessionDAO       *dao.SessionDAO
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
//	@update 2026-03-10 10:00:00
func (pm *Manager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	logger := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)
	return pm.messageStorePool.Go(func() {
		messages := lo.Map(task.Messages, func(m *dto.ChatCompletionMessageParam, _ int) *model.Message {
			return &model.Message{
				Model:    task.Model,
				Message:  m,
				CheckSum: util.ComputeMessageChecksum(m),
			}
		})
		err := db.Transaction(func(tx *gorm.DB) error {
			messageIDs := make([]uint, 0)
			for idx, m := range messages {
				message, err := pm.messageDAO.Get(tx, &model.Message{CheckSum: m.CheckSum, Model: m.Model}, []string{"id"})
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					logger.Error("[submitMessageStoreTask] failed to get message", zap.Int("idx", idx), zap.Error(err))
					return err
				}
				// record not found
				if message == nil {
					logger.Info("[submitMessageStoreTask] message not found, creating message", zap.Int("idx", idx))
					err = pm.messageDAO.Create(tx, m)
					if err != nil {
						logger.Error("[submitMessageStoreTask] failed to create message", zap.Int("idx", idx), zap.Error(err))
						return err
					}
					message = m
				}
				messageIDs = append(messageIDs, message.ID)
			}

			session := &model.Session{
				APIKeyName: task.APIKeyName,
				MessageIDs: messageIDs,
			}
			err := pm.sessionDAO.Create(tx, session)
			if err != nil {
				logger.Error("[submitMessageStoreTask] failed to create session", zap.Error(err))
			}
			return err
		})
		if err != nil {
			logger.Error("[submitMessageStoreTask] failed to store messages", zap.Error(err))
			return
		}
		logger.Info("[submitMessageStoreTask] messages stored successfully")
	})
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
