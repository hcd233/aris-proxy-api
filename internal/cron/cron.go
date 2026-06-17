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

	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	cachepkg "github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Cron 定时任务接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type Cron interface {
	Start(spec string) error
	Stop()
	StopGracefully()
}

// CronRegistryEntry 单个定时任务注册项
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronRegistryEntry struct {
	Name              string
	Type              string
	Spec              string
	Description       string
	Enabled           func() bool
	Factory           func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron
	LockTTL           time.Duration // 0 → constant.CronLockDefaultTTL
	LockRenewInterval time.Duration // 0 → ttl / constant.CronLockDefaultRenewDivisor
}

// CronJobStore 定时任务元数据存储接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobStore interface {
	Sync(ctx context.Context, jobs []*cronmgmtport.CronJobView) error
	Get(ctx context.Context, name string) (*cronmgmtport.CronJobView, error)
}

// CronCallAuditStore 定时任务执行审计存储接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAuditStore interface {
	Save(ctx context.Context, audit *cronauditport.CronCallAuditView) error
}

var (
	cronJobStore       CronJobStore
	cronCallAuditStore CronCallAuditStore
)

// SetCronStores 设置 cron 任务元数据和审计存储
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func SetCronStores(jobStore CronJobStore, auditStore CronCallAuditStore) {
	cronJobStore = jobStore
	cronCallAuditStore = auditStore
}

// DefaultCronRegistry 默认定时任务注册表，保留用于测试覆盖。生产由 buildRegistryEntries 构造
// （需要 cache 注入锁依赖，registry Factory 签名不接受 cache 故退化）。
//
//	@update 2026-06-01 10:00:00
var DefaultCronRegistry []CronRegistryEntry

// InitCronJobs 初始化定时任务（每个 cron 自带分布式锁），返回创建的 cron 列表。
func InitCronJobs(parentCtx context.Context, db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository, jobStore CronJobStore, auditStore CronCallAuditStore, manager *CronManager) []Cron {
	SetBootstrapContext(parentCtx)
	SetCronStores(jobStore, auditStore)
	var entries []CronRegistryEntry
	if len(DefaultCronRegistry) > 0 {
		entries = DefaultCronRegistry
	} else {
		entries = buildRegistryEntries()
	}

	if cronJobStore != nil {
		jobs := lo.Map(entries, func(entry CronRegistryEntry, _ int) *cronmgmtport.CronJobView {
			return &cronmgmtport.CronJobView{
				Name:        entry.Name,
				Type:        entry.Type,
				Spec:        entry.Spec,
				Description: entry.Description,
			}
		})
		if err := cronJobStore.Sync(parentCtx, jobs); err != nil {
			logger.Logger().Error("[Cron] Sync cron jobs failed", zap.Error(err))
		}
	}

	var crons []Cron
	for _, entry := range entries {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		// 从 DB 读取实际 spec（允许与常量不同）
		actualSpec := entry.Spec
		if cronJobStore != nil {
			job, err := cronJobStore.Get(parentCtx, entry.Name)
			if err == nil && job != nil && job.Spec != "" {
				actualSpec = job.Spec
			}
		}

		c := entry.Factory(db, poolManager, cache, thinkRepo)
		lo.Must0(c.Start(actualSpec))
		crons = append(crons, c)

		// 注册到 CronManager
		if manager != nil {
			manager.Register(entry.Name, c, actualSpec, entry.Factory)
		}

		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name), zap.String("spec", actualSpec))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(crons)))
	return crons
}

func buildRegistryEntries() []CronRegistryEntry {
	return []CronRegistryEntry{
		{
			Name:        constant.CronModuleSessionDeduplicate,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecSessionDeduplicate,
			Description: constant.CronDescriptionSessionDeduplicate,
			Enabled:     func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSessionDeduplicateCron(db, cache)
			},
		},
		{
			Name:        constant.CronModuleSoftDeletePurge,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecSoftDeletePurge,
			Description: constant.CronDescriptionSoftDeletePurge,
			Enabled:     func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSoftDeletePurgeCron(db, cache)
			},
		},
		{
			Name:        constant.CronModuleThinkExtract,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecThinkExtract,
			Description: constant.CronDescriptionThinkExtract,
			Enabled:     func() bool { return config.CronThinkExtractEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron {
				return NewThinkExtractCron(thinkRepo, cache)
			},
		},
		{
			Name:        constant.CronModuleBlockedHitSync,
			Type:        constant.CronTypeCore,
			Spec:        constant.CronSpecBlockedHitSync,
			Description: constant.CronDescriptionBlockedHitSync,
			Enabled:     func() bool { return true },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				blockedRepo := repository.NewBlockedRepository(db)
				hitCache := cachepkg.NewBlockedHitCache(cache)
				return NewBlockedHitSyncCron(db, blockedRepo, hitCache, cache)
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
		raw, ok := keysAndValues[i*2].(string)
		key := mo.TupleToOption(raw, ok).OrElse(constant.CronInvalidKey)
		zapKeyValues = append(zapKeyValues, zap.Any(key, keysAndValues[i*2+1]))
	}
	return zapKeyValues
}
