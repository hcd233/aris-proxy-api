package modules

import (
	"context"

	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var CronModule = fx.Module(constant.DigNameCronModule,
	fx.Provide(
		NewThinkExtractRepo,
		NewCronManager,
		NewCronEntries,
	),
)

func NewThinkExtractRepo(db *gorm.DB) conversation.ThinkExtractRepository {
	return repository.NewThinkExtractRepository(db)
}

func NewCronManager(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) *cron.CronManager {
	return cron.NewCronManager(cron.CronDeps{
		DB:          db,
		PoolManager: poolManager,
		Cache:       cache,
		ThinkRepo:   thinkRepo,
	})
}

func NewCronEntries(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository, cronJobRepo cronmgmtport.CronJobRepository, cronCallAuditRepo cronauditport.CronCallAuditRepository, manager *cron.CronManager) []cron.Cron {
	return cron.InitCronJobs(context.Background(), db, poolManager, cache, thinkRepo, cronJobRepo, cronCallAuditRepo, manager)
}
