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

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelTrend",
		Method:      http.MethodGet,
		Path:        "/stats/model/trend",
		Summary:     "QueryModelTrend",
		Description: "Query model call count trend grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryModelTrend", enum.PermissionUser)},
	}, auditHandler.HandleModelTrend)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryRequestRate",
		Method:      http.MethodGet,
		Path:        "/stats/request/rate",
		Summary:     "QueryRequestRate",
		Description: "Query request success rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryRequestRate", enum.PermissionUser)},
	}, auditHandler.HandleRequestRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenThroughput",
		Method:      http.MethodGet,
		Path:        "/stats/token/throughput",
		Summary:     "QueryTokenThroughput",
		Description: "Query token throughput (volume + output rate) grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenThroughput", enum.PermissionUser)},
	}, auditHandler.HandleTokenThroughput)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenRate",
		Method:      http.MethodGet,
		Path:        "/stats/token/rate",
		Summary:     "QueryTokenRate",
		Description: "Query output token rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenRate", enum.PermissionUser)},
	}, auditHandler.HandleTokenRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenUsage",
		Method:      http.MethodGet,
		Path:        "/stats/token/usage",
		Summary:     "QueryTokenUsage",
		Description: "Query aggregated token usage per model. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenUsage", enum.PermissionUser)},
	}, auditHandler.HandleTokenUsage)
}
