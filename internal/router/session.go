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
		Path:        "/",
		Summary:     "GetSession",
		Description: "Get session detail by session ID (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSession", enum.PermissionUser)},
	}, sessionHandler.HandleGetSessionByUser)

	shareGroup := huma.NewGroup(sessionGroup, "/share")
	initSessionShareRouter(shareGroup, sessionHandler)
}

func initSessionShareRouter(shareGroup huma.API, sessionHandler handler.SessionHandler) {
	huma.Register(shareGroup, huma.Operation{
		OperationID: "createShare",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateShare",
		Description: "Create a share link for a session",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleCreateShare)

	huma.Register(shareGroup, huma.Operation{
		OperationID: "listShares",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListShares",
		Description: "List all share links for current user",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleListShares)

	huma.Register(shareGroup, huma.Operation{
		OperationID: "deleteShare",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteShare",
		Description: "Delete a share link",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleDeleteShare)
}

func initSessionPublicRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, cache *redis.Client) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getShareContent",
		Method:      http.MethodGet,
		Path:        "/share/{id}",
		Summary:     "GetShareContent",
		Description: "Get shared session content (public, rate limited)",
		Tags:        []string{"Session"},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "getShareContent", "", constant.PeriodGetShareContent, constant.LimitGetShareContent),
		},
	}, sessionHandler.HandleGetShareContent)
}
