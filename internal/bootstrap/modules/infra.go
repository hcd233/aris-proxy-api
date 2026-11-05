package modules

import (
	"github.com/gofiber/fiber/v3"
	demoauditport "github.com/hcd233/aris-mass/internal/application/demoaccessaudit/port"
	modelport "github.com/hcd233/aris-mass/internal/application/model/port"
	sessionport "github.com/hcd233/aris-mass/internal/application/session/port"
	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/inflight"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/domain/modelcall"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/cache"
	"github.com/hcd233/aris-mass/internal/infrastructure/database"
	"github.com/hcd233/aris-mass/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-mass/internal/infrastructure/metrics"
	"github.com/hcd233/aris-mass/internal/infrastructure/modelsdev"
	"github.com/hcd233/aris-mass/internal/infrastructure/objectstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/pool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var InfraModule = fx.Module(constant.DigNameInfraModule,
	fx.Provide(
		NewDB,
		NewCache,
		NewReadCache,
		NewObjectStore,
		NewBlobExternalizer,
		NewPoolManager,
		NewInflightTracker,
		metrics.NewRegistry,
		NewSSEGauge,
		NewTokenUsageCounter,
		NewHTTPCollector,
		NewMetricsMiddleware,
		NewRuntimeMetricsCache,
		NewMetricsFlusher,
		NewModelsDevClient,
		NewModelSpecProvider,
	),
	fx.Invoke(InitHTTPClient),
)

// NewModelsDevClient 构造 models.dev 公开规格客户端（复用通用 HTTP 客户端与 Redis 缓存）
func NewModelsDevClient(cache *redis.Client) *modelsdev.Client {
	return modelsdev.NewClient(httpclient.GetHTTPClient(), cache)
}

// NewModelSpecProvider 绑定公开模型规格来源端口到 modelsdev 客户端
func NewModelSpecProvider(client *modelsdev.Client) modelport.ModelSpecProvider {
	return client
}

func NewDB() *gorm.DB {
	return database.InitDatabase()
}

func NewCache() *redis.Client {
	return cache.InitCache()
}

// NewReadCache 构造通用读缓存（模型目录类读接口的 cache-aside，防穿透/雪崩/击穿）
func NewReadCache(client *redis.Client) *cache.ReadCache {
	return cache.NewReadCache(client)
}

func NewPoolManager(
	db *gorm.DB,
	auditRepo modelcall.AuditRepository,
	demoAccessAuditRepo demoauditport.DemoAccessAuditRepository,
	sessionCache sessionport.SessionDetailCache,
	externalizer *blobstore.Externalizer,
) *pool.PoolManager {
	return pool.NewPoolManager(db, auditRepo, demoAccessAuditRepo, sessionCache, externalizer)
}

// NewObjectStore 消息图片对象存储；未配置时为 nil（图片暂存 blobs 表）
func NewObjectStore() blob.ObjectStore {
	return objectstore.NewFromConfig()
}

// NewBlobExternalizer 消息图片外置器
func NewBlobExternalizer(db *gorm.DB, store blob.ObjectStore) *blobstore.Externalizer {
	return blobstore.NewExternalizer(db, store)
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

func NewTokenUsageCounter(registry *prometheus.Registry) *metrics.TokenUsageCounter {
	return metrics.NewTokenUsageCounter(registry)
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
