package cron

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CronDeps 创建 cron 实例所需的依赖
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronDeps struct {
	DB          *gorm.DB
	PoolManager *pool.PoolManager
	Cache       *redis.Client
	ThinkRepo   conversation.ThinkExtractRepository
}

// managedEntry 管理中的 cron 实例
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type managedEntry struct {
	cron    Cron
	spec    string
	factory func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron
}

// CronManager cron 实例热重载管理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronManager struct {
	mu      sync.RWMutex
	entries map[string]*managedEntry
	deps    CronDeps
}

// NewCronManager 构造 CronManager
//
//	@param deps CronDeps
//	@return *CronManager
func NewCronManager(deps CronDeps) *CronManager {
	return &CronManager{
		entries: make(map[string]*managedEntry),
		deps:    deps,
	}
}

// Register 注册运行中的 cron 实例
//
//	@receiver m *CronManager
//	@param name string
//	@param c Cron
//	@param spec string
//	@param factory func(...) Cron
func (m *CronManager) Register(name string, c Cron, spec string, factory func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[name] = &managedEntry{
		cron:    c,
		spec:    spec,
		factory: factory,
	}
}

// Restart 停旧启新（热重载）
//
//	@receiver m *CronManager
//	@param name string
//	@param newSpec string
//	@return error
func (m *CronManager) Restart(name, newSpec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return ierr.New(ierr.ErrDataNotExists, "cron job "+name+" not found in manager")
	}

	// 停旧实例（仅停止调度，不等待运行中任务）
	entry.cron.StopGracefully()
	logger.Logger().Info("[CronManager] Stopped old cron instance", zap.String("name", name))

	// 用新 spec 创建新实例
	newCron := entry.factory(m.deps.DB, m.deps.PoolManager, m.deps.Cache, m.deps.ThinkRepo)
	if err := newCron.Start(newSpec); err != nil {
		logger.Logger().Error("[CronManager] Failed to start new cron instance",
			zap.String("name", name), zap.Error(err))
		return err
	}

	m.entries[name] = &managedEntry{
		cron:    newCron,
		spec:    newSpec,
		factory: entry.factory,
	}

	logger.Logger().Info("[CronManager] Restarted cron instance with new spec",
		zap.String("name", name), zap.String("spec", newSpec))
	return nil
}

// Disable 停止指定任务（只停不启）
//
//	@receiver m *CronManager
//	@param name string
//	@return error
func (m *CronManager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return ierr.New(ierr.ErrDataNotExists, "cron job "+name+" not found in manager")
	}

	entry.cron.StopGracefully()
	logger.Logger().Info("[CronManager] Disabled cron instance", zap.String("name", name))

	return nil
}

// Enable 启用指定任务（从停用状态恢复）
//
//	@receiver m *CronManager
//	@param name string
//	@param spec string
//	@return error
func (m *CronManager) Enable(name, spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return ierr.New(ierr.ErrDataNotExists, "cron job "+name+" not found in manager")
	}

	newCron := entry.factory(m.deps.DB, m.deps.PoolManager, m.deps.Cache, m.deps.ThinkRepo)
	if err := newCron.Start(spec); err != nil {
		logger.Logger().Error("[CronManager] Failed to enable cron instance",
			zap.String("name", name), zap.Error(err))
		return err
	}

	m.entries[name] = &managedEntry{
		cron:    newCron,
		spec:    spec,
		factory: entry.factory,
	}

	logger.Logger().Info("[CronManager] Enabled cron instance", zap.String("name", name))
	return nil
}

// StopAll 优雅关闭所有 cron 实例
//
//	@receiver m *CronManager
//	@param ctx context.Context
//	@return error
func (m *CronManager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for name, entry := range m.entries {
			entry.cron.Stop()
			logger.Logger().Info("[CronManager] Stopped cron instance", zap.String("name", name))
		}
	}()

	select {
	case <-done:
		logger.Logger().Info("[CronManager] All cron jobs stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
