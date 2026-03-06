// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-02-04 16:10:57
package pool

import (
	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// Manager 全局协程池管理器
//
//	author centonhuang
//	update 2026-01-31 16:00:00
type Manager struct {
	pingPool pond.Pool
}

var poolManager *Manager

// InitPoolManager 初始化全局协程池管理器
//
//	@author centonhuang
//	@update 2026-01-31 03:37:28
func InitPoolManager() {
	poolManager = &Manager{
		pingPool: pond.NewPool(config.PoolWorkers, pond.WithQueueSize(config.PoolQueueSize)),
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

// Stop 停止所有协程池
//
//	author centonhuang
//	update 2026-01-31 16:00:00
func (pm *Manager) Stop() {
	if pm.pingPool != nil {
		pm.pingPool.Stop()
	}
}
