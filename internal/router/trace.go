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

// TraceRouterDependencies trace 路由依赖
type TraceRouterDependencies struct {
	TraceHandler handler.TraceHandler
}

func initTraceRouter(traceGroup huma.API, deps TraceRouterDependencies, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	// 查询组（JWT + owner 隔离）
	queryGroup := huma.NewGroup(traceGroup, "")
	queryGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(queryGroup, huma.Operation{
		OperationID: "listTraces", Method: http.MethodGet, Path: constant.RoutePathList,
		Summary: "ListTraces", Description: "Paginate trace list for current user",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraces", enum.PermissionUser)},
	}, deps.TraceHandler.HandleListTraces)

	huma.Register(queryGroup, huma.Operation{
		OperationID: "getTrace", Method: http.MethodGet, Path: "",
		Summary: "GetTrace", Description: "Get trace detail by trace ID",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getTrace", enum.PermissionUser)},
	}, deps.TraceHandler.HandleGetTrace)

	huma.Register(queryGroup, huma.Operation{
		OperationID: "listTraceEvents", Method: http.MethodGet, Path: "/event/list",
		Summary: "ListTraceEvents", Description: "Paginate trace event timeline",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraceEvents", enum.PermissionUser)},
	}, deps.TraceHandler.HandleListTraceEvents)

	// 上报组（API Key 鉴权，codex hook 用 Bearer）
	reportGroup := huma.NewGroup(traceGroup, "")
	reportGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(reportGroup, huma.Operation{
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: "/event",
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)
}
