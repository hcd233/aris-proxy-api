package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initEndpointRouter(endpointGroup huma.API, endpointHandler handler.EndpointHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	endpointGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
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
			middleware.LimitUserPermissionMiddleware("createEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleCreateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "listEndpoints",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListEndpoints",
		Description: "List all endpoint configurations",
		Tags:        []string{constant.TagEndpoint},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listEndpoints", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleListEndpoints)

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
			middleware.LimitUserPermissionMiddleware("updateEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleUpdateEndpoint)

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
			middleware.LimitUserPermissionMiddleware("deleteEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleDeleteEndpoint)
}
