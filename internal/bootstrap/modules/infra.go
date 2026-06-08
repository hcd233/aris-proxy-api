package modules

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var InfraModule = fx.Module(constant.DigNameInfraModule,
	fx.Provide(
		NewDB,
		NewCache,
		NewPoolManager,
		NewInflightTracker,
	),
	fx.Invoke(InitHTTPClient),
)

func NewDB() *gorm.DB {
	return database.InitDatabase()
}

func NewCache() *redis.Client {
	return cache.InitCache()
}

func NewPoolManager(db *gorm.DB) *pool.PoolManager {
	return pool.NewPoolManager(db)
}

func NewInflightTracker() *inflight.Tracker {
	return inflight.NewTracker()
}

func InitHTTPClient() {
	httpclient.InitHTTPClient()
}
