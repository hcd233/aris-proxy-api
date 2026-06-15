package modules

import (
	apikeycommand "github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	apikeyport "github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	apikeyquery "github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"
	blockedcommand "github.com/hcd233/aris-proxy-api/internal/application/blocked/command"
	blockedport "github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	blockedquery "github.com/hcd233/aris-proxy-api/internal/application/blocked/query"
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
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"go.uber.org/fx"
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
		NewListSessionsByUserHandler,
		NewGetSessionByUserHandler,
		NewGetSessionMetaByUserHandler,
		NewListSessionMessagesHandler,
		NewListSessionToolsHandler,
		NewDeleteSessionHandler,
		NewScoreSessionHandler,
		NewDeleteScoreSessionHandler,
		NewSessionOptionHandler,
		usecase.NewListOpenAIModels,
		usecase.NewListAnthropicModels,
		usecase.NewCountTokens,
		usecase.NewOpenAIUseCase,
		usecase.NewAnthropicUseCase,
		NewBlockedService,
		NewBlockedChecker,
		NewBlockedHitRecorder,
		NewCreateBlockedHandler,
		NewDeleteBlockedHandler,
		NewListBlockedHandler,
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

	Platforms     map[string]oauth2service.Platform
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
	DirCreator    oauth2port.ObjectStorageDirCreator
	StateManager  oauth2service.StateManager
}

func NewHandleCallbackHandler(params handleCallbackParams) oauth2port.HandleCallbackHandler {
	return applicationoauth2.NewHandleCallbackHandler(
		params.Platforms,
		params.UserRepo,
		params.AccessSigner,
		params.RefreshSigner,
		params.DirCreator,
		params.StateManager,
	)
}

func NewInitiateLoginHandler(platforms map[string]oauth2service.Platform, stateManager oauth2service.StateManager) oauth2port.InitiateLoginHandler {
	return applicationoauth2.NewInitiateLoginHandler(platforms, stateManager)
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
) auditport.AuditService {
	return auditquery.NewAuditService(listAll, listByUser, listAuditOption, modelTrend, modelTrendByUser, requestRate, requestRateByUser, tokenThroughput, tokenThroughputByUser, tokenRate, tokenRateByUser, modelUsage, modelUsageByUser, firstTokenLatency, firstTokenLatencyByUser)
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

func NewSessionOptionHandler(readRepo session.SessionReadRepository) sessionport.ListSessionOptionHandler {
	return sessionquery.NewListSessionOptionHandler(readRepo)
}

func NewBlockedService(repo blockeddomain.BlockedRepository, hitRecorder blockedport.HitRecorder) *blockedapp.BlockedService {
	return blockedapp.NewBlockedService(repo, hitRecorder)
}

func NewBlockedChecker(svc *blockedapp.BlockedService) usecase.BlockedChecker {
	return svc
}

func NewBlockedHitRecorder(blockedCache *cache.BlockedHitCache) blockedport.HitRecorder {
	return blockedCache
}

func NewCreateBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.CreateBlockedHandler {
	return blockedcommand.NewCreateBlockedHandler(repo, svc.Rebuild)
}

func NewDeleteBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.DeleteBlockedHandler {
	return blockedcommand.NewDeleteBlockedHandler(repo, svc.Rebuild)
}

func NewListBlockedHandler(repo blockeddomain.BlockedRepository) blockedport.ListBlockedHandler {
	return blockedquery.NewListBlockedHandler(repo)
}
