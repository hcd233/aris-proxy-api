package bootstrap

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/api"
	applicationoauth2 "github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"go.uber.org/dig"
)

// Server 启动阶段解析出的 HTTP 服务对象。
//
//	@author centonhuang
//	@update 2026-04-28 10:00:00
type Server struct {
	Container *dig.Container
	App       *fiber.App
	HumaAPI   huma.API
}

// BuildServer 构建启动依赖容器并解析 HTTP 服务对象。
//
//	@return *Server
//	@return error
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func BuildServer() (*Server, error) {
	container := dig.New()
	if err := provide(container); err != nil {
		return nil, err
	}

	var server *Server
	if err := container.Invoke(func(app *fiber.App, humaAPI huma.API) {
		api.SetFiberApp(app)
		api.SetHumaAPI(humaAPI)
		server = &Server{Container: container, App: app, HumaAPI: humaAPI}
	}); err != nil {
		return nil, err
	}
	return server, nil
}

func provide(container *dig.Container) error {
	providers := []any{
		api.NewFiberApp,
		api.NewHumaAPI,
		newUserRepository,
		newAPIKeyRepository,
		newSessionReadRepository,
		newEndpointRepository,
		newEndpointReadRepository,
		newAudioDirCreator,
		transport.NewOpenAIProxy,
		transport.NewAnthropicProxy,
		apikeyservice.NewAPIKeyGenerator,
		newOauth2Platforms,
		newTokenDependencies,
		newOauth2Dependencies,
		newUserDependencies,
		newAPIKeyDependencies,
		newSessionDependencies,
		newOpenAIDependencies,
		newAnthropicDependencies,
		handler.NewPingHandler,
		handler.NewTokenHandler,
		handler.NewOauth2Handler,
		handler.NewUserHandler,
		handler.NewAPIKeyHandler,
		handler.NewSessionHandler,
		handler.NewOpenAIHandler,
		handler.NewAnthropicHandler,
	}
	for _, provider := range providers {
		if err := container.Provide(provider); err != nil {
			return err
		}
	}
	if err := container.Provide(newAccessTokenSigner, dig.Name("accessSigner")); err != nil {
		return err
	}
	if err := container.Provide(newRefreshTokenSigner, dig.Name("refreshSigner")); err != nil {
		return err
	}
	return nil
}

func newUserRepository() identity.UserRepository {
	return repository.NewUserRepository()
}

func newAPIKeyRepository() apikey.APIKeyRepository {
	return repository.NewAPIKeyRepository()
}

func newSessionReadRepository() session.SessionReadRepository {
	return repository.NewSessionReadRepository()
}

func newEndpointRepository() llmproxy.EndpointRepository {
	return repository.NewEndpointRepository()
}

func newEndpointReadRepository() llmproxy.EndpointReadRepository {
	return repository.NewEndpointReadRepository()
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

func newOauth2Platforms() handler.Oauth2Platforms {
	return handler.Oauth2Platforms{
		constant.OAuthProviderGithub: oauth2.NewGithubPlatform(),
		constant.OAuthProviderGoogle: oauth2.NewGooglePlatform(),
	}
}

type tokenDependencyParams struct {
	dig.In

	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
}

func newTokenDependencies(params tokenDependencyParams) handler.TokenDependencies {
	return handler.TokenDependencies{
		UserRepo:      params.UserRepo,
		AccessSigner:  params.AccessSigner,
		RefreshSigner: params.RefreshSigner,
	}
}

type oauth2DependencyParams struct {
	dig.In

	Platforms     handler.Oauth2Platforms
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
	DirCreator    applicationoauth2.ObjectStorageDirCreator
}

func newOauth2Dependencies(params oauth2DependencyParams) handler.Oauth2Dependencies {
	return handler.Oauth2Dependencies{
		Platforms:     params.Platforms,
		UserRepo:      params.UserRepo,
		AccessSigner:  params.AccessSigner,
		RefreshSigner: params.RefreshSigner,
		DirCreator:    params.DirCreator,
	}
}

func newUserDependencies(userRepo identity.UserRepository) handler.UserDependencies {
	return handler.UserDependencies{UserRepo: userRepo}
}

func newAPIKeyDependencies(apiKeyRepo apikey.APIKeyRepository, userRepo identity.UserRepository, generator apikeyservice.APIKeyGenerator) handler.APIKeyDependencies {
	return handler.APIKeyDependencies{
		APIKeyRepo: apiKeyRepo,
		UserRepo:   userRepo,
		Generator:  generator,
	}
}

func newSessionDependencies(sessionReadRepo session.SessionReadRepository) handler.SessionDependencies {
	return handler.SessionDependencies{SessionReadRepo: sessionReadRepo}
}

func newOpenAIDependencies(endpointRepo llmproxy.EndpointRepository, endpointReadRepo llmproxy.EndpointReadRepository, openAIProxy transport.OpenAIProxy, anthropicProxy transport.AnthropicProxy) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{
		EndpointRepo:     endpointRepo,
		EndpointReadRepo: endpointReadRepo,
		OpenAIProxy:      openAIProxy,
		AnthropicProxy:   anthropicProxy,
	}
}

func newAnthropicDependencies(endpointRepo llmproxy.EndpointRepository, endpointReadRepo llmproxy.EndpointReadRepository, openAIProxy transport.OpenAIProxy, anthropicProxy transport.AnthropicProxy) handler.AnthropicDependencies {
	return handler.AnthropicDependencies{
		EndpointRepo:     endpointRepo,
		EndpointReadRepo: endpointReadRepo,
		OpenAIProxy:      openAIProxy,
		AnthropicProxy:   anthropicProxy,
	}
}
