package modules

import (
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	oauth2port "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	infraoauth2 "github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
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
		NewEndpointRepository,
		NewModelRepository,
		NewEndpointReadRepository,
		NewAudioDirCreator,
		NewShareCache,
		NewSessionDetailCache,
		NewOpenAIProxy,
		NewAnthropicProxy,
		NewAPIKeyGenerator,
		NewOauth2Platforms,
		NewStateManager,
		NewTaskSubmitter,
		NewEndpointResolver,
		NewBlockedRepository,
		NewBlockedCache,
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

func NewSessionReadRepository(db *gorm.DB) session.SessionReadRepository {
	return repository.NewSessionReadRepository(db)
}

func NewSessionWriteRepository(db *gorm.DB) session.SessionRepository {
	return repository.NewSessionRepository(db)
}

func NewAuditRepository(db *gorm.DB) modelcall.AuditRepository {
	return repository.NewAuditRepository(db)
}

func NewEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return repository.NewEndpointRepository(db)
}

func NewModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return repository.NewModelRepository(db)
}

func NewEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return repository.NewEndpointReadRepository(db)
}

func NewAudioDirCreator() oauth2port.ObjectStorageDirCreator {
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

func NewOpenAIProxy() usecase.OpenAIProxyPort {
	return transport.NewOpenAIProxy()
}

func NewAnthropicProxy() usecase.AnthropicProxyPort {
	return transport.NewAnthropicProxy()
}

func NewAPIKeyGenerator() apikeyservice.APIKeyGenerator {
	return apikeyservice.NewAPIKeyGenerator()
}

func NewOauth2Platforms() map[string]oauth2service.Platform {
	return map[string]oauth2service.Platform{
		enum.Oauth2PlatformGithub: infraoauth2.NewGithubPlatform(),
		enum.Oauth2PlatformGoogle: infraoauth2.NewGooglePlatform(),
	}
}

func NewStateManager() oauth2service.StateManager {
	return infraoauth2.NewStateManager()
}

func NewTaskSubmitter(pm *pool.PoolManager) usecase.TaskSubmitter {
	return pm
}

func NewEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
) llmproxyservice.EndpointResolver {
	return llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo)
}

func NewBlockedRepository(db *gorm.DB) blocked.BlockedRepository {
	return repository.NewBlockedRepository(db)
}

func NewBlockedCache(c *redis.Client) *cache.BlockedHitCache {
	return cache.NewBlockedHitCache(c)
}

func NewAccessTokenSignerImpl() identityservice.TokenSigner {
	return jwt.NewAccessTokenSigner()
}

func NewRefreshTokenSignerImpl() identityservice.TokenSigner {
	return jwt.NewRefreshTokenSigner()
}
