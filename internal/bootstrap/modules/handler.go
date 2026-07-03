package modules

import (
	apikeyport "github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	blockedport "github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	datasetport "github.com/hcd233/aris-proxy-api/internal/application/dataset/port"
	endpointport "github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	identityport "github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	llmproxyport "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	metricsport "github.com/hcd233/aris-proxy-api/internal/application/metrics/port"
	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	oauthport "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"go.uber.org/fx"
)

var HandlerModule = fx.Module(constant.DigNameHandlerModule,
	fx.Provide(
		NewTokenDependencies,
		NewOauth2Dependencies,
		NewUserDependencies,
		NewAPIKeyDependencies,
		NewEndpointDependencies,
		NewModelDependencies,
		NewSessionDependencies,
		NewAuditDependencies,
		NewCronDependencies,
		NewOpenAIDependencies,
		NewAnthropicDependencies,
		handler.NewPingHandler,
		handler.NewTokenHandler,
		handler.NewOauth2Handler,
		handler.NewUserHandler,
		handler.NewAPIKeyHandler,
		handler.NewEndpointHandler,
		handler.NewModelHandler,
		handler.NewSessionHandler,
		handler.NewAuditHandler,
		handler.NewCronHandler,
		handler.NewOpenAIHandler,
		handler.NewAnthropicHandler,
		NewBlockedDependencies,
		handler.NewBlockedHandler,
		NewMetricsDependencies,
		handler.NewMetricsHandler,
		NewDatasetDependencies,
		handler.NewDatasetHandler,
	),
)

func NewTokenDependencies(refresh identityport.RefreshTokensHandler) handler.TokenDependencies {
	return handler.TokenDependencies{Refresh: refresh}
}

func NewOauth2Dependencies(initiate oauthport.InitiateLoginHandler, callback oauthport.HandleCallbackHandler) handler.Oauth2Dependencies {
	return handler.Oauth2Dependencies{
		Initiate: initiate,
		Callback: callback,
	}
}

func NewUserDependencies(getCurrentUser identityport.GetCurrentUserHandler, updateProfile identityport.UpdateProfileHandler) handler.UserDependencies {
	return handler.UserDependencies{
		GetCurrentUser: getCurrentUser,
		UpdateProfile:  updateProfile,
	}
}

func NewAPIKeyDependencies(issue apikeyport.IssueAPIKeyHandler, revoke apikeyport.RevokeAPIKeyHandler, list apikeyport.ListAPIKeysHandler) handler.APIKeyDependencies {
	return handler.APIKeyDependencies{
		Issue:  issue,
		Revoke: revoke,
		List:   list,
	}
}

func NewSessionDependencies(
	listByUser sessionport.ListSessionsByUserHandler,
	getByUser sessionport.GetSessionByUserHandler,
	shareCache cache.ShareCache,
	createShare sessionport.CreateShareHandler,
	getMetaByUser sessionport.GetSessionMetaByUserHandler,
	listMessages sessionport.ListSessionMessagesHandler,
	listTools sessionport.ListSessionToolsHandler,
	deleteSession sessionport.DeleteSessionHandler,
	scoreSession sessionport.ScoreSessionHandler,
	deleteScoreSession sessionport.DeleteScoreSessionHandler,
	sessionCache sessionport.SessionDetailCache,
	listOption sessionport.ListSessionOptionHandler,
) handler.SessionDependencies {
	return handler.SessionDependencies{
		ListByUser:         listByUser,
		GetByUser:          getByUser,
		ShareCache:         shareCache,
		CreateShare:        createShare,
		GetMetaByUser:      getMetaByUser,
		ListMessages:       listMessages,
		ListTools:          listTools,
		DeleteSession:      deleteSession,
		ScoreSession:       scoreSession,
		DeleteScoreSession: deleteScoreSession,
		SessionCache:       sessionCache,
		ListOption:         listOption,
	}
}

func NewOpenAIDependencies(useCase llmproxyport.OpenAIUseCase, sseGauge *metrics.SSEGauge) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{UseCase: useCase, SSEGauge: sseGauge}
}

func NewAnthropicDependencies(useCase llmproxyport.AnthropicUseCase, sseGauge *metrics.SSEGauge) handler.AnthropicDependencies {
	return handler.AnthropicDependencies{UseCase: useCase, SSEGauge: sseGauge}
}

func NewAuditDependencies(svc auditport.AuditService) handler.AuditDependencies {
	return handler.AuditDependencies{Service: svc}
}

func NewCronDependencies(
	listJobs cronmgmtport.ListCronJobsHandler,
	updateJob cronmgmtport.UpdateCronJobHandler,
	listAudits cronauditport.ListCronCallAuditsHandler,
	listAuditOpts cronauditport.ListCronCallAuditOptionsHandler,
) handler.CronDependencies {
	return handler.CronDependencies{
		ListCronJobs:             listJobs,
		UpdateCronJob:            updateJob,
		ListCronCallAudits:       listAudits,
		ListCronCallAuditOptions: listAuditOpts,
	}
}

func NewEndpointDependencies(create endpointport.CreateEndpointHandler, update endpointport.UpdateEndpointHandler, deleteHandler endpointport.DeleteEndpointHandler, list endpointport.ListEndpointsHandler) handler.EndpointDependencies {
	return handler.EndpointDependencies{Create: create, Update: update, Delete: deleteHandler, List: list}
}

func NewModelDependencies(create modelport.CreateModelHandler, update modelport.UpdateModelHandler, deleteHandler modelport.DeleteModelHandler, list modelport.ListModelsHandler) handler.ModelDependencies {
	return handler.ModelDependencies{Create: create, Update: update, Delete: deleteHandler, List: list}
}

func NewBlockedDependencies(create blockedport.CreateBlockedHandler, del blockedport.DeleteBlockedHandler, list blockedport.ListBlockedHandler) handler.BlockedDependencies {
	return handler.BlockedDependencies{Create: create, Delete: del, List: list}
}

func NewMetricsDependencies(runtimeMetrics metricsport.RuntimeMetricsService) handler.MetricsDependencies {
	return handler.MetricsDependencies{RuntimeMetrics: runtimeMetrics}
}

func NewDatasetDependencies(preview datasetport.PreviewDatasetHandler, export datasetport.ExportDatasetHandler) handler.DatasetDependencies {
	return handler.DatasetDependencies{Preview: preview, Export: export}
}
