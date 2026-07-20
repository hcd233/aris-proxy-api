package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	traceport "github.com/hcd233/aris-proxy-api/internal/application/trace/port"
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
	TicketStore  traceport.TraceClientTicketStore
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

	huma.Register(queryGroup, huma.Operation{
		OperationID: "getTraceConversation", Method: http.MethodGet, Path: "/conversation",
		Summary: "GetTraceConversation", Description: "Get reconstructed Codex conversation",
		Tags: []string{constant.TagTrace}, Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getTraceConversation", enum.PermissionUser)},
	}, deps.TraceHandler.HandleGetTraceConversation)

	huma.Register(queryGroup, huma.Operation{
		OperationID: "issueTraceClientTicket", Method: http.MethodPost, Path: "/client/ticket",
		Summary: "IssueTraceClientTicket", Description: "Issue a one-time trace client download ticket",
		Tags: []string{constant.TagTrace}, Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(
				cache,
				constant.TraceClientTicketRateLimitService,
				constant.CtxKeyUserID,
				constant.PeriodIssueTraceClientTicket,
				constant.LimitIssueTraceClientTicket,
			),
			middleware.LimitUserPermissionMiddleware("issueTraceClientTicket", enum.PermissionUser),
		},
	}, deps.TraceHandler.HandleIssueTraceClientTicket)

	// 上报组（API Key 鉴权，codex hook 用 Bearer）
	reportGroup := huma.NewGroup(traceGroup, "")
	reportGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(reportGroup, huma.Operation{
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: "/event",
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)

	huma.Register(reportGroup, huma.Operation{
		OperationID: "checkTraceClientAPIKey", Method: http.MethodGet, Path: "/client/check",
		Summary: "CheckTraceClientAPIKey", Description: "Validate the trace client API key",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleCheckTraceClient)

	downloadGroup := huma.NewGroup(traceGroup, "")
	downloadGroup.UseMiddleware(middleware.TraceClientTicketMiddleware(deps.TicketStore))
	huma.Register(downloadGroup, huma.Operation{
		OperationID: "downloadTraceClient", Method: http.MethodGet, Path: "/client",
		Summary: "DownloadTraceClient", Description: "Download a supported trace client binary",
		Tags: []string{constant.TagTrace},
	}, deps.TraceHandler.HandleDownloadTraceClient)

	// install 不使用 middleware，handler 内部验证票据并统一返回 bash 脚本
	// （包括错误路径），避免 curl|bash 执行到 JSON 报错。
	huma.Register(traceGroup, huma.Operation{
		OperationID: "installTraceClient", Method: http.MethodGet, Path: "/client/install",
		Summary:     "InstallTraceClient",
		Description: "Return a short install script authenticated by a one-time ticket",
		Tags:        []string{constant.TagTrace},
	}, deps.TraceHandler.HandleInstallTraceClient)
}
