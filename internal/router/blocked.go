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

func initBlockedRouter(group huma.API, handler handler.BlockedHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	group.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(group, huma.Operation{
		OperationID: "createBlocked",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "CreateBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleCreateBlocked)

	huma.Register(group, huma.Operation{
		OperationID: "listBlocked",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleListBlocked)

	huma.Register(group, huma.Operation{
		OperationID: "deleteBlocked",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleDeleteBlocked)
}
