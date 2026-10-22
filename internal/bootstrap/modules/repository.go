package modules

import (
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	demoauditport "github.com/hcd233/aris-proxy-api/internal/application/demoaccessaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	oauthport "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauthsvc "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	infraoauth "github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var RepositoryModule = fx.Module(constant.DigNameRepositoryModule,
	fx.Provide(
		NewUserRepository,
		NewAPIKeyRepository,
		NewSessionReadRepository,
		NewSessionWriteRepository,
		NewAuditRepository,
		NewDemoConfigRepository,
		NewDemoSessionRepository,
		NewEndpointRepository,
		NewModelRepository,
		NewEndpointReadRepository,
		NewOAuthEndpointRepository,
		NewOAuthCredentialRepository,
		NewOAuthFlowStore,
		NewOAuthAccessCache,
		NewOAuthClients,
		NewOAuthTokenProvider,
		NewAudioDirCreator,
		NewShareCache,
		fx.Annotate(
			NewShareCache,
			fx.As(new(sessionport.ShareCreator)),
		),
		NewSessionDetailCache,
		NewEndpointGuard,
		NewOpenAIProxy,
		NewAnthropicProxy,
		NewAPIKeyGenerator,
		NewOauth2Platforms,
		NewStateManager,
		NewTaskSubmitter,
		cache.NewEndpointAffinityCache,
		NewEndpointResolver,
		NewTriggerRepository,
		NewTriggerCache,
		NewCronRepository,
		NewCronCallAuditRepository,
		NewDemoAccessAuditRepository,
		fx.Annotate(
			NewAccessTokenSignerImpl,
			fx.ResultTags(`name:"accessSigner"`),
		),
		fx.Annotate(
			NewRefreshTokenSignerImpl,
			fx.ResultTags(`name:"refreshSigner"`),
		),
	),
)

func NewUserRepository(db *gorm.DB) identity.UserRepository {
	return repository.NewUserRepository(db)
}

func NewAPIKeyRepository(db *gorm.DB) apikey.APIKeyRepository {
	return repository.NewAPIKeyRepository(db)
}

func NewSessionReadRepository(db *gorm.DB, rc *cache.ReadCache) session.SessionReadRepository {
	return repository.NewSessionReadRepository(db, rc)
}

func NewSessionWriteRepository(db *gorm.DB) session.SessionRepository {
	return repository.NewSessionRepository(db)
}

func NewAuditRepository(db *gorm.DB, rc *cache.ReadCache) modelcall.AuditRepository {
	return repository.NewAuditRepository(db, rc)
}

func NewDemoConfigRepository(db *gorm.DB) demoport.DemoConfigRepository {
	return repository.NewDemoConfigRepository(db)
}

func NewDemoAccessAuditRepository(db *gorm.DB) demoauditport.DemoAccessAuditRepository {
	return repository.NewDemoAccessAuditRepository(db)
}

func NewDemoSessionRepository(db *gorm.DB) demoport.DemoSessionRepository {
	return repository.NewDemoSessionRepository(db)
}

func NewEndpointRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.EndpointRepository {
	return repository.NewCachedEndpointRepository(db, readCache)
}

func NewModelRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.ModelRepository {
	return repository.NewCachedModelRepository(db, readCache)
}

func NewEndpointReadRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.EndpointReadRepository {
	return repository.NewCachedEndpointReadRepository(db, readCache)
}

// NewOAuthEndpointRepository 订阅端点原子创建仓储（OAuthEndpointRepository 接口视图）
func NewOAuthEndpointRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.OAuthEndpointRepository {
	return repository.NewOAuthEndpointRepository(db, readCache)
}

// NewOAuthCredentialRepository 订阅账号凭据仓储（token 密文读写）
func NewOAuthCredentialRepository(db *gorm.DB) llmproxy.OAuthCredentialRepository {
	return repository.NewOAuthCredentialRepository(db)
}

// NewOAuthFlowStore 订阅账号授权流存储（Redis，一次性 Take）
func NewOAuthFlowStore(redisClient *redis.Client) llmproxyservice.OAuthFlowStore {
	return cache.NewOAuthFlowCache(redisClient)
}

// NewOAuthAccessCache 订阅账号 access token 缓存（Redis，token 以同一把凭据密钥加密）
func NewOAuthAccessCache(redisClient *redis.Client) llmproxyservice.OAuthAccessCache {
	return cache.NewOAuthAccessCache(redisClient, config.OAuthCredentialKey)
}

// NewOAuthClients 订阅账号 provider 客户端集合（Claude / Codex）
func NewOAuthClients() map[enum.OAuthProvider]llmproxyservice.OAuthProviderClient {
	return oauthprovider.NewClients()
}

// NewOAuthTokenProvider 订阅账号 token 提供者（锁/缓存/刷新）
func NewOAuthTokenProvider(
	credRepo llmproxy.OAuthCredentialRepository,
	redisClient *redis.Client,
) llmproxyservice.OAuthTokenProvider {
	return oauthprovider.NewDefaultTokenProvider(credRepo, redisClient)
}

func NewAudioDirCreator() oauthport.ObjectStorageDirCreator {
	if config.CosAppID == "" && config.MinioEndpoint == "" {
		return nil
	}
	return repository.NewAudioDirCreator()
}

func NewShareCache(redisClient *redis.Client) cache.ShareCache {
	return cache.NewShareCache(redisClient)
}

func NewSessionDetailCache(redisClient *redis.Client) sessionport.SessionDetailCache {
	return cache.NewSessionDetailCache(redisClient)
}

// NewOpenAIProxy 装配 OpenAI 上游代理：外层 mock 装饰器按开关与标记决定本地生成，
// 内层为真实 HTTP 代理。装饰器默认不生效（config.MockUpstreamEnabled=false）。
func NewOpenAIProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.OpenAIProxyPort {
	return transport.NewMockOpenAIProxy(transport.NewOpenAIProxy(tracker, guard))
}

// NewAnthropicProxy 同 NewOpenAIProxy，覆盖 Anthropic 协议。
func NewAnthropicProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.AnthropicProxyPort {
	return transport.NewMockAnthropicProxy(transport.NewAnthropicProxy(tracker, guard))
}

func NewEndpointGuard(registry *prometheus.Registry) *transport.EndpointGuard {
	return transport.NewEndpointGuard(registry)
}

func NewAPIKeyGenerator() apikeyservice.APIKeyGenerator {
	return apikeyservice.NewAPIKeyGenerator()
}

func NewOauth2Platforms() map[string]oauthsvc.Platform {
	return map[string]oauthsvc.Platform{
		enum.Oauth2PlatformGithub: infraoauth.NewGithubPlatform(),
		enum.Oauth2PlatformGoogle: infraoauth.NewGooglePlatform(),
	}
}

func NewStateManager(redisClient *redis.Client) oauthsvc.StateManager {
	return infraoauth.NewRedisStateManager(redisClient)
}

func NewTaskSubmitter(pm *pool.PoolManager) usecase.TaskSubmitter {
	return pm
}

func NewEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
	affinity llmproxyservice.EndpointAffinity,
) llmproxyservice.EndpointResolver {
	return llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo, config.GatewaySharedPoolFallback, affinity)
}

func NewTriggerRepository(db *gorm.DB) trigger.TriggerRepository {
	return repository.NewTriggerRepository(db)
}

func NewTriggerCache(c *redis.Client) *cache.TriggerHitCache {
	return cache.NewTriggerHitCache(c)
}

func NewCronRepository(db *gorm.DB) cronmgmtport.CronJobRepository {
	return repository.NewCronRepository(db)
}

func NewCronCallAuditRepository(db *gorm.DB, rc *cache.ReadCache) cronauditport.CronCallAuditRepository {
	return repository.NewCronCallAuditRepository(db, rc)
}

func NewAccessTokenSignerImpl() identityservice.TokenSigner {
	return jwt.NewAccessTokenSigner()
}

func NewRefreshTokenSignerImpl() identityservice.TokenSigner {
	return jwt.NewRefreshTokenSigner()
}
