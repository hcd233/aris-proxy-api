package bootstrap

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/api"
	apikeycommand "github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	apikeyport "github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	apikeyquery "github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	endpointport "github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	identitycommand "github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	identityport "github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	identityquery "github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	modelquery "github.com/hcd233/aris-proxy-api/internal/application/model/query"
	applicationoauth2 "github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	oauth2port "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessioncommand "github.com/hcd233/aris-proxy-api/internal/application/session/command"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
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
	"github.com/hcd233/aris-proxy-api/internal/dto"
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
	thinkExtractRepo := repository.NewThinkExtractRepository(db)
	cron.InitCronJobs(context.Background(), db, poolManager, cache, thinkExtractRepo)
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
	if err := container.Provide(newSessionWriteRepository); err != nil {
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
	if err := container.Provide(newIssueAPIKeyHandler); err != nil {
		return err
	}
	if err := container.Provide(newRevokeAPIKeyHandler); err != nil {
		return err
	}
	if err := container.Provide(newListAPIKeysHandler); err != nil {
		return err
	}
	if err := container.Provide(newCreateEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(newUpdateEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(newDeleteEndpointHandler); err != nil {
		return err
	}
	if err := container.Provide(newListEndpointsHandler); err != nil {
		return err
	}
	if err := container.Provide(newCreateModelHandler); err != nil {
		return err
	}
	if err := container.Provide(newUpdateModelHandler); err != nil {
		return err
	}
	if err := container.Provide(newDeleteModelHandler); err != nil {
		return err
	}
	if err := container.Provide(newListModelsHandler); err != nil {
		return err
	}
	if err := container.Provide(newRefreshTokensHandler); err != nil {
		return err
	}
	if err := container.Provide(newUpdateProfileHandler); err != nil {
		return err
	}
	if err := container.Provide(newGetCurrentUserHandler); err != nil {
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
	if err := container.Provide(auditquery.NewTokenThroughputHandler); err != nil {
		return err
	}
	if err := container.Provide(newTokenThroughputByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewTokenRateHandler); err != nil {
		return err
	}
	if err := container.Provide(newTokenRateByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewModelUsageHandler); err != nil {
		return err
	}
	if err := container.Provide(newModelUsageByUserHandler); err != nil {
		return err
	}
	if err := container.Provide(auditquery.NewFirstTokenLatencyHandler); err != nil {
		return err
	}
	if err := container.Provide(newFirstTokenLatencyByUserHandler); err != nil {
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
	if err := container.Provide(newDeleteSessionHandler); err != nil {
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

func newAudioDirCreator() oauth2port.ObjectStorageDirCreator {
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

func newRefreshTokensHandler(params refreshTokensParams) identityport.RefreshTokensHandler {
	return identitycommand.NewRefreshTokensHandler(params.UserRepo, params.AccessSigner, params.RefreshSigner)
}

type handleCallbackParams struct {
	dig.In

	Platforms     map[string]oauth2service.Platform
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
	DirCreator    oauth2port.ObjectStorageDirCreator
	StateManager  oauth2service.StateManager
}

func newHandleCallbackHandler(params handleCallbackParams) oauth2port.HandleCallbackHandler {
	return applicationoauth2.NewHandleCallbackHandler(
		params.Platforms,
		params.UserRepo,
		params.AccessSigner,
		params.RefreshSigner,
		params.DirCreator,
		params.StateManager,
	)
}

func newInitiateLoginHandler(platforms map[string]oauth2service.Platform, stateManager oauth2service.StateManager) oauth2port.InitiateLoginHandler {
	return applicationoauth2.NewInitiateLoginHandler(platforms, stateManager)
}

func newTokenDependencies(refresh identityport.RefreshTokensHandler) handler.TokenDependencies {
	return handler.TokenDependencies{Refresh: refresh}
}

func newOauth2Dependencies(initiate oauth2port.InitiateLoginHandler, callback oauth2port.HandleCallbackHandler) handler.Oauth2Dependencies {
	return handler.Oauth2Dependencies{
		Initiate: initiate,
		Callback: callback,
	}
}

func newUserDependencies(getCurrentUser identityport.GetCurrentUserHandler, updateProfile identityport.UpdateProfileHandler) handler.UserDependencies {
	return handler.UserDependencies{
		GetCurrentUser: getCurrentUser,
		UpdateProfile:  updateProfile,
	}
}

func newAPIKeyDependencies(issue apikeyport.IssueAPIKeyHandler, revoke apikeyport.RevokeAPIKeyHandler, list apikeyport.ListAPIKeysHandler) handler.APIKeyDependencies {
	return handler.APIKeyDependencies{
		Issue:  issue,
		Revoke: revoke,
		List:   list,
	}
}

func newSessionDependencies(
	listByUser sessionport.ListSessionsByUserHandler,
	getByUser sessionport.GetSessionByUserHandler,
	shareCache cache.ShareCache,
	getMetaByUser sessionport.GetSessionMetaByUserHandler,
	listMessages sessionport.ListSessionMessagesHandler,
	listTools sessionport.ListSessionToolsHandler,
	deleteSession sessionport.DeleteSessionHandler,
	sessionRepo session.SessionRepository,
	sessionCache sessionport.SessionDetailCache,
) handler.SessionDependencies {
	return handler.SessionDependencies{
		ListByUser:    listByUser,
		GetByUser:     getByUser,
		ShareCache:    shareCache,
		GetMetaByUser: getMetaByUser,
		ListMessages:  listMessages,
		ListTools:     listTools,
		DeleteSession: deleteSession,
		SessionRepo:   sessionRepo,
		SessionCache:  sessionCache,
	}
}

func newOpenAIDependencies(useCase usecase.OpenAIUseCase) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{UseCase: &openAIUseCaseAdapter{inner: useCase}}
}

func newAnthropicDependencies(useCase usecase.AnthropicUseCase) handler.AnthropicDependencies {
	return handler.AnthropicDependencies{UseCase: &anthropicUseCaseAdapter{inner: useCase}}
}

type openAIUseCaseAdapter struct {
	inner usecase.OpenAIUseCase
}

func (a *openAIUseCaseAdapter) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return a.inner.ListModels(ctx)
}
func (a *openAIUseCaseAdapter) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	return a.inner.CreateChatCompletion(ctx, req)
}
func (a *openAIUseCaseAdapter) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	return a.inner.CreateResponse(ctx, req)
}

type anthropicUseCaseAdapter struct {
	inner usecase.AnthropicUseCase
}

func (a *anthropicUseCaseAdapter) ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	return a.inner.ListModels(ctx)
}
func (a *anthropicUseCaseAdapter) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	return a.inner.CreateMessage(ctx, req)
}
func (a *anthropicUseCaseAdapter) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return a.inner.CountTokens(ctx, req)
}

type auditServiceAdapter struct {
	inner auditquery.AuditService
}

func (a *auditServiceAdapter) ListLogs(ctx context.Context, permission enum.Permission, userID uint, q auditport.ListAuditLogsParams) ([]*auditport.AuditLogView, *model.PageInfo, error) {
	views, pageInfo, err := a.inner.ListLogs(ctx, permission, userID, auditquery.ListAuditLogsParams(q))
	if err != nil {
		return nil, nil, err
	}
	result := make([]*auditport.AuditLogView, len(views))
	for i, v := range views {
		result[i] = &auditport.AuditLogView{
			ID:                       v.ID,
			CreatedAt:                v.CreatedAt,
			Model:                    v.Model,
			UpstreamProtocol:         v.UpstreamProtocol,
			APIProtocol:              v.APIProtocol,
			Endpoint:                 v.Endpoint,
			InputTokens:              v.InputTokens,
			OutputTokens:             v.OutputTokens,
			CacheCreationInputTokens: v.CacheCreationInputTokens,
			CacheReadInputTokens:     v.CacheReadInputTokens,
			FirstTokenLatencyMs:      v.FirstTokenLatencyMs,
			StreamDurationMs:         v.StreamDurationMs,
			UserAgent:                v.UserAgent,
			UpstreamStatusCode:       v.UpstreamStatusCode,
			ErrorMessage:             v.ErrorMessage,
			TraceID:                  v.TraceID,
			APIKeyName:               v.APIKeyName,
			UserName:                 v.UserName,
			UserEmail:                v.UserEmail,
		}
	}
	return result, pageInfo, nil
}
func (a *auditServiceAdapter) ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	return a.inner.ModelTrend(ctx, permission, userID, startTime, endTime, granularity)
}
func (a *auditServiceAdapter) RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error) {
	return a.inner.RequestRate(ctx, permission, userID, startTime, endTime, granularity)
}
func (a *auditServiceAdapter) TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	return a.inner.TokenThroughput(ctx, permission, userID, startTime, endTime, granularity)
}
func (a *auditServiceAdapter) TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error) {
	return a.inner.TokenRate(ctx, permission, userID, startTime, endTime, granularity)
}
func (a *auditServiceAdapter) ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error) {
	return a.inner.ModelUsage(ctx, permission, userID, startTime, endTime, granularity)
}
func (a *auditServiceAdapter) FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error) {
	return a.inner.FirstTokenLatency(ctx, permission, userID, startTime, endTime, granularity)
}

func newAuditRepository(db *gorm.DB) modelcall.AuditRepository {
	return repository.NewAuditRepository(db)
}

func newAuditDependencies(svc auditport.AuditService) handler.AuditDependencies {
	return handler.AuditDependencies{Service: svc}
}

func newAuditService(
	listAll auditquery.ListAllAuditLogsHandler,
	listByUser auditquery.ListAuditLogsByUserHandler,
	modelTrend auditquery.ModelTrendHandler,
	modelTrendByUser auditquery.ModelTrendByUserHandler,
	requestRate auditquery.RequestRateHandler,
	requestRateByUser auditquery.RequestRateByUserHandler,
	tokenThroughput auditquery.TokenThroughputHandler,
	tokenThroughputByUser auditquery.TokenThroughputByUserHandler,
	tokenRate auditquery.TokenRateHandler,
	tokenRateByUser auditquery.TokenRateByUserHandler,
	modelUsage auditquery.ModelUsageHandler,
	modelUsageByUser auditquery.ModelUsageByUserHandler,
	firstTokenLatency auditquery.FirstTokenLatencyHandler,
	firstTokenLatencyByUser auditquery.FirstTokenLatencyByUserHandler,
) auditport.AuditService {
	return &auditServiceAdapter{inner: auditquery.NewAuditService(listAll, listByUser, modelTrend, modelTrendByUser, requestRate, requestRateByUser, tokenThroughput, tokenThroughputByUser, tokenRate, tokenRateByUser, modelUsage, modelUsageByUser, firstTokenLatency, firstTokenLatencyByUser)}
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

func newTokenThroughputByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.TokenThroughputByUserHandler {
	return auditquery.NewTokenThroughputByUserHandler(repo, apiKeyRepo)
}

func newTokenRateByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.TokenRateByUserHandler {
	return auditquery.NewTokenRateByUserHandler(repo, apiKeyRepo)
}

func newModelUsageByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ModelUsageByUserHandler {
	return auditquery.NewModelUsageByUserHandler(repo, apiKeyRepo)
}

func newFirstTokenLatencyByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.FirstTokenLatencyByUserHandler {
	return auditquery.NewFirstTokenLatencyByUserHandler(repo, apiKeyRepo)
}

func newEndpointDependencies(create endpointport.CreateEndpointHandler, update endpointport.UpdateEndpointHandler, delete endpointport.DeleteEndpointHandler, list endpointport.ListEndpointsHandler) handler.EndpointDependencies {
	return handler.EndpointDependencies{Create: create, Update: update, Delete: delete, List: list}
}

func newDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) endpointport.DeleteEndpointHandler {
	return endpointcommand.NewDeleteEndpointHandler(endpointRepo, modelRepo)
}

func newCreateEndpointHandler(repo llmproxy.EndpointRepository) endpointport.CreateEndpointHandler {
	return endpointcommand.NewCreateEndpointHandler(repo)
}

func newUpdateEndpointHandler(repo llmproxy.EndpointRepository) endpointport.UpdateEndpointHandler {
	return endpointcommand.NewUpdateEndpointHandler(repo)
}

func newIssueAPIKeyHandler(repo apikey.APIKeyRepository, generator apikeyservice.APIKeyGenerator, userExistsCh apikeycommand.UserExistenceChecker) apikeyport.IssueAPIKeyHandler {
	return apikeycommand.NewIssueAPIKeyHandler(repo, generator, userExistsCh)
}

func newRevokeAPIKeyHandler(repo apikey.APIKeyRepository) apikeyport.RevokeAPIKeyHandler {
	return apikeycommand.NewRevokeAPIKeyHandler(repo)
}

func newListAPIKeysHandler(repo apikey.APIKeyRepository) apikeyport.ListAPIKeysHandler {
	return apikeyquery.NewListAPIKeysHandler(repo)
}

func newCreateModelHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) modelport.CreateModelHandler {
	return modelcommand.NewCreateModelHandler(endpointRepo, modelRepo)
}

func newUpdateModelHandler(repo llmproxy.ModelRepository) modelport.UpdateModelHandler {
	return modelcommand.NewUpdateModelHandler(repo)
}

func newDeleteModelHandler(repo llmproxy.ModelRepository) modelport.DeleteModelHandler {
	return modelcommand.NewDeleteModelHandler(repo)
}

func newUpdateProfileHandler(repo identity.UserRepository) identityport.UpdateProfileHandler {
	return identitycommand.NewUpdateProfileHandler(repo)
}

func newGetCurrentUserHandler(repo identity.UserRepository) identityport.GetCurrentUserHandler {
	return identityquery.NewGetCurrentUserHandler(repo)
}

func newListEndpointsHandler(repo llmproxy.EndpointRepository) endpointport.ListEndpointsHandler {
	return endpointquery.NewListEndpointsHandler(repo)
}

func newListModelsHandler(repo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository) modelport.ListModelsHandler {
	return modelquery.NewListModelsHandler(repo, endpointRepo)
}

func newModelDependencies(create modelport.CreateModelHandler, update modelport.UpdateModelHandler, delete modelport.DeleteModelHandler, list modelport.ListModelsHandler) handler.ModelDependencies {
	return handler.ModelDependencies{Create: create, Update: update, Delete: delete, List: list}
}

func newListSessionsByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.ListSessionsByUserHandler {
	return sessionquery.NewListSessionsByUserHandler(readRepo, apiKeyRepo)
}

func newGetSessionByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.GetSessionByUserHandler {
	return sessionquery.NewGetSessionByUserHandler(readRepo, apiKeyRepo)
}

func newShareCache(redisClient *redis.Client) cache.ShareCache {
	return cache.NewShareCache(redisClient)
}

func newSessionDetailCache(redisClient *redis.Client) sessionport.SessionDetailCache {
	return cache.NewSessionDetailCache(redisClient)
}

func newGetSessionMetaByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository, detailCache sessionport.SessionDetailCache) sessionport.GetSessionMetaByUserHandler {
	return sessionquery.NewGetSessionMetaByUserHandler(readRepo, apiKeyRepo, detailCache)
}

func newListSessionMessagesHandler(readRepo session.SessionReadRepository, metaQuery sessionport.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionport.ListSessionMessagesHandler {
	return sessionquery.NewListSessionMessagesHandler(readRepo, metaQuery, detailCache)
}

func newListSessionToolsHandler(readRepo session.SessionReadRepository, metaQuery sessionport.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionport.ListSessionToolsHandler {
	return sessionquery.NewListSessionToolsHandler(readRepo, metaQuery, detailCache)
}

func newSessionWriteRepository(db *gorm.DB) session.SessionRepository {
	return repository.NewSessionRepository(db)
}

func newDeleteSessionHandler(sessionRepo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.DeleteSessionHandler {
	return sessioncommand.NewDeleteSessionHandler(sessionRepo, apiKeyRepo)
}
