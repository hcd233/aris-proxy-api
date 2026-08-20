package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"gorm.io/gorm"
)

func initDemoRouter(demoGroup huma.API, demoHandler handler.DemoHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	// 无鉴权组：登录入口（IP 限流，参考 oauth2 callback）
	huma.Register(demoGroup, huma.Operation{
		OperationID: "demoLogin",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "DemoLogin",
		Description: "Login as the global demo account without OAuth (requires demo login entry enabled and a demo account configured)",
		Tags:        []string{constant.TagDemo},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "demoLogin", "", constant.PeriodDemoLogin, constant.LimitDemoLogin),
		},
	}, demoHandler.HandleLogin)

	huma.Register(demoGroup, huma.Operation{
		OperationID: "demoStatus",
		Method:      http.MethodGet,
		Path:        "/status",
		Summary:     "DemoStatus",
		Description: "Get the demo login entry status for the login page (public, rate limited)",
		Tags:        []string{constant.TagDemo},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "demoStatus", "", constant.PeriodDemoLogin, constant.LimitDemoLogin),
		},
	}, demoHandler.HandleStatus)

	// JWT 组：配置读取（登录用户均可）与更新（admin）
	demoConfigGroup := huma.NewGroup(demoGroup, "/config")
	demoConfigGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(demoConfigGroup, huma.Operation{
		OperationID: "getDemoConfig",
		Method:      http.MethodGet,
		Path:        "",
		Summary:     "GetDemoConfig",
		Description: "Get the demo account configuration (any logged-in user; used by the web app to render demo navigation)",
		Tags:        []string{constant.TagDemo},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
	}, demoHandler.HandleGetConfig)

	huma.Register(demoConfigGroup, huma.Operation{
		OperationID: "updateDemoConfig",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateDemoConfig",
		Description: "Update the demo account configuration (admin only)",
		Tags:        []string{constant.TagDemo},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateDemoConfig", enum.PermissionAdmin),
		},
	}, demoHandler.HandleUpdateConfig)

	demoSessionsGroup := huma.NewGroup(demoGroup, "/sessions")
	demoSessionsGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "listDemoSessions",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListDemoSessions",
		Description: "List sessions whitelisted for the demo account (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleListDemoSessions)

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "addDemoSessions",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "AddDemoSessions",
		Description: "Batch add sessions to the demo whitelist (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("addDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleAddDemoSessions)

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "removeDemoSessions",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "RemoveDemoSessions",
		Description: "Batch remove sessions from the demo whitelist (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("removeDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleRemoveDemoSessions)
}
