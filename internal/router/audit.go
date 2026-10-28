package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler, cronHandler handler.CronHandler, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/model/log/list",
		Summary:     "ListAuditLogs",
		Description: "Paginate audit logs scoped by current JWT user. Admin sees all records; regular user sees records under their own API keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listAuditLogs", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleListAuditLogs)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditOptions",
		Method:      http.MethodGet,
		Path:        "/model/option/list",
		Summary:     "ListAuditOptions",
		Description: "Get available options for audit filter fields (user, model)",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listAuditOptions", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleListAuditOption)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelTrend",
		Method:      http.MethodGet,
		Path:        "/stats/model/trend",
		Summary:     "QueryModelTrend",
		Description: "Query model call count trend grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryModelTrend", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleModelTrend)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelCost",
		Method:      http.MethodGet,
		Path:        "/stats/model/cost",
		Summary:     "QueryModelCost",
		Description: "Query per-model estimated cost (input/output/cache write/cache read + total), row = model x currency. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryModelCost", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleModelCost)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryDashboardOverview",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/overview",
		Summary:     "QueryDashboardOverview",
		Description: "Dashboard KPI summary and bucketed series for current and previous equal-length period. Admin/demo see all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryDashboardOverview", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardOverview)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryDashboardModelSeries",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/model/series",
		Summary:     "QueryDashboardModelSeries",
		Description: "Dashboard per-model usage (requests/tokens/cost) by time bucket.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryDashboardModelSeries", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardModelSeries)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listDashboardErrors",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/error/list",
		Summary:     "ListDashboardErrors",
		Description: "Dashboard top failed-call errors grouped by status code and message.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listDashboardErrors", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardErrors)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listDashboardEndpoints",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/endpoint/list",
		Summary:     "ListDashboardEndpoints",
		Description: "Dashboard endpoint health (requests, error rate, P95 latency, speed, cost). Endpoint names are masked for demo.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listDashboardEndpoints", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardEndpoints)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listDashboardTop",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/top/list",
		Summary:     "ListDashboardTop",
		Description: "Dashboard top ranking by model/user/apikey/harness. The user dimension is admin/demo only; demo names are masked.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listDashboardTop", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardTop)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryDashboardHeatmap",
		Method:      http.MethodGet,
		Path:        "/stats/dashboard/heatmap",
		Summary:     "QueryDashboardHeatmap",
		Description: "Dashboard weekday x hour request heatmap in the given IANA timezone.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryDashboardHeatmap", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleDashboardHeatmap)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryHarnessTrend",
		Method:      http.MethodGet,
		Path:        "/stats/harness/trend",
		Summary:     "QueryHarnessTrend",
		Description: "Query model call count trend grouped by agent harness and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryHarnessTrend", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleHarnessTrend)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryRequestRate",
		Method:      http.MethodGet,
		Path:        "/stats/request/rate",
		Summary:     "QueryRequestRate",
		Description: "Query request success rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryRequestRate", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleRequestRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenThroughput",
		Method:      http.MethodGet,
		Path:        "/stats/token/throughput",
		Summary:     "QueryTokenThroughput",
		Description: "Query token throughput (volume + output rate) grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryTokenThroughput", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleTokenThroughput)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenRate",
		Method:      http.MethodGet,
		Path:        "/stats/token/rate",
		Summary:     "QueryTokenRate",
		Description: "Query output token rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryTokenRate", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleTokenRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryModelUsage",
		Method:      http.MethodGet,
		Path:        "/stats/model/usage",
		Summary:     "QueryModelUsage",
		Description: "Query aggregated model usage per model. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryModelUsage", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleModelUsage)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryFirstTokenLatency",
		Method:      http.MethodGet,
		Path:        "/stats/token/latency",
		Summary:     "QueryFirstTokenLatency",
		Description: "Query average first token latency grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("queryFirstTokenLatency", enum.PermissionUser, enum.DemoModuleAudit, demoAccessor, auditSubmitter)},
	}, auditHandler.HandleFirstTokenLatency)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAudits",
		Method:      http.MethodGet,
		Path:        "/cron/log" + constant.RoutePathList,
		Summary:     "ListCronCallAudits",
		Description: "Paginate cron call audit records",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listCronCallAudits", enum.PermissionAdmin, enum.DemoModuleCronAudit, demoAccessor, auditSubmitter)},
	}, cronHandler.HandleListCronCallAudits)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAuditOptions",
		Method:      http.MethodGet,
		Path:        "/cron/option/list",
		Summary:     "ListCronCallAuditOptions",
		Description: "Get available filter options for cron call audit (cron type)",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("listCronCallAuditOptions", enum.PermissionAdmin, enum.DemoModuleCronAudit, demoAccessor, auditSubmitter)},
	}, cronHandler.HandleListCronCallAuditOptions)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listDemoAccessAudits",
		Method:      http.MethodGet,
		Path:        "/demo/log" + constant.RoutePathList,
		Summary:     "ListDemoAccessAudits",
		Description: "Paginate demo access audit records (admin only; not exposed to the demo account)",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listDemoAccessAudits", enum.PermissionAdmin)},
	}, cronHandler.HandleListDemoAccessAudits)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listDemoAccessAuditOptions",
		Method:      http.MethodGet,
		Path:        "/demo/option/list",
		Summary:     "ListDemoAccessAuditOptions",
		Description: "Get available filter options for demo access audit (action, module)",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listDemoAccessAuditOptions", enum.PermissionAdmin)},
	}, cronHandler.HandleListDemoAccessAuditOptions)
}
