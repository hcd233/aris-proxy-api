package modules

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	apikeyport "github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	endpointport "github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	identityport "github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	oauth2port "github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"go.uber.org/fx"
)

var HandlerModule = fx.Module("handler",
	fx.Provide(
		NewTokenDependencies,
		NewOauth2Dependencies,
		NewUserDependencies,
		NewAPIKeyDependencies,
		NewEndpointDependencies,
		NewModelDependencies,
		NewSessionDependencies,
		NewAuditDependencies,
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
		handler.NewOpenAIHandler,
		handler.NewAnthropicHandler,
	),
)

func NewTokenDependencies(refresh identityport.RefreshTokensHandler) handler.TokenDependencies {
	return handler.TokenDependencies{Refresh: refresh}
}

func NewOauth2Dependencies(initiate oauth2port.InitiateLoginHandler, callback oauth2port.HandleCallbackHandler) handler.Oauth2Dependencies {
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
	getMetaByUser sessionport.GetSessionMetaByUserHandler,
	listMessages sessionport.ListSessionMessagesHandler,
	listTools sessionport.ListSessionToolsHandler,
	deleteSession sessionport.DeleteSessionHandler,
	scoreSession sessionport.ScoreSessionHandler,
	deleteScoreSession sessionport.DeleteScoreSessionHandler,
	sessionCache sessionport.SessionDetailCache,
) handler.SessionDependencies {
	return handler.SessionDependencies{
		ListByUser:         listByUser,
		GetByUser:          getByUser,
		ShareCache:         shareCache,
		GetMetaByUser:      getMetaByUser,
		ListMessages:       listMessages,
		ListTools:          listTools,
		DeleteSession:      deleteSession,
		ScoreSession:       scoreSession,
		DeleteScoreSession: deleteScoreSession,
		SessionCache:       sessionCache,
	}
}

func NewOpenAIDependencies(useCase usecase.OpenAIUseCase) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{UseCase: &openAIUseCaseAdapter{inner: useCase}}
}

func NewAnthropicDependencies(useCase usecase.AnthropicUseCase) handler.AnthropicDependencies {
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

func NewAuditDependencies(svc auditport.AuditService) handler.AuditDependencies {
	return handler.AuditDependencies{Service: svc}
}

func NewEndpointDependencies(create endpointport.CreateEndpointHandler, update endpointport.UpdateEndpointHandler, deleteHandler endpointport.DeleteEndpointHandler, list endpointport.ListEndpointsHandler) handler.EndpointDependencies {
	return handler.EndpointDependencies{Create: create, Update: update, Delete: deleteHandler, List: list}
}

func NewModelDependencies(create modelport.CreateModelHandler, update modelport.UpdateModelHandler, deleteHandler modelport.DeleteModelHandler, list modelport.ListModelsHandler) handler.ModelDependencies {
	return handler.ModelDependencies{Create: create, Update: update, Delete: deleteHandler, List: list}
}
