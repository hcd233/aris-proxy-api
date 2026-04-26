package middleware

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// LimitUserPermissionMiddleware 限制用户权限中间件
//
//	@param serviceName string
//	@param requiredPermission model.Permission
//	@return ctx huma.Context
//	@return next func(huma.Context)
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2025-11-02 04:16:51
func LimitUserPermissionMiddleware(serviceName string, requiredPermission enum.Permission) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		permission := util.CtxValuePermission(ctx.Context())
		if permission == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError()))
			return
		}

		if permission.Level() < requiredPermission.Level() {
			logger.WithCtx(ctx.Context()).Info("[LimitUserPermissionMiddleware] Permission denied",
				zap.String("serviceName", serviceName),
				zap.String("requiredPermission", string(requiredPermission)),
				zap.String("permission", string(permission)))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError()))
			return
		}

		next(ctx)
	}
}
