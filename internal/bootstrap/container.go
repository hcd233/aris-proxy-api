package bootstrap

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/api"
	apikeycommand "github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	apikeyquery "github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	identitycommand "github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	identityquery "github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	modelquery "github.com/hcd233/aris-proxy-api/internal/application/model/query"
	applicationoauth2 "github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	infraoauth2 "github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// Server 启动阶段解析出的 HTTP 服务对象。
//
//	@author centonhuang
//	@update 2026-04-28 10:00:00
type Server struct {
	container *dig.Container
	App       *fiber.App
	HumaAPI   huma.API
}

// Infrastructure 启动阶段初始化完成的基础设施依赖。
//
//	@author centonhuang
//	@update 2026-05-12 20:30:00
type Infrastructure struct {
	DB          *gorm.DB
	Cache       *redis.Client
	PoolManager *pool.PoolManager
}

// InitInfrastructure 初始化所有基础设施组件（数据库、Redis、HTTP Client、协程池、定时任务）。
//
//	@return *Infrastructure
//	@author centonhuang
//	@update 2026-05-09 10:00:00
func InitInfrastructure() *Infrastructure {
	db := database.InitDatabase()
	cache := cache.InitCache()
	httpclient.InitHTTPClient()
	inflight.InitTracker()
	poolManager := pool.InitPoolManager(db)
	cron.InitCronJobs(db, poolManager, cache)
	return &Infrastructure{DB: db, Cache: cache, PoolManager: poolManager}
}

// BuildServer 构建启动依赖容器并解析 HTTP 服务对象。
//
//	@param infras ...*Infrastructure
//	@return *Server
//	@return error
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func BuildServer(infras ...*Infrastructure) (*Server, error) {
	infra := &Infrastructure{}
	if len(infras) > 0 && infras[0] != nil {
		infra = infras[0]
	}
	container := dig.New()
	if err := provide(container, infra); err != nil {
		return nil, err
	}

	var server *Server
	if err := container.Invoke(func(app *fiber.App, humaAPI huma.API) {
		server = &Server{container: container, App: app, HumaAPI: humaAPI}
	}); err != nil {
		return nil, err
	}
	return server, nil
}

func provide(container *dig.Container, infra *Infrastructure) error {
	if err := provideHTTP(container); err != nil {
		return err
	}
	if err := provideInfrastructure(container, infra); err != nil {
		return err
	}
	if err := provideApplication(container); err != nil {
		return err
	}
	if err := provideHandlers(container); err != nil {
		return err
	}
	if err := container.Provide(newAccessTokenSigner, dig.Name(constant.DigNameAccessSigner)); err != nil {
		return err
	}
	if err := container.Provide(newRefreshTokenSigner, dig.Name(constant.DigNameRefreshSigner)); err != nil {
		return err
	}
	return nil
}

func provideHTTP(container *dig.Container) error {
	if err := container.Provide(api.NewFiberApp); err != nil {
		return err
	}
	if err := container.Provide(api.NewHumaAPI); err != nil {
		return err
	}
	return nil
}

func provideInfrastructure(container *dig.Container, infra *Infrastructure) error {
	if err := container.Provide(func() *gorm.DB { return infra.DB }); err != nil {
		return err
	}
	if err := container.Provide(func() *redis.Client { return infra.Cache }); err != nil {
		return err
	}
	if err := container.Provide(func() *pool.PoolManager { return infra.PoolManager }); err != nil {
		return err
	}
	if err := container.Provide(newUserRepository); err != nil {
		return err
	}
	if err := container.Provide(newAPIKeyRepository); err != nil {
		return err
	}
	if err := container.Provide(newSessionReadRepository); err != nil {
		return err
	}
	if err := container.Provide(newAuditRepository); err != nil {
		return err
	}
	if err := container.Provide(newEndpointRepository); err != nil {
		return err
	}
	if err := container.Provide(newModelRepository); err != nil {
		return err
	}
	if err := container.Provide(newEndpointReadRepository); err != nil {
		return err
	}
	if err := container.Provide(newAudioDirCreator); err != nil {
		return err
	}
	if err := container.Provide(newShareCache); err != nil {
		return err
	}
	if err := container.Provide(newSessionDetailCache); err != nil {
		return err
	}
	if err := container.Provide(transport.NewOpenAIProxy); err != nil {
		return err
	}
	if err := container.Provide(transport.NewAnthropicProxy); err != nil {
		return err
	}
	if err := container.Provide(apikeyservice.NewAPIKeyGenerator); err != nil {
		return err
	}
	if err := container.Provide(newOauth2Platforms); err != nil {
		return err
	}
	if err := container.Provide(newStateManager); err != nil {
		return err
	}
	if err := container.Provide(newTaskSubmitter); err != nil {
		return err
	}
	return nil
}

func provideApplication(container *dig.Container) error {
	if err := container.Provide(newEndpointResolver); err != nil {
		return err
	}
	if err := container.Provide(apikeycommand.NewUserExistenceChecker); err != nil {
		return err
	}
	if err := container.Provide(apikeycommand.NewIssueAPIKeyHandler); err != nil {
		return err
	}
	if err := container.Provide(apikeycommand.NewRevokeAPIKeyHandler); err != nil {
		return err
	}
	if err := container.Provide(apikeyquery.NewListAPIKeysHandler); err != nil {
		return err
	}
	if err := container.Provide(endpointcommand.NewCreateEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(endpointcommand.NewUpdateEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(newDeleteEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(endpointquery.NewListEndpointsHandler); err != nil {
		return err
	}
	if err := container.Provide(modelcommand.NewCreateModelHandler); err != nil {
		return err
	}
	if err := container.Provide(modelcommand.NewUpdateModelHandler); err != nil {
		return err
	}
	if err := container.Provide(modelcommand.NewDeleteModelHandler); err != nil {
		return err
	}
	if err := container.Provide(modelquery.NewListModelsHandler); err != nil {
		return err
	}
	if err := container.Provide(newRefreshTokensHandler); err != nil {
		return err
	}
	if err := container.Provide(identitycommand.NewUpdateProfileHandler); err != nil {
		return err
	}
	if err := container.Provide(identityquery.NewGetCurrentUserHandler); err != nil {
		return err
	}
	if err := container.Provide(newInitiateLoginHandler); err != nil {
		return err
	}
	if err := container.Provide(newHandleCallbackHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewListAllAuditLogsHandler); err != nil {
		return err
	}
	if err := container.Provide(newListAuditLogsByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewModelTrendHandler); err != nil {
		return err
	}
	if err := container.Provide(newModelTrendByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewRequestRateHandler); err != nil {
		return err
	}
	if err := container.Provide(newRequestRateByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(newAuditService); err != nil {
		return err
	}
	if err := container.Provide(newListSessionsByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(newGetSessionByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(newGetSessionMetaByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(newListSessionMessagesHandler); err != nil {
		return err
	}
	if err := container.Provide(newListSessionToolsHandler); err != nil {
		return err
	}
	if err := container.Provide(usecase.NewListOpenAIModels); err != nil {
		return err
	}
	if err := container.Provide(usecase.NewListAnthropicModels); err != nil {
		return err
	}
	if err := container.Provide(usecase.NewCountTokens); err != nil {
		return err
	}
	if err := container.Provide(usecase.NewOpenAIUseCase); err != nil {
		return err
	}
	if err := container.Provide(usecase.NewAnthropicUseCase); err != nil {
		return err
	}
	return nil
}

func provideHandlers(container *dig.Container) error {
	if err := container.Provide(newTokenDependencies); err != nil {
		return err
	}
	if err := container.Provide(newOauth2Dependencies); err != nil {
		return err
	}
	if err := container.Provide(newUserDependencies); err != nil {
		return err
	}
	if err := container.Provide(newAPIKeyDependencies); err != nil {
		return err
	}
	if err := container.Provide(newEndpointDependencies); err != nil {
		return err
	}
	if err := container.Provide(newModelDependencies); err != nil {
		return err
	}
	if err := container.Provide(newSessionDependencies); err != nil {
		return err
	}
	if err := container.Provide(newAuditDependencies); err != nil {
		return err
	}
	if err := container.Provide(newOpenAIDependencies); err != nil {
		return err
	}
	if err := container.Provide(newAnthropicDependencies); err != nil {
		return err
	}
	if err := container.Provide(handler.NewPingHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewTokenHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewOauth2Handler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewUserHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewAPIKeyHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewModelHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewSessionHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewAuditHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewOpenAIHandler); err != nil {
		return err
	}
	if err := container.Provide(handler.NewAnthropicHandler); err != nil {
		return err
	}
	return nil
}

func newUserRepository(db *gorm.DB) identity.UserRepository {
	return repository.NewUserRepository(db)
}

func newAPIKeyRepository(db *gorm.DB) apikey.APIKeyRepository {
	return repository.NewAPIKeyRepository(db)
}

func newSessionReadRepository(db *gorm.DB) session.SessionReadRepository {
	return repository.NewSessionReadRepository(db)
}

func newEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return repository.NewEndpointRepository(db)
}

func newModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return repository.NewModelRepository(db)
}

func newEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return repository.NewEndpointReadRepository(db)
}

func newTaskSubmitter() usecase.TaskSubmitter {
	return pool.GetPoolManager()
}

func newAudioDirCreator() applicationoauth2.ObjectStorageDirCreator {
	if config.CosAppID == "" && config.MinioEndpoint == "" {
		return nil
	}
	return repository.NewAudioDirCreator()
}

func newAccessTokenSigner() identityservice.TokenSigner {
	return jwt.GetAccessTokenSigner()
}

func newRefreshTokenSigner() identityservice.TokenSigner {
	return jwt.GetRefreshTokenSigner()
}

func newOauth2Platforms() map[string]oauth2service.Platform {
	return map[string]oauth2service.Platform{
		enum.Oauth2PlatformGithub: infraoauth2.NewGithubPlatform(),
		enum.Oauth2PlatformGoogle: infraoauth2.NewGooglePlatform(),
	}
}

func newStateManager() oauth2service.StateManager {
	return infraoauth2.NewStateManager()
}

func newEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
) llmproxyservice.EndpointResolver {
	return llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo)
}

type refreshTokensParams struct {
	dig.In

	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
}

func newRefreshTokensHandler(params refreshTokensParams) identitycommand.RefreshTokensHandler {
	return identitycommand.NewRefreshTokensHandler(params.UserRepo, params.AccessSigner, params.RefreshSigner)
}

type handleCallbackParams struct {
	dig.In

	Platforms     map[string]oauth2service.Platform
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
	DirCreator    applicationoauth2.ObjectStorageDirCreator
	StateManager  oauth2service.StateManager
}

func newHandleCallbackHandler(params handleCallbackParams) applicationoauth2.HandleCallbackHandler {
	return applicationoauth2.NewHandleCallbackHandler(
		params.Platforms,
		params.UserRepo,
		params.AccessSigner,
		params.RefreshSigner,
		params.DirCreator,
		params.StateManager,
	)
}

func newInitiateLoginHandler(platforms map[string]oauth2service.Platform, stateManager oauth2service.StateManager) applicationoauth2.InitiateLoginHandler {
	return applicationoauth2.NewInitiateLoginHandler(platforms, stateManager)
}

func newTokenDependencies(refresh identitycommand.RefreshTokensHandler) handler.TokenDependencies {
	return handler.TokenDependencies{Refresh: refresh}
}

func newOauth2Dependencies(initiate applicationoauth2.InitiateLoginHandler, callback applicationoauth2.HandleCallbackHandler) handler.Oauth2Dependencies {
	return handler.Oauth2Dependencies{
		Initiate: initiate,
		Callback: callback,
	}
}

func newUserDependencies(getCurrentUser identityquery.GetCurrentUserHandler, updateProfile identitycommand.UpdateProfileHandler) handler.UserDependencies {
	return handler.UserDependencies{
		GetCurrentUser: getCurrentUser,
		UpdateProfile:  updateProfile,
	}
}

func newAPIKeyDependencies(issue apikeycommand.IssueAPIKeyHandler, revoke apikeycommand.RevokeAPIKeyHandler, list apikeyquery.ListAPIKeysHandler) handler.APIKeyDependencies {
	return handler.APIKeyDependencies{
		Issue:  issue,
		Revoke: revoke,
		List:   list,
	}
}

func newSessionDependencies(
	listByUser sessionquery.ListSessionsByUserHandler,
	getByUser sessionquery.GetSessionByUserHandler,
	shareCache cache.ShareCache,
	getMetaByUser sessionquery.GetSessionMetaByUserHandler,
	listMessages sessionquery.ListSessionMessagesHandler,
	listTools sessionquery.ListSessionToolsHandler,
) handler.SessionDependencies {
	return handler.SessionDependencies{
		ListByUser:    listByUser,
		GetByUser:     getByUser,
		ShareCache:    shareCache,
		GetMetaByUser: getMetaByUser,
		ListMessages:  listMessages,
		ListTools:     listTools,
	}
}

func newOpenAIDependencies(useCase usecase.OpenAIUseCase) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{UseCase: useCase}
}

func newAnthropicDependencies(useCase usecase.AnthropicUseCase) handler.AnthropicDependencies {
	return handler.AnthropicDependencies{UseCase: useCase}
}

func newAuditRepository(db *gorm.DB) modelcall.AuditRepository {
	return repository.NewAuditRepository(db)
}

func newAuditDependencies(svc auditquery.AuditService) handler.AuditDependencies {
	return handler.AuditDependencies{Service: svc}
}

func newAuditService(
	listAll auditquery.ListAllAuditLogsHandler,
	listByUser auditquery.ListAuditLogsByUserHandler,
	modelTrend auditquery.ModelTrendHandler,
	modelTrendByUser auditquery.ModelTrendByUserHandler,
	requestRate auditquery.RequestRateHandler,
	requestRateByUser auditquery.RequestRateByUserHandler,
) auditquery.AuditService {
	return auditquery.NewAuditService(listAll, listByUser, modelTrend, modelTrendByUser, requestRate, requestRateByUser)
}

func newListAuditLogsByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ListAuditLogsByUserHandler {
	return auditquery.NewListAuditLogsByUserHandler(repo, apiKeyRepo)
}

func newModelTrendByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ModelTrendByUserHandler {
	return auditquery.NewModelTrendByUserHandler(repo, apiKeyRepo)
}

func newRequestRateByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.RequestRateByUserHandler {
	return auditquery.NewRequestRateByUserHandler(repo, apiKeyRepo)
}

func newEndpointDependencies(create endpointcommand.CreateEndpointHandler, update endpointcommand.UpdateEndpointHandler, delete endpointcommand.DeleteEndpointHandler, list endpointquery.ListEndpointsHandler) handler.EndpointDependencies {
	return handler.EndpointDependencies{Create: create, Update: update, Delete: delete, List: list}
}

func newDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) endpointcommand.DeleteEndpointHandler {
	return endpointcommand.NewDeleteEndpointHandler(endpointRepo, modelRepo)
}

func newModelDependencies(create modelcommand.CreateModelHandler, update modelcommand.UpdateModelHandler, delete modelcommand.DeleteModelHandler, list modelquery.ListModelsHandler) handler.ModelDependencies {
	return handler.ModelDependencies{Create: create, Update: update, Delete: delete, List: list}
}

func newListSessionsByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionquery.ListSessionsByUserHandler {
	return sessionquery.NewListSessionsByUserHandler(readRepo, apiKeyRepo)
}

func newGetSessionByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionquery.GetSessionByUserHandler {
	return sessionquery.NewGetSessionByUserHandler(readRepo, apiKeyRepo)
}

func newShareCache(redisClient *redis.Client) cache.ShareCache {
	return cache.NewShareCache(redisClient)
}

func newSessionDetailCache(redisClient *redis.Client) sessionport.SessionDetailCache {
	return cache.NewSessionDetailCache(redisClient)
}

func newGetSessionMetaByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository, detailCache sessionport.SessionDetailCache) sessionquery.GetSessionMetaByUserHandler {
	return sessionquery.NewGetSessionMetaByUserHandler(readRepo, apiKeyRepo, detailCache)
}

func newListSessionMessagesHandler(readRepo session.SessionReadRepository, metaQuery sessionquery.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionquery.ListSessionMessagesHandler {
	return sessionquery.NewListSessionMessagesHandler(readRepo, metaQuery, detailCache)
}

func newListSessionToolsHandler(readRepo session.SessionReadRepository, metaQuery sessionquery.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionquery.ListSessionToolsHandler {
	return sessionquery.NewListSessionToolsHandler(readRepo, metaQuery, detailCache)
}
