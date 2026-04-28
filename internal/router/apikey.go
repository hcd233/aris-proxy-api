// Package router API Key 路由
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAPIKeyRouter(apikeyGroup huma.API, apiKeyHandler handler.APIKeyHandler) {
	apikeyGroup.UseMiddleware(middleware.JwtMiddleware())
	// 限流: 防止快速创建 Key 或枚举 ID
	apikeyGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		"apikey-manage",
		string(constant.CtxKeyUserID),
		constant.PeriodManageAPIKey,
		constant.LimitManageAPIKey,
	))

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "createAPIKey",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateAPIKey",
		Description: "Create a new API key for the current user",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createAPIKey", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleCreateAPIKey)

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "listAPIKeys",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "ListAPIKeys",
		Description: "List all API keys for the current user (admin sees all)",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listAPIKeys", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleListAPIKeys)

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "deleteAPIKey",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteAPIKey",
		Description: "Delete an API key by ID (owner or admin)",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteAPIKey", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleDeleteAPIKey)
}
