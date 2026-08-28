package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initSessionJWTRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessions",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListSessions",
		Description: "Paginate session list for current user (JWT auth)",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listSessions", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleListSessionsByUser)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "",
		Summary:     "GetSession",
		Description: "Get session detail by session ID (JWT auth)",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("getSession", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleGetSessionByUser)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSessionMetadata",
		Method:      http.MethodGet,
		Path:        "/metadata",
		Summary:     "GetSessionMetadata",
		Description: "Get session metadata (without messages/tools content)",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("getSessionMetadata", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleGetSessionMetadata)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionMessages",
		Method:      http.MethodGet,
		Path:        "/message/list",
		Summary:     "ListSessionMessages",
		Description: "Paginate session messages by offset+limit",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listSessionMessages", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleListSessionMessages)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionTools",
		Method:      http.MethodGet,
		Path:        "/tool/list",
		Summary:     "ListSessionTools",
		Description: "Paginate session tools by offset+limit",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listSessionTools", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleListSessionTools)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "deleteSession",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "DeleteSession",
		Description: "Delete a session by ID (owner or admin)",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteSession", enum.PermissionUser)},
	}, sessionHandler.HandleDeleteSession)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "scoreSession",
		Method:      http.MethodPost,
		Path:        "/score",
		Summary:     "ScoreSession",
		Description: "Submit manual rating (1-5) for a session",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("scoreSession", enum.PermissionUser)},
	}, sessionHandler.HandleScoreSession)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "deleteScoreSession",
		Method:      http.MethodDelete,
		Path:        "/score",
		Summary:     "DeleteScoreSession",
		Description: "Remove manual rating for a session",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteScoreSession", enum.PermissionUser)},
	}, sessionHandler.HandleDeleteScoreSession)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionOptions",
		Method:      http.MethodGet,
		Path:        "/option/list",
		Summary:     "ListSessionOptions",
		Description: "Get available options for session filter fields (score, model)",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listSessionOptions", enum.PermissionUser, enum.DemoModuleSessions, demoAccessor, auditSubmitter)},
	}, sessionHandler.HandleListSessionOption)

	initSessionShareRouter(sessionGroup, sessionHandler)
}

func initSessionShareRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "createShare",
		Method:      http.MethodPost,
		Path:        "/share",
		Summary:     "CreateShare",
		Description: "Create a share link for a session",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		// shares 不在 demo 模块白名单，demo 一律拒绝
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("createShare", enum.PermissionUser)},
	}, sessionHandler.HandleCreateShare)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listShares",
		Method:      http.MethodGet,
		Path:        "/share/list",
		Summary:     "ListShares",
		Description: "List all share links for current user",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listShares", enum.PermissionUser)},
	}, sessionHandler.HandleListShares)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "deleteShare",
		Method:      http.MethodDelete,
		Path:        "/share",
		Summary:     "DeleteShare",
		Description: "Delete a share link",
		Tags:        []string{constant.TagSession},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteShare", enum.PermissionUser)},
	}, sessionHandler.HandleDeleteShare)
}

func initSessionPublicRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, cache *redis.Client) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getShareMetadata",
		Method:      http.MethodGet,
		Path:        "/share/metadata",
		Summary:     "GetShareMetadata",
		Description: "Get shared session metadata (public, rate limited)",
		Tags:        []string{constant.TagSession},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "getShareMetadata", "", constant.PeriodGetShareMetadata, constant.LimitGetShareMetadata),
		},
	}, sessionHandler.HandleGetShareMetadata)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listShareMessages",
		Method:      http.MethodGet,
		Path:        "/share/message/list",
		Summary:     "ListShareMessages",
		Description: "Paginate shared session messages (public, rate limited)",
		Tags:        []string{constant.TagSession},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "listShareMessages", "", constant.PeriodListShareMessages, constant.LimitListShareMessages),
		},
	}, sessionHandler.HandleListShareMessages)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listShareTools",
		Method:      http.MethodGet,
		Path:        "/share/tool/list",
		Summary:     "ListShareTools",
		Description: "Paginate shared session tools (public, rate limited)",
		Tags:        []string{constant.TagSession},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "listShareTools", "", constant.PeriodListShareTools, constant.LimitListShareTools),
		},
	}, sessionHandler.HandleListShareTools)
}
