package modules

import (
	"context"
	"time"

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
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/dto"
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
		NewAuditService,
		NewListSessionsByUserHandler,
		NewGetSessionByUserHandler,
		NewGetSessionMetaByUserHandler,
		NewListSessionMessagesHandler,
		NewListSessionToolsHandler,
		NewDeleteSessionHandler,
		NewScoreSessionHandler,
		NewDeleteScoreSessionHandler,
		usecase.NewListOpenAIModels,
		usecase.NewListAnthropicModels,
		usecase.NewCountTokens,
		usecase.NewOpenAIUseCase,
		usecase.NewAnthropicUseCase,
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

func NewDeleteEndpointHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) endpointport.DeleteEndpointHandler {
	return endpointcommand.NewDeleteEndpointHandler(endpointRepo, modelRepo)
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

func NewAuditService(
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
