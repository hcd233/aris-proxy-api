// Package router Web 分区路由（/api/web/v1，session JWT 鉴权）
package router

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// RegisterWebAPIRoutes 注册 Web 分区路由。
//
// 结构约束（huma Group.Middlewares 继承语义，见 master 29727321 的教训）：
//   - jwtGroup：统一挂 JwtMiddleware + demoAccess 限流，所有业务子组挂在它下面
//   - publicGroup：同前缀下的平行组，不含 JWT；公开入口（oauth2 / token 刷新 /
//     demo 登录与状态 / 分享公开读）必须且只能注册到这里
//
// 模块级自有限流（如 userManage/modelManage）仍留在各自 init 函数内。
//
//	@param webRoot huma.API
//	@param deps APIRouterDependencies
//	@author centonhuang
//	@update 2026-08-27
func RegisterWebAPIRoutes(webRoot huma.API, deps APIRouterDependencies) {
	jwtGroup := huma.NewGroup(webRoot)
	jwtGroup.UseMiddleware(
		middleware.JwtMiddleware(deps.DB, deps.Cache, deps.AccessSigner),
		middleware.TokenBucketRateLimiterMiddleware(deps.Cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)),
	)
	publicGroup := huma.NewGroup(webRoot)

	// ── 公开入口（无 JWT） ──
	oauth2Group := huma.NewGroup(publicGroup, "/oauth2")
	initOauth2Router(oauth2Group, deps.Oauth2Handler, deps.Cache)

	tokenGroup := huma.NewGroup(publicGroup, "/token")
	initTokenRouter(tokenGroup, deps.TokenHandler, deps.Cache)

	demoPublicGroup := huma.NewGroup(publicGroup, "/demo")
	demoJWTGroup := huma.NewGroup(jwtGroup, "/demo")
	initDemoRouter(demoPublicGroup, demoJWTGroup, deps.DemoHandler, deps.Cache)

	sessionPublicGroup := huma.NewGroup(publicGroup, "/session")
	initSessionPublicRouter(sessionPublicGroup, deps.SessionHandler, deps.Cache)

	// ── JWT 业务子组 ──
	userGroup := huma.NewGroup(jwtGroup, "/user")
	initUserRouter(userGroup, deps.UserHandler, deps.Cache)

	apikeyGroup := huma.NewGroup(jwtGroup, "/apikey")
	initAPIKeyRouter(apikeyGroup, deps.APIKeyHandler, deps.Cache)

	sessionJWTGroup := huma.NewGroup(jwtGroup, "/session")
	initSessionJWTRouter(sessionJWTGroup, deps.SessionHandler, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	endpointGroup := huma.NewGroup(jwtGroup, "/endpoint")
	initEndpointRouter(endpointGroup, deps.EndpointHandler, deps.Cache)

	modelGroup := huma.NewGroup(jwtGroup, "/model")
	initModelRouter(modelGroup, deps.ModelHandler, deps.Cache)

	upstreamGroup := huma.NewGroup(jwtGroup, "/upstream")
	initUpstreamRouter(upstreamGroup, deps.UpstreamHandler, deps.Cache, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	auditGroup := huma.NewGroup(jwtGroup, "/audit")
	initAuditRouter(auditGroup, deps.AuditHandler, deps.CronHandler, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	cronGroup := huma.NewGroup(jwtGroup, "/cron")
	initCronRouter(cronGroup, deps.CronHandler, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	triggerGroup := huma.NewGroup(jwtGroup, "/trigger")
	initTriggerRouter(triggerGroup, deps.TriggerHandler, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	metricsGroup := huma.NewGroup(jwtGroup, "/metrics")
	initMetricsRouter(metricsGroup, deps.MetricsHandler, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	datasetGroup := huma.NewGroup(jwtGroup, "/dataset")
	initDatasetRouter(datasetGroup, deps.DatasetHandler)

	traceGroup := huma.NewGroup(jwtGroup, "/trace")
	initTraceQueryRouter(traceGroup, deps.TraceHandler)
}
