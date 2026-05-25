package bootstrap

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	appenum "github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/hcd233/aris-proxy-api/internal/web"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go.uber.org/dig"
	"gorm.io/gorm"
)

type oauth2ExchangeCodeBody struct {
	Code string `json:"code"`
}

type routeParams struct {
	dig.In

	DB               *gorm.DB
	Cache            *redis.Client
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	Oauth2Callback   command.HandleCallbackHandler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	EndpointHandler  handler.EndpointHandler
	ModelHandler     handler.ModelHandler
	AuditHandler     handler.AuditHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
}

// RegisterRoutes 注册文档和 API 路由。
//
//	@param server *Server
//	@return error
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func RegisterRoutes(server *Server) error {
	return server.container.Invoke(func(params routeParams) {
		if config.Env != appenum.EnvProduction {
			router.RegisterDocsRouter(server.App)
		}
		router.RegisterAPIRouter(server.HumaAPI, router.APIRouterDependencies{
			DB:               params.DB,
			Cache:            params.Cache,
			PingHandler:      params.PingHandler,
			TokenHandler:     params.TokenHandler,
			Oauth2Handler:    params.Oauth2Handler,
			UserHandler:      params.UserHandler,
			APIKeyHandler:    params.APIKeyHandler,
			SessionHandler:   params.SessionHandler,
			EndpointHandler:  params.EndpointHandler,
			ModelHandler:     params.ModelHandler,
			AuditHandler:     params.AuditHandler,
			OpenAIHandler:    params.OpenAIHandler,
			AnthropicHandler: params.AnthropicHandler,
		})

		server.App.Get(constant.RoutePathOAuth2Callback, func(c fiber.Ctx) error {
			code := c.Query(constant.QueryCode)
			state := c.Query(constant.QueryState)
			platform := c.Query(constant.QueryPlatform, string(enum.Oauth2PlatformGithub))

			result, err := params.Oauth2Callback.Handle(c.Context(), command.HandleCallbackCommand{
				Platform: enum.Oauth2Platform(platform),
				Code:     code,
				State:    state,
			})
			if err != nil {
				logger.WithCtx(c.Context()).Error("[OAuth2BrowserCallback] Callback failed", zap.Error(err))
				return c.Redirect().Status(fiber.StatusFound).To(constant.RoutePathWebLogin + "?" + constant.QueryParamError + "=" + constant.MsgAuthFailed)
			}

			oneTimeCode, storeErr := storeOAuthCallbackTokens(c, params.Cache, result.TokenPair.AccessToken(), result.TokenPair.RefreshToken())
			if storeErr != nil {
				logger.WithCtx(c.Context()).Error("[OAuth2BrowserCallback] Failed to store callback tokens", zap.Error(storeErr))
				return c.Redirect().Status(fiber.StatusFound).To(constant.RoutePathWebLogin + "?" + constant.QueryParamError + "=" + constant.MsgAuthFailed)
			}

			redirectURL := constant.RoutePathWebAuthCallback + "?" + constant.QueryCode + "=" + oneTimeCode
			return c.Redirect().Status(fiber.StatusFound).To(redirectURL)
		})

		server.App.Post(constant.RoutePathOAuth2ExchangeCode, func(c fiber.Ctx) error {
			var body oauth2ExchangeCodeBody
			if err := c.Bind().JSON(&body); err != nil || body.Code == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{constant.QueryParamError: constant.MsgCodeRequired})
			}

			accessToken, refreshToken, err := exchangeOAuthCallbackCode(c, params.Cache, body.Code)
			if err != nil {
				return c.Status(fiber.StatusGone).JSON(fiber.Map{constant.QueryParamError: constant.MsgInvalidOrExpiredCode})
			}

			return c.JSON(fiber.Map{
				constant.FieldAccessToken:  accessToken,
				constant.FieldRefreshToken: refreshToken,
			})
		})

		router.RegisterWebRouter(server.App, web.DistFS)
	})
}

func storeOAuthCallbackTokens(c fiber.Ctx, cache *redis.Client, accessToken, refreshToken string) (string, error) {
	b := make([]byte, constant.OAuthCallbackCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	oneTimeCode := hex.EncodeToString(b)

	key := constant.OAuthCallbackCodeKeyPrefix + oneTimeCode
	if err := cache.HSet(c.Context(), key, constant.FieldOAuthAccessToken, accessToken, constant.FieldOAuthRefreshToken, refreshToken).Err(); err != nil {
		return "", err
	}
	if err := cache.Expire(c.Context(), key, constant.OAuthCallbackCodeTTL).Err(); err != nil {
		return "", err
	}
	return oneTimeCode, nil
}

func exchangeOAuthCallbackCode(c fiber.Ctx, cache *redis.Client, code string) (string, string, error) {
	key := constant.OAuthCallbackCodeKeyPrefix + code

	pipe := cache.Pipeline()
	getCmd := pipe.HGetAll(c.Context(), key)
	pipe.Del(c.Context(), key)
	if _, err := pipe.Exec(c.Context()); err != nil {
		return "", "", err
	}

	result := getCmd.Val()
	accessToken, hasAccess := result[constant.FieldOAuthAccessToken]
	refreshToken, hasRefresh := result[constant.FieldOAuthRefreshToken]
	if !hasAccess || !hasRefresh {
		return "", "", ierr.New(ierr.ErrUnauthorized, constant.MsgInvalidOrExpiredCode)
	}
	return accessToken, refreshToken, nil
}
