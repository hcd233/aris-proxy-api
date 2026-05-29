package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler, db *gorm.DB, cache *redis.Client) {
	auditGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/log/list",
		Summary:     "ListAuditLogs",
		Description: "Paginate audit logs scoped by current JWT user. Admin sees all records; regular user sees records under their own API keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listAuditLogs", enum.PermissionUser)},
	}, auditHandler.HandleListAuditLogs)
}
