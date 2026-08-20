package modules

import (
	"context"
	"fmt"

	apikeycommand "github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	apikeyport "github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	apikeyquery "github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronauditquery "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/query"
	cronmgmtcommand "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/command"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	cronmgmtquery "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/query"
	datasetport "github.com/hcd233/aris-proxy-api/internal/application/dataset/port"
	datasetquery "github.com/hcd233/aris-proxy-api/internal/application/dataset/query"
	democommand "github.com/hcd233/aris-proxy-api/internal/application/demo/command"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	demoquery "github.com/hcd233/aris-proxy-api/internal/application/demo/query"
	endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	endpointport "github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	identitycommand "github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	identityport "github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	identityquery "github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	metricsport "github.com/hcd233/aris-proxy-api/internal/application/metrics/port"
	metricsquery "github.com/hcd233/aris-proxy-api/internal/application/metrics/query"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	modelquery "github.com/hcd233/aris-proxy-api/internal/application/model/query"
	appoauth "github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	oauthport "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessioncommand "github.com/hcd233/aris-proxy-api/internal/application/session/command"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	tracecommand "github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	traceport "github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	tracequery "github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	triggerapp "github.com/hcd233/aris-proxy-api/internal/application/trigger"
	triggercommand "github.com/hcd233/aris-proxy-api/internal/application/trigger/command"
	triggerport "github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	triggerquery "github.com/hcd233/aris-proxy-api/internal/application/trigger/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	cronpkg "github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauthsvc "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ApplicationModule = fx.Module(constant.DigNameApplicationModule,
	fx.Provide(
		apikeycommand.NewUserExistenceChecker,
		NewIssueAPIKeyHandler,
		NewRevokeAPIKeyHandler,
		NewListAPIKeysHandler,
		NewCreateEndpointHandler,
		NewUpdateEndpointHandler,
		NewDeleteEndpointHandler,
		NewListEndpointsHandler,
		NewCreateModelHandler,
		NewUpdateModelHandler,
		NewDeleteModelHandler,
		NewListModelsHandler,
		NewRefreshTokensHandler,
		NewUpdateProfileHandler,
		NewGetCurrentUserHandler,
		NewListUsersHandler,
		NewApproveUserHandler,
		NewDemoteUserHandler,
		NewDeleteUserHandler,
		NewSetDemoUserHandler,
		NewRestoreDemoUserHandler,
		demoquery.NewGetDemoConfigHandler,
		demoquery.NewDemoModuleAccessor,
		demoquery.NewDemoScopeProvider,
		demoquery.NewDemoSessionAccessor,
		democommand.NewUpdateDemoConfigHandler,
		NewDemoLoginHandler,
		NewDemoStatusHandler,
		NewInitiateLoginHandler,
		NewHandleCallbackHandler,
		auditquery.NewListAllAuditLogsHandler,
		NewListAuditLogsByUserHandler,
		auditquery.NewModelTrendHandler,
		NewModelTrendByUserHandler,
		auditquery.NewRequestRateHandler,
		NewRequestRateByUserHandler,
		auditquery.NewTokenThroughputHandler,
		NewTokenThroughputByUserHandler,
		auditquery.NewTokenRateHandler,
		NewTokenRateByUserHandler,
		auditquery.NewModelUsageHandler,
		NewModelUsageByUserHandler,
		auditquery.NewFirstTokenLatencyHandler,
		NewFirstTokenLatencyByUserHandler,
		auditquery.NewListAuditOptionHandler,
		NewAuditService,
		NewListCronJobsHandler,
		NewUpdateCronJobHandler,
		NewTriggerCronJobHandler,
		NewListCronCallAuditsHandler,
		NewListCronCallAuditOptionsHandler,
		NewListSessionsByUserHandler,
		NewGetSessionByUserHandler,
		NewGetSessionMetaByUserHandler,
		NewListSessionMessagesHandler,
		NewListSessionToolsHandler,
		NewDeleteSessionHandler,
		NewScoreSessionHandler,
		NewDeleteScoreSessionHandler,
		NewCreateShareHandler,
		NewSessionOptionHandler,
		usecase.NewListOpenAIModels,
		usecase.NewListAnthropicModels,
		usecase.NewCountTokens,
		usecase.NewOpenAIUseCase,
		usecase.NewAnthropicUseCase,
		NewTriggerService,
		NewTriggerChecker,
		NewTriggerHitRecorder,
		NewCreateTriggerHandler,
		NewUpdateTriggerHandler,
		NewDeleteTriggerHandler,
		NewListTriggerHandler,
		NewRuntimeMetricsHandler,
		NewPreviewDatasetHandler,
		NewExportDatasetHandler,
		NewPreviewFormatDatasetHandler,
		NewTraceRepository,
		NewReportTraceEventHandler,
		NewListTracesHandler,
		NewGetTraceHandler,
		NewListTraceEventsHandler,
		NewDeleteTraceHandler,
	),
)

type refreshTokensParams struct {
	fx.In

	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
}

func NewRefreshTokensHandler(params refreshTokensParams) identityport.RefreshTokensHandler {
	return identitycommand.NewRefreshTokensHandler(params.UserRepo, params.AccessSigner, params.RefreshSigner)
}

type handleCallbackParams struct {
	fx.In

	Platforms     map[string]oauthsvc.Platform
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
	DirCreator    oauthport.ObjectStorageDirCreator
	StateManager  oauthsvc.StateManager
}

func NewHandleCallbackHandler(params handleCallbackParams) oauthport.HandleCallbackHandler {
	return appoauth.NewHandleCallbackHandler(
		params.Platforms,
		params.UserRepo,
		params.AccessSigner,
		params.RefreshSigner,
		params.DirCreator,
		params.StateManager,
	)
}

func NewInitiateLoginHandler(platforms map[string]oauthsvc.Platform, stateManager oauthsvc.StateManager) oauthport.InitiateLoginHandler {
	return appoauth.NewInitiateLoginHandler(platforms, stateManager)
}

func NewIssueAPIKeyHandler(repo apikey.APIKeyRepository, generator apikeyservice.APIKeyGenerator, userExistsCh apikeycommand.UserExistenceChecker) apikeyport.IssueAPIKeyHandler {
	return apikeycommand.NewIssueAPIKeyHandler(repo, generator, userExistsCh)
}

func NewRevokeAPIKeyHandler(repo apikey.APIKeyRepository) apikeyport.RevokeAPIKeyHandler {
	return apikeycommand.NewRevokeAPIKeyHandler(repo)
}

func NewListAPIKeysHandler(repo apikey.APIKeyRepository) apikeyport.ListAPIKeysHandler {
	return apikeyquery.NewListAPIKeysHandler(repo)
}

func NewCreateEndpointHandler(repo llmproxy.EndpointRepository) endpointport.CreateEndpointHandler {
	return endpointcommand.NewCreateEndpointHandler(repo)
}

func NewUpdateEndpointHandler(repo llmproxy.EndpointRepository) endpointport.UpdateEndpointHandler {
	return endpointcommand.NewUpdateEndpointHandler(repo)
}

func NewDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository) endpointport.DeleteEndpointHandler {
	return endpointcommand.NewDeleteEndpointHandler(endpointRepo)
}

func NewListEndpointsHandler(repo llmproxy.EndpointRepository) endpointport.ListEndpointsHandler {
	return endpointquery.NewListEndpointsHandler(repo)
}

func NewCreateModelHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) modelport.CreateModelHandler {
	return modelcommand.NewCreateModelHandler(endpointRepo, modelRepo)
}

func NewUpdateModelHandler(repo llmproxy.ModelRepository) modelport.UpdateModelHandler {
	return modelcommand.NewUpdateModelHandler(repo)
}

func NewDeleteModelHandler(repo llmproxy.ModelRepository) modelport.DeleteModelHandler {
	return modelcommand.NewDeleteModelHandler(repo)
}

func NewListModelsHandler(repo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository) modelport.ListModelsHandler {
	return modelquery.NewListModelsHandler(repo, endpointRepo)
}

func NewUpdateProfileHandler(repo identity.UserRepository) identityport.UpdateProfileHandler {
	return identitycommand.NewUpdateProfileHandler(repo)
}

func NewGetCurrentUserHandler(repo identity.UserRepository) identityport.GetCurrentUserHandler {
	return identityquery.NewGetCurrentUserHandler(repo)
}

func NewListUsersHandler(repo identity.UserRepository) identityport.ListUsersHandler {
	return identityquery.NewListUsersHandler(repo)
}

func NewApproveUserHandler(repo identity.UserRepository, cache *redis.Client) identityport.ApproveUserHandler {
	return identitycommand.NewApproveUserHandler(repo, invalidateJWTUserCache(cache))
}

func NewDemoteUserHandler(repo identity.UserRepository, cache *redis.Client) identityport.DemoteUserHandler {
	return identitycommand.NewDemoteUserHandler(repo, invalidateJWTUserCache(cache))
}

func NewDeleteUserHandler(repo identity.UserRepository, cache *redis.Client) identityport.DeleteUserHandler {
	return identitycommand.NewDeleteUserHandler(repo, invalidateJWTUserCache(cache))
}

func NewSetDemoUserHandler(repo identity.UserRepository, cache *redis.Client) identityport.SetDemoUserHandler {
	return identitycommand.NewSetDemoUserHandler(repo, invalidateJWTUserCache(cache))
}

func NewRestoreDemoUserHandler(repo identity.UserRepository, cache *redis.Client) identityport.RestoreDemoUserHandler {
	return identitycommand.NewRestoreDemoUserHandler(repo, invalidateJWTUserCache(cache))
}

type demoLoginParams struct {
	fx.In

	ConfigRepo    demoport.DemoConfigRepository
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
}

func NewDemoLoginHandler(params demoLoginParams) demoport.DemoLoginHandler {
	return democommand.NewDemoLoginHandler(params.ConfigRepo, params.UserRepo, params.AccessSigner, params.RefreshSigner)
}

func NewDemoStatusHandler(configRepo demoport.DemoConfigRepository, userRepo identity.UserRepository) demoport.DemoStatusHandler {
	return democommand.NewDemoStatusHandler(configRepo, userRepo)
}

// invalidateJWTUserCache 构造删除 Redis jwt:user:{id} 缓存的回调
//
// 用户删除/权限变更后调用，使 JwtMiddleware 的用户缓存立即失效（TTL 内不再以旧状态放行）。
func invalidateJWTUserCache(cache *redis.Client) func(ctx context.Context, userID uint) {
	return func(ctx context.Context, userID uint) {
		if cache == nil {
			return
		}
		key := fmt.Sprintf(constant.JWTUserCacheKeyTemplate, userID)
		if err := cache.Del(ctx, key).Err(); err != nil {
			logger.WithCtx(ctx).Warn("[IdentityCommand] Failed to invalidate jwt user cache",
				zap.Error(err), zap.Uint("userID", userID), zap.String("key", key))
		}
	}
}

func NewListAuditLogsByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ListAuditLogsByUserHandler {
	return auditquery.NewListAuditLogsByUserHandler(repo, apiKeyRepo)
}

func NewModelTrendByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ModelTrendByUserHandler {
	return auditquery.NewModelTrendByUserHandler(repo, apiKeyRepo)
}

func NewRequestRateByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.RequestRateByUserHandler {
	return auditquery.NewRequestRateByUserHandler(repo, apiKeyRepo)
}

func NewTokenThroughputByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.TokenThroughputByUserHandler {
	return auditquery.NewTokenThroughputByUserHandler(repo, apiKeyRepo)
}

func NewTokenRateByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.TokenRateByUserHandler {
	return auditquery.NewTokenRateByUserHandler(repo, apiKeyRepo)
}

func NewModelUsageByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ModelUsageByUserHandler {
	return auditquery.NewModelUsageByUserHandler(repo, apiKeyRepo)
}

func NewFirstTokenLatencyByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.FirstTokenLatencyByUserHandler {
	return auditquery.NewFirstTokenLatencyByUserHandler(repo, apiKeyRepo)
}

func NewAuditService(
	listAll auditquery.ListAllAuditLogsHandler,
	listByUser auditquery.ListAuditLogsByUserHandler,
	listAuditOption auditquery.ListAuditOptionHandler,
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
	demoScope demoport.DemoScopeProvider,
) auditport.AuditService {
	return auditquery.NewAuditService(listAll, listByUser, listAuditOption, modelTrend, modelTrendByUser, requestRate, requestRateByUser, tokenThroughput, tokenThroughputByUser, tokenRate, tokenRateByUser, modelUsage, modelUsageByUser, firstTokenLatency, firstTokenLatencyByUser, demoScope)
}

func NewListSessionsByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.ListSessionsByUserHandler {
	return sessionquery.NewListSessionsByUserHandler(readRepo, apiKeyRepo)
}

func NewGetSessionByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.GetSessionByUserHandler {
	return sessionquery.NewGetSessionByUserHandler(readRepo, apiKeyRepo)
}

func NewGetSessionMetaByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository, detailCache sessionport.SessionDetailCache) sessionport.GetSessionMetaByUserHandler {
	return sessionquery.NewGetSessionMetaByUserHandler(readRepo, apiKeyRepo, detailCache)
}

func NewListSessionMessagesHandler(readRepo session.SessionReadRepository, metaQuery sessionport.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionport.ListSessionMessagesHandler {
	return sessionquery.NewListSessionMessagesHandler(readRepo, metaQuery, detailCache)
}

func NewListSessionToolsHandler(readRepo session.SessionReadRepository, metaQuery sessionport.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionport.ListSessionToolsHandler {
	return sessionquery.NewListSessionToolsHandler(readRepo, metaQuery, detailCache)
}

func NewDeleteSessionHandler(sessionRepo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.DeleteSessionHandler {
	return sessioncommand.NewDeleteSessionHandler(sessionRepo, apiKeyRepo)
}

func NewScoreSessionHandler(sessionRepo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.ScoreSessionHandler {
	return sessioncommand.NewScoreSessionHandler(sessionRepo, apiKeyRepo)
}

func NewDeleteScoreSessionHandler(sessionRepo session.SessionRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.DeleteScoreSessionHandler {
	return sessioncommand.NewDeleteScoreSessionHandler(sessionRepo, apiKeyRepo)
}

func NewCreateShareHandler(getByUser sessionport.GetSessionByUserHandler, shareCache sessionport.ShareCreator) sessionport.CreateShareHandler {
	return sessioncommand.NewCreateShareHandler(getByUser, shareCache)
}

func NewSessionOptionHandler(readRepo session.SessionReadRepository) sessionport.ListSessionOptionHandler {
	return sessionquery.NewListSessionOptionHandler(readRepo)
}

func NewTriggerService(repo triggerdomain.TriggerRepository, hitRecorder triggerport.HitRecorder, cache *redis.Client) *triggerapp.TriggerService {
	return triggerapp.NewTriggerService(repo, hitRecorder, cache)
}

func NewTriggerChecker(svc *triggerapp.TriggerService) usecase.TriggerChecker {
	return svc
}

func NewTriggerHitRecorder(triggerCache *cache.TriggerHitCache) triggerport.HitRecorder {
	return triggerCache
}

func NewCreateTriggerHandler(repo triggerdomain.TriggerRepository, svc *triggerapp.TriggerService) triggerport.CreateTriggerHandler {
	return triggercommand.NewCreateTriggerHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) }, //nolint:errcheck // 本进程立即生效路径，失败由 A/B/C 通道重试
		svc.NotifyChanged,
	)
}

func NewUpdateTriggerHandler(repo triggerdomain.TriggerRepository, svc *triggerapp.TriggerService) triggerport.UpdateTriggerHandler {
	return triggercommand.NewUpdateTriggerHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) }, //nolint:errcheck // 本进程立即生效路径，失败由 A/B/C 通道重试
		svc.NotifyChanged,
	)
}

func NewDeleteTriggerHandler(repo triggerdomain.TriggerRepository, svc *triggerapp.TriggerService) triggerport.DeleteTriggerHandler {
	return triggercommand.NewDeleteTriggerHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) }, //nolint:errcheck // 本进程立即生效路径，失败由 A/B/C 通道重试
		svc.NotifyChanged,
	)
}

func NewListTriggerHandler(repo triggerdomain.TriggerRepository) triggerport.ListTriggerHandler {
	return triggerquery.NewListTriggerHandler(repo)
}

func NewRuntimeMetricsHandler(runtimeCache *cache.RuntimeMetricsCache) metricsport.RuntimeMetricsService {
	return metricsquery.NewRuntimeMetricsHandler(runtimeCache)
}

func NewPreviewDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.PreviewDatasetHandler {
	return datasetquery.NewPreviewDatasetHandler(readRepo, apiKeyRepo)
}

func NewExportDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.ExportDatasetHandler {
	return datasetquery.NewExportDatasetHandler(readRepo, apiKeyRepo)
}

func NewPreviewFormatDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.PreviewFormatDatasetHandler {
	return datasetquery.NewPreviewFormatDatasetHandler(readRepo, apiKeyRepo)
}

func NewListCronJobsHandler(repo cronmgmtport.CronJobRepository) cronmgmtport.ListCronJobsHandler {
	return cronmgmtquery.NewListCronJobsHandler(repo)
}

func NewUpdateCronJobHandler(repo cronmgmtport.CronJobRepository, manager *cronpkg.CronManager) cronmgmtport.UpdateCronJobHandler {
	return cronmgmtcommand.NewUpdateCronJobHandler(repo, manager)
}

func NewTriggerCronJobHandler(manager *cronpkg.CronManager) cronmgmtport.TriggerCronJobHandler {
	return cronmgmtcommand.NewTriggerCronJobHandler(manager)
}

func NewListCronCallAuditsHandler(repo cronauditport.CronCallAuditRepository) cronauditport.ListCronCallAuditsHandler {
	return cronauditquery.NewListCronCallAuditsHandler(repo)
}

func NewListCronCallAuditOptionsHandler(repo cronauditport.CronCallAuditRepository) cronauditport.ListCronCallAuditOptionsHandler {
	return cronauditquery.NewListCronCallAuditOptionsHandler(repo)
}

func NewTraceRepository(db *gorm.DB) trace.TraceRepository {
	return repository.NewTraceRepository(db)
}

func NewReportTraceEventHandler(repo trace.TraceRepository) traceport.ReportTraceEventHandler {
	return tracecommand.NewReportTraceEventHandler(repo)
}

func NewListTracesHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) traceport.ListTracesHandler {
	return tracequery.NewListTracesHandler(repo, apiKeyRepo)
}

func NewGetTraceHandler(
	repo trace.TraceRepository,
	apiKeyRepo apikey.APIKeyRepository,
) traceport.GetTraceHandler {
	return tracequery.NewGetTraceHandler(repo, apiKeyRepo)
}

func NewListTraceEventsHandler(
	repo trace.TraceRepository,
	apiKeyRepo apikey.APIKeyRepository,
) traceport.ListTraceEventsHandler {
	return tracequery.NewListTraceEventsHandler(repo, apiKeyRepo)
}

func NewDeleteTraceHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) traceport.DeleteTraceHandler {
	return tracecommand.NewDeleteTraceHandler(repo, apiKeyRepo)
}
