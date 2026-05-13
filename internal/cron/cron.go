// Package cron 定时任务模块
//
//	update 2024-12-09 15:55:25
package cron

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
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
//	@update 2026-05-01 10:00:00
type CronRegistryEntry struct {
	Name    string
	Enabled func() bool
	Factory func(db *gorm.DB, poolManager *pool.PoolManager) Cron
}

var cronInstances []Cron

// DefaultCronRegistry 默认定时任务注册表，用于测试注入
//
//	@update 2026-05-01 10:00:00
var DefaultCronRegistry = []CronRegistryEntry{
	{
		Name:    constant.CronModuleSessionDeduplicate,
		Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
		Factory: func(db *gorm.DB, _ *pool.PoolManager) Cron { return NewSessionDeduplicateCron(db) },
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
		Factory: func(db *gorm.DB, _ *pool.PoolManager) Cron { return NewSoftDeletePurgeCron(db) },
	},
}

// InitCronJobs 初始化定时任务
//
//	author centonhuang
//	update 2026-04-02 10:00:00
func InitCronJobs(db *gorm.DB, poolManager *pool.PoolManager) {
	for _, entry := range DefaultCronRegistry {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		c := entry.Factory(db, poolManager)
		lo.Must0(c.Start())
		cronInstances = append(cronInstances, c)
		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(cronInstances)))
}

// StopCronJobs 停止所有定时任务，用于优雅关闭
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func StopCronJobs() {
	for _, c := range cronInstances {
		c.Stop()
	}
	cronInstances = nil
	logger.Logger().Info("[Cron] All cron jobs stopped")
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
