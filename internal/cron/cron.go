// Package cron 定时任务模块
//
//	update 2024-12-09 15:55:25
package cron

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Cron 定时任务接口
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
type Cron interface {
	Start() error
	Stop()
}

// CronRegistryEntry 单个定时任务注册项
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type CronRegistryEntry struct {
	Name              string
	Enabled           func() bool
	Factory           func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) Cron
	LockTTL           time.Duration // 0 → constant.CronLockDefaultTTL
	LockRenewInterval time.Duration // 0 → ttl / constant.CronLockDefaultRenewDivisor
}

var cronInstances []Cron

// DefaultCronRegistry 默认定时任务注册表，保留用于测试覆盖。生产由 buildRegistryEntries 构造
// （需要 cache 注入锁依赖，registry Factory 签名不接受 cache 故退化）。
//
//	@update 2026-06-01 10:00:00
var DefaultCronRegistry []CronRegistryEntry

// InitCronJobs 初始化定时任务（每个 cron 自带分布式锁）
//
// parentCtx 通常传入 bootstrap 阶段的 shutdown context；nil 时退化为 context.Background()。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func InitCronJobs(parentCtx context.Context, db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) {
	SetBootstrapContext(parentCtx)
	var entries []CronRegistryEntry
	if len(DefaultCronRegistry) > 0 {
		entries = DefaultCronRegistry
	} else {
		entries = buildRegistryEntries()
	}
	for _, entry := range entries {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		c := entry.Factory(db, poolManager, cache)
		lo.Must0(c.Start())
		cronInstances = append(cronInstances, c)
		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(cronInstances)))
}

func buildRegistryEntries() []CronRegistryEntry {
	return []CronRegistryEntry{
		{
			Name:    constant.CronModuleSessionDeduplicate,
			Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client) Cron {
				return NewSessionDeduplicateCron(db, cache)
			},
		},
		{
			Name:    constant.CronModuleSessionSummarize,
			Enabled: func() bool { return config.CronSessionSummarizeEnabled },
			Factory: NewSessionSummarizeCron,
		},
		{
			Name:    constant.CronModuleSessionScore,
			Enabled: func() bool { return config.CronSessionScoreEnabled },
			Factory: NewSessionScoreCron,
		},
		{
			Name:    constant.CronModuleSoftDeletePurge,
			Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client) Cron {
				return NewSoftDeletePurgeCron(db, cache)
			},
		},
		{
			Name:    constant.CronModuleThinkExtract,
			Enabled: func() bool { return config.CronThinkExtractEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client) Cron {
				return NewThinkExtractCron(db, cache)
			},
		},
	}
}

// StopCronJobs 停止所有定时任务，用于优雅关闭
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func StopCronJobs() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, c := range cronInstances {
			c.Stop()
		}
	}()

	select {
	case <-done:
		logger.Logger().Info("[Cron] All cron jobs stopped")
	case <-time.After(constant.CronStopTimeout):
		logger.Logger().Warn("[Cron] Cron stop timed out, some jobs may not have completed",
			zap.Duration("timeout", constant.CronStopTimeout))
	}
	cronInstances = nil
}

// CronInstanceCount 返回当前已注册的定时任务实例数量，供测试使用
//
//	@author centonhuang
//	@update 2026-05-01 10:00:00
func CronInstanceCount() int {
	return len(cronInstances)
}

type cronLoggerAdapter struct {
	module string
	logger *zap.Logger
}

func newCronLoggerAdapter(module string) *cronLoggerAdapter {
	if module == "" {
		module = constant.CronDefaultModule
	}
	module = strings.TrimSpace(strings.TrimRight(strings.TrimLeft(strings.TrimSpace(module), "["), "]"))
	return &cronLoggerAdapter{module: module, logger: logger.Logger()}
}

func (l *cronLoggerAdapter) Error(err error, msg string, keysAndValues ...any) {
	zapKeyValues := []zap.Field{zap.Error(err)}
	zapKeyValues = append(zapKeyValues, convertZapKeyValues(keysAndValues...)...)
	l.logger.Error(fmt.Sprintf("[%s] %s", l.module, capitalizeFirst(msg)), zapKeyValues...)
}

func (l *cronLoggerAdapter) Info(msg string, keysAndValues ...any) {
	zapKeyValues := convertZapKeyValues(keysAndValues...)
	l.logger.Info(fmt.Sprintf("[%s] %s", l.module, capitalizeFirst(msg)), zapKeyValues...)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+utf8.RuneLen(r):]
	}
	return s
}

func convertZapKeyValues(keysAndValues ...any) []zap.Field {
	if len(keysAndValues)%2 != 0 {
		return []zap.Field{zap.String("error", "keysAndValues must be a slice of key-value pairs")}
	}
	kvLen := len(keysAndValues) / 2
	zapKeyValues := make([]zap.Field, 0, kvLen)
	for i := range kvLen {
		key, ok := keysAndValues[i*2].(string)
		if !ok {
			key = constant.CronInvalidKey
		}
		value := keysAndValues[i*2+1]
		zapKeyValues = append(zapKeyValues, zap.Any(key, value))
	}
	return zapKeyValues
}
