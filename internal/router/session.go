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

func initSessionJWTRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, db *gorm.DB, cache *redis.Client) {
	sessionGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessions",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListSessions",
		Description: "Paginate session list for current user (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessions", enum.PermissionUser)},
	}, sessionHandler.HandleListSessionsByUser)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "",
		Summary:     "GetSession",
		Description: "Get session detail by session ID (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSession", enum.PermissionUser)},
	}, sessionHandler.HandleGetSessionByUser)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSessionMetadata",
		Method:      http.MethodGet,
		Path:        "/metadata",
		Summary:     "GetSessionMetadata",
		Description: "Get session metadata (without messages/tools content)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSessionMetadata", enum.PermissionUser)},
	}, sessionHandler.HandleGetSessionMetadata)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionMessages",
		Method:      http.MethodGet,
		Path:        "/message/list",
		Summary:     "ListSessionMessages",
		Description: "Paginate session messages by offset+limit",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessionMessages", enum.PermissionUser)},
	}, sessionHandler.HandleListSessionMessages)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionTools",
		Method:      http.MethodGet,
		Path:        "/tool/list",
		Summary:     "ListSessionTools",
		Description: "Paginate session tools by offset+limit",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessionTools", enum.PermissionUser)},
	}, sessionHandler.HandleListSessionTools)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "deleteSession",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "DeleteSession",
		Description: "Delete a session by ID (owner or admin)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteSession", enum.PermissionUser)},
	}, sessionHandler.HandleDeleteSession)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "scoreSession",
		Method:      http.MethodPost,
		Path:        "/score",
		Summary:     "ScoreSession",
		Description: "Submit manual rating (1-5) for a session",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("scoreSession", enum.PermissionUser)},
	}, sessionHandler.HandleScoreSession)

	initSessionShareRouter(sessionGroup, sessionHandler)
}

func initSessionShareRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "createShare",
		Method:      http.MethodPost,
		Path:        "/share",
		Summary:     "CreateShare",
		Description: "Create a share link for a session",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleCreateShare)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listShares",
		Method:      http.MethodGet,
		Path:        "/share/list",
		Summary:     "ListShares",
		Description: "List all share links for current user",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleListShares)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "deleteShare",
		Method:      http.MethodDelete,
		Path:        "/share",
		Summary:     "DeleteShare",
		Description: "Delete a share link",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleDeleteShare)
}

func initSessionPublicRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, cache *redis.Client) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getShareMetadata",
		Method:      http.MethodGet,
		Path:        "/share/metadata",
		Summary:     "GetShareMetadata",
		Description: "Get shared session metadata (public, rate limited)",
		Tags:        []string{"Session"},
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
		Tags:        []string{"Session"},
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
		Tags:        []string{"Session"},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "listShareTools", "", constant.PeriodListShareTools, constant.LimitListShareTools),
		},
	}, sessionHandler.HandleListShareTools)
}
