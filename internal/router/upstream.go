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

// initUpstreamRouter 初始化 Upstream 分组视图路由（JWT 鉴权）
//
//	@param upstreamGroup huma.API
//	@param upstreamHandler handler.UpstreamHandler
//	@param db *gorm.DB
//	@param cache *redis.Client
//	@param accessSigner jwt.TokenSigner
//	@param demoAccessor demoport.DemoModuleAccessor
//	@param auditSubmitter demoport.DemoSubmitter
//	@author centonhuang
//	@update 2026-08-27 10:00:00
func initUpstreamRouter(upstreamGroup huma.API, upstreamHandler handler.UpstreamHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
	upstreamGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
	upstreamGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))
	upstreamGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "upstreamManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(upstreamGroup, huma.Operation{
		OperationID: "listUpstream",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListUpstream",
		Description: "List endpoint-grouped upstream configurations",
		Tags:        []string{constant.TagUpstream},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionWithDemoMiddleware("listUpstream", enum.PermissionUser, enum.DemoModuleUpstream, demoAccessor, auditSubmitter),
		},
	}, upstreamHandler.HandleListUpstream)
}
