package modules

import (
	"context"

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
		NewCronEntries,
	),
)

func NewThinkExtractRepo(db *gorm.DB) conversation.ThinkExtractRepository {
	return repository.NewThinkExtractRepository(db)
}

func NewCronEntries(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) []cron.Cron {
	return cron.InitCronJobs(context.Background(), db, poolManager, cache, thinkRepo)
}
