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
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
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
	Factory           func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron
	LockTTL           time.Duration // 0 → constant.CronLockDefaultTTL
	LockRenewInterval time.Duration // 0 → ttl / constant.CronLockDefaultRenewDivisor
}

// DefaultCronRegistry 默认定时任务注册表，保留用于测试覆盖。生产由 buildRegistryEntries 构造
// （需要 cache 注入锁依赖，registry Factory 签名不接受 cache 故退化）。
//
//	@update 2026-06-01 10:00:00
var DefaultCronRegistry []CronRegistryEntry

// InitCronJobs 初始化定时任务（每个 cron 自带分布式锁），返回创建的 cron 列表。
func InitCronJobs(parentCtx context.Context, db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) []Cron {
	SetBootstrapContext(parentCtx)
	var entries []CronRegistryEntry
	if len(DefaultCronRegistry) > 0 {
		entries = DefaultCronRegistry
	} else {
		entries = buildRegistryEntries()
	}
	var crons []Cron
	for _, entry := range entries {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		c := entry.Factory(db, poolManager, cache, thinkRepo)
		lo.Must0(c.Start())
		crons = append(crons, c)
		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(crons)))
	return crons
}

func buildRegistryEntries() []CronRegistryEntry {
	return []CronRegistryEntry{
		{
			Name:    constant.CronModuleSessionDeduplicate,
			Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSessionDeduplicateCron(db, cache)
			},
		},
		{
			Name:    constant.CronModuleSoftDeletePurge,
			Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSoftDeletePurgeCron(db, cache)
			},
		},
		{
			Name:    constant.CronModuleThinkExtract,
			Enabled: func() bool { return config.CronThinkExtractEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron {
				return NewThinkExtractCron(thinkRepo, cache)
			},
		},
	}
}

func StopCronJobsWithContext(ctx context.Context, crons []Cron) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, c := range crons {
			c.Stop()
		}
	}()

	select {
	case <-done:
		logger.Logger().Info("[Cron] All cron jobs stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func CronInstanceCount(crons []Cron) int {
	return len(crons)
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
