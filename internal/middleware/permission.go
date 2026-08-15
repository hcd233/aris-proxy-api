package middleware

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// LimitUserPermissionMiddleware 限制用户权限中间件
//
// Demo 账户（level 低于 user）一律拒绝；需要按模块放行 Demo 的只读接口
// 使用 LimitUserPermissionWithDemoMiddleware。
//
//	@param serviceName string
//	@param requiredPermission model.Permission
//	@return ctx huma.Context
//	@return next func(huma.Context)
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func LimitUserPermissionMiddleware(serviceName string, requiredPermission enum.Permission) func(ctx huma.Context, next func(huma.Context)) {
	return limitUserPermission(serviceName, requiredPermission, "", nil)
}

// LimitUserPermissionWithDemoMiddleware 限制用户权限中间件（Demo 模块白名单放行）
//
// 非 Demo 用户走常规 level 比较；Demo 用户在 demoModule 非空且配置开放该模块时放行
// （fail-closed：accessor 缺失或配置读取失败均拒绝）。
//
//	@param serviceName string
//	@param requiredPermission enum.Permission
//	@param demoModule enum.DemoModule 开放给 Demo 的模块 key，空串=对 Demo 一律拒绝
//	@param demoAccessor demoport.DemoModuleAccessor Demo 模块放行判断
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func LimitUserPermissionWithDemoMiddleware(serviceName string, requiredPermission enum.Permission, demoModule enum.DemoModule, demoAccessor demoport.DemoModuleAccessor) func(ctx huma.Context, next func(huma.Context)) {
	return limitUserPermission(serviceName, requiredPermission, demoModule, demoAccessor)
}

func limitUserPermission(serviceName string, requiredPermission enum.Permission, demoModule enum.DemoModule, demoAccessor demoport.DemoModuleAccessor) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		permission := util.CtxValuePermission(ctx.Context())
		if permission == "" {
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		if permission == enum.PermissionDemo {
			if demoModule == "" || demoAccessor == nil || !demoAccessor.IsModuleOpen(ctx.Context(), demoModule) {
				logger.WithCtx(ctx.Context()).Info("[LimitUserPermissionMiddleware] Demo module denied",
					zap.String("serviceName", serviceName),
					zap.String("demoModule", demoModule))
				lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError().Localize(i18n.FromCtx(ctx.Context()))))
				return
			}
			next(ctx)
			return
		}

		if permission.Level() < requiredPermission.Level() {
			logger.WithCtx(ctx.Context()).Info("[LimitUserPermissionMiddleware] Permission denied",
				zap.String("serviceName", serviceName),
				zap.String("requiredPermission", string(requiredPermission)),
				zap.String("permission", string(permission)))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		next(ctx)
	}
}
