package modules

import (
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/prometheus/client_golang/prometheus"
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
		metrics.NewRegistry,
		NewSSEGauge,
		NewHTTPCollector,
		NewMetricsMiddleware,
		NewRuntimeMetricsCache,
		NewMetricsFlusher,
	),
	fx.Invoke(InitHTTPClient),
)

func NewDB() *gorm.DB {
	return database.InitDatabase()
}

func NewCache() *redis.Client {
	return cache.InitCache()
}

func NewPoolManager(db *gorm.DB, auditRepo modelcall.AuditRepository) *pool.PoolManager {
	return pool.NewPoolManager(db, auditRepo)
}

func NewInflightTracker() *inflight.Tracker {
	return inflight.NewTracker()
}

func InitHTTPClient() {
	httpclient.InitHTTPClient()
}

func NewSSEGauge(registry *prometheus.Registry) *metrics.SSEGauge {
	return metrics.NewSSEGauge(registry)
}

func NewHTTPCollector(registry *prometheus.Registry) *metrics.HTTPCollector {
	return metrics.NewHTTPCollector(registry)
}

func NewMetricsMiddleware(collector *metrics.HTTPCollector) fiber.Handler {
	return collector.Middleware()
}

func NewRuntimeMetricsCache(client *redis.Client) *cache.RuntimeMetricsCache {
	return cache.NewRuntimeMetricsCache(client)
}

func NewMetricsFlusher(registry *prometheus.Registry, store *cache.RuntimeMetricsCache) *metrics.Flusher {
	return metrics.NewFlusher(registry, store, constant.RuntimeMetricsFlushInterval, constant.RuntimeMetricsRetention)
}
