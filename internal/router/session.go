package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
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
}
