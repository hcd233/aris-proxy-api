package cron

import (
	"context"
	"os"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
	podID   string
	pubSub  *redis.PubSub
}

// NewCronManager 构造 CronManager
//
//	@param deps CronDeps
//	@return *CronManager
func NewCronManager(deps CronDeps) *CronManager {
	podID, err := os.Hostname()
	if err != nil {
		podID = constant.CronDefaultModule
	}
	return &CronManager{
		entries: make(map[string]*managedEntry),
		deps:    deps,
		podID:   podID,
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

// StartListener 启动 Redis pub/sub 监听，接收来自其他 Pod 的 cron 变更通知
//
//	@receiver m *CronManager
//	@param parentCtx context.Context
func (m *CronManager) StartListener(parentCtx context.Context) {
	if m.deps.Cache == nil {
		return
	}
	m.pubSub = m.deps.Cache.Subscribe(parentCtx, constant.CronReloadChannel)
	go func() {
		for msg := range m.pubSub.Channel() {
			m.handleMessage(msg)
		}
	}()
	logger.Logger().Info("[CronManager] Started pub/sub listener", zap.String("channel", constant.CronReloadChannel))
}

// cronReloadMsg Redis pub/sub 消息体
type cronReloadMsg struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Pod    string `json:"pod"`
}

// handleMessage 处理来自 Redis pub/sub 的 cron 变更消息
//
//	@receiver m *CronManager
//	@param msg *redis.Message
func (m *CronManager) handleMessage(msg *redis.Message) {
	var payload cronReloadMsg
	if err := sonic.Unmarshal([]byte(msg.Payload), &payload); err != nil {
		logger.Logger().Error("[CronManager] Failed to unmarshal reload message", zap.Error(err))
		return
	}
	if payload.Pod == m.podID {
		return
	}

	job, err := cronJobStore.Get(context.Background(), payload.Name)
	if err != nil {
		logger.Logger().Error("[CronManager] Failed to get cron job for reload",
			zap.String("name", payload.Name), zap.Error(err))
		return
	}

	switch payload.Action {
	case constant.CronReloadActionRestart:
		if job.Enabled {
			if err := m.restartLocked(payload.Name, job.Spec); err != nil {
				logger.Logger().Error("[CronManager] Failed to restart cron from reload",
					zap.String("name", payload.Name), zap.Error(err))
			}
		}
	case constant.CronReloadActionDisable:
		if err := m.disableLocked(payload.Name); err != nil {
			logger.Logger().Error("[CronManager] Failed to disable cron from reload",
				zap.String("name", payload.Name), zap.Error(err))
		}
	case constant.CronReloadActionEnable:
		if job.Enabled {
			if err := m.enableLocked(payload.Name, job.Spec); err != nil {
				logger.Logger().Error("[CronManager] Failed to enable cron from reload",
					zap.String("name", payload.Name), zap.Error(err))
			}
		}
	}
}

// publish 向 Redis pub/sub 广播 cron 变更
//
//	@receiver m *CronManager
//	@param action string
//	@param name string
func (m *CronManager) publish(action, name string) {
	if m.deps.Cache == nil {
		return
	}
	payload, err := sonic.Marshal(cronReloadMsg{Action: action, Name: name, Pod: m.podID})
	if err != nil {
		logger.Logger().Error("[CronManager] Failed to marshal reload message",
			zap.String("action", action), zap.String("name", name), zap.Error(err))
		return
	}
	if err := m.deps.Cache.Publish(context.Background(), constant.CronReloadChannel, string(payload)).Err(); err != nil {
		logger.Logger().Error("[CronManager] Failed to publish reload message",
			zap.String("action", action), zap.String("name", name), zap.Error(err))
	}
}

// Restart 停旧启新（热重载），成功后广播到其他 Pod
//
//	@receiver m *CronManager
//	@param name string
//	@param newSpec string
//	@return error
func (m *CronManager) Restart(name, newSpec string) error {
	if err := m.restartLocked(name, newSpec); err != nil {
		return err
	}
	m.publish(constant.CronReloadActionRestart, name)
	return nil
}

func (m *CronManager) restartLocked(name, newSpec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return ierr.New(ierr.ErrDataNotExists, "cron job "+name+" not found in manager")
	}

	entry.cron.StopGracefully()
	logger.Logger().Info("[CronManager] Stopped old cron instance", zap.String("name", name))

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

// Disable 停止指定任务（只停不启），成功后广播到其他 Pod
//
//	@receiver m *CronManager
//	@param name string
//	@return error
func (m *CronManager) Disable(name string) error {
	if err := m.disableLocked(name); err != nil {
		return err
	}
	m.publish(constant.CronReloadActionDisable, name)
	return nil
}

func (m *CronManager) disableLocked(name string) error {
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

// Enable 启用指定任务（从停用状态恢复），成功后广播到其他 Pod
//
//	@receiver m *CronManager
//	@param name string
//	@param spec string
//	@return error
func (m *CronManager) Enable(name, spec string) error {
	if err := m.enableLocked(name, spec); err != nil {
		return err
	}
	m.publish(constant.CronReloadActionEnable, name)
	return nil
}

func (m *CronManager) enableLocked(name, spec string) error {
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
