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

func initCronRouter(cronGroup huma.API, cronHandler handler.CronHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	cronGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(cronGroup, huma.Operation{
		OperationID: "listCronJobs",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListCronJobs",
		Description: "List all cron jobs with their enabled status",
		Tags:        []string{constant.TagCron},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronJobs", enum.PermissionAdmin)},
	}, cronHandler.HandleListCronJobs)

	huma.Register(cronGroup, huma.Operation{
		OperationID: "updateCronJob",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateCronJob",
		Description: "Enable or disable a cron job",
		Tags:        []string{constant.TagCron},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateCronJob", enum.PermissionAdmin)},
	}, cronHandler.HandleUpdateCronJob)
}

func initCronAuditRouter(auditGroup huma.API, cronHandler handler.CronHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	auditGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAudits",
		Method:      http.MethodGet,
		Path:        "/log" + constant.RoutePathList,
		Summary:     "ListCronCallAudits",
		Description: "Paginate cron call audit records",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronCallAudits", enum.PermissionAdmin)},
	}, cronHandler.HandleListCronCallAudits)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listCronCallAuditOptions",
		Method:      http.MethodGet,
		Path:        "/option" + constant.RoutePathOptionList,
		Summary:     "ListCronCallAuditOptions",
		Description: "Get available filter options for cron call audit (cron type)",
		Tags:        []string{constant.TagCronAudit},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronCallAuditOptions", enum.PermissionAdmin)},
	}, cronHandler.HandleListCronCallAuditOptions)
}
