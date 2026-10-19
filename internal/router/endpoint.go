package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
)

func initEndpointRouter(endpointGroup huma.API, endpointHandler handler.EndpointHandler, cache *redis.Client) {
	endpointGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "endpointManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "createEndpoint",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "CreateEndpoint",
		Description: "Create a new endpoint configuration",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleCreateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "updateEndpoint",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateEndpoint",
		Description: "Update an endpoint configuration",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleUpdateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "authorizeOAuthEndpoint",
		Method:      http.MethodPost,
		Path:        "/oauth/authorize",
		Summary:     "AuthorizeOAuthEndpoint",
		Description: "Start subscription-account OAuth authorization and return the authorize URL",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("authorizeOAuthEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleAuthorizeOAuth)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "exchangeOAuthEndpoint",
		Method:      http.MethodPost,
		Path:        "/oauth/exchange",
		Summary:     "ExchangeOAuthEndpoint",
		Description: "Bind a subscription account as an endpoint using the authorization code",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("exchangeOAuthEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleExchangeOAuth)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "reauthOAuthEndpoint",
		Method:      http.MethodPost,
		Path:        "/oauth/reauth",
		Summary:     "ReauthOAuthEndpoint",
		Description: "Reauthorize an existing subscription endpoint with a fresh authorization code",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("reauthOAuthEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleReauthOAuth)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "getOAuthQuota",
		Method:      http.MethodGet,
		Path:        "/oauth/quota",
		Summary:     "GetOAuthQuota",
		Description: "Get subscription endpoint credential status and quota snapshot",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
	}, endpointHandler.HandleGetOAuthQuota)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "probeOAuthQuota",
		Method:      http.MethodPost,
		Path:        "/oauth/probe",
		Summary:     "ProbeOAuthQuota",
		Description: "Probe subscription endpoint quota synchronously and return the fresh snapshot",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("probeOAuthQuota", enum.PermissionUser),
		},
	}, endpointHandler.HandleProbeOAuthQuota)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "deleteEndpoint",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "DeleteEndpoint",
		Description: "Delete an endpoint configuration",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteEndpoint", enum.PermissionUser),
		},
	}, endpointHandler.HandleDeleteEndpoint)
}
