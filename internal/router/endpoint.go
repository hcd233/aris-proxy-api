package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initEndpointRouter(endpointGroup huma.API, endpointHandler handler.EndpointHandler, db *gorm.DB, cache *redis.Client) {
	endpointGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))
	endpointGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "endpointManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "createEndpoint",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateEndpoint",
		Description: "Create a new endpoint configuration",
		Tags:        []string{"Endpoint"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleCreateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "listEndpoints",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListEndpoints",
		Description: "List all endpoint configurations",
		Tags:        []string{"Endpoint"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listEndpoints", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleListEndpoints)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "updateEndpoint",
		Method:      http.MethodPatch,
		Path:        "/",
		Summary:     "UpdateEndpoint",
		Description: "Update an endpoint configuration",
		Tags:        []string{"Endpoint"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleUpdateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "deleteEndpoint",
		Method:      http.MethodDelete,
		Path:        "/",
		Summary:     "DeleteEndpoint",
		Description: "Delete an endpoint configuration",
		Tags:        []string{"Endpoint"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteEndpoint", enum.PermissionAdmin),
		},
	}, endpointHandler.HandleDeleteEndpoint)
}
