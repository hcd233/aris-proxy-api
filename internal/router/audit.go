package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler, cronHandler handler.CronHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner, demoAccessor demoport.DemoModuleAccessor) {
	auditGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
	auditGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/model/log/list",
		Summary:     "ListAuditLogs",
		Description: "Paginate audit logs scoped by current JWT user. Admin sees all records; regular user sees records under their own API keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listAuditLogs", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleListAuditLogs)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditOptions",
		Method:      http.MethodGet,
		Path:        "/model/option/list",
		Summary:     "ListAuditOptions",
		Description: "Get available options for audit filter fields (user, model)",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listAuditOptions", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleListAuditOption)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelTrend",
		Method:      http.MethodGet,
		Path:        "/stats/model/trend",
		Summary:     "QueryModelTrend",
		Description: "Query model call count trend grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryModelTrend", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleModelTrend)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryRequestRate",
		Method:      http.MethodGet,
		Path:        "/stats/request/rate",
		Summary:     "QueryRequestRate",
		Description: "Query request success rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryRequestRate", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleRequestRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenThroughput",
		Method:      http.MethodGet,
		Path:        "/stats/token/throughput",
		Summary:     "QueryTokenThroughput",
		Description: "Query token throughput (volume + output rate) grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryTokenThroughput", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleTokenThroughput)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenRate",
		Method:      http.MethodGet,
		Path:        "/stats/token/rate",
		Summary:     "QueryTokenRate",
		Description: "Query output token rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryTokenRate", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleTokenRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelUsage",
		Method:      http.MethodGet,
		Path:        "/stats/model/usage",
		Summary:     "QueryModelUsage",
		Description: "Query aggregated model usage per model. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryModelUsage", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleModelUsage)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryFirstTokenLatency",
		Method:      http.MethodGet,
		Path:        "/stats/token/latency",
		Summary:     "QueryFirstTokenLatency",
		Description: "Query average first token latency grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryFirstTokenLatency", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor)},
	}, auditHandler.HandleFirstTokenLatency)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAudits",
		Method:      http.MethodGet,
		Path:        "/cron/log" + constant.RoutePathList,
		Summary:     "ListCronCallAudits",
		Description: "Paginate cron call audit records",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listCronCallAudits", enum.PermissionAdmin, enum.DemoModuleCronAudit, demoAccessor)},
	}, cronHandler.HandleListCronCallAudits)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAuditOptions",
		Method:      http.MethodGet,
		Path:        "/cron/option/list",
		Summary:     "ListCronCallAuditOptions",
		Description: "Get available filter options for cron call audit (cron type)",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listCronCallAuditOptions", enum.PermissionAdmin, enum.DemoModuleCronAudit, demoAccessor)},
	}, cronHandler.HandleListCronCallAuditOptions)
}
