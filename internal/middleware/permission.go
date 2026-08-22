package middleware

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
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
	return limitUserPermission(serviceName, requiredPermission, "", nil, nil)
}

// LimitUserPermissionWithDemoMiddleware 限制用户权限中间件（Demo 模块白名单放行 + 访问审计）
//
// 非 Demo 用户走常规 level 比较；Demo 用户在 demoModule 非空且配置开放该模块时放行
// （fail-closed：accessor 缺失或配置读取失败均拒绝）。Demo 用户的每次访问（放行与被拒）
// 均经 auditSubmitter 提交审计（nil 时跳过）。
//
//	@param serviceName string
//	@param requiredPermission enum.Permission
//	@param demoModule enum.DemoModule 开放给 Demo 的模块 key，空串=对 Demo 一律拒绝
//	@param demoAccessor demoport.DemoModuleAccessor Demo 模块放行判断
//	@param auditSubmitter demoport.DemoSubmitter Demo 访问审计提交
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func LimitUserPermissionWithDemoMiddleware(serviceName string, requiredPermission enum.Permission, demoModule enum.DemoModule, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) func(ctx huma.Context, next func(huma.Context)) {
	return limitUserPermission(serviceName, requiredPermission, demoModule, demoAccessor, auditSubmitter)
}

// ClassifyDemoAccess 判定一次 demo 模块访问是否需要审计及动作分类
//
// 非 demo 身份返回 ok=false（admin/user 正常使用路由不产生审计记录）。
//
//	@param permission enum.Permission 当前请求身份
//	@param open bool 目标模块是否对 demo 开放
//	@return enum.DemoAccessAction
//	@return string 拒绝原因
//	@return bool 是否产生审计
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func ClassifyDemoAccess(permission enum.Permission, open bool) (action enum.DemoAccessAction, reason string, audited bool) {
	switch {
	case permission != enum.PermissionDemo:
		return "", "", false
	case open:
		return enum.DemoAccessActionModuleAccess, "", true
	default:
		return enum.DemoAccessActionModuleDenied, constant.DemoAccessReasonModuleClosed, true
	}
}

func limitUserPermission(serviceName string, requiredPermission enum.Permission, demoModule enum.DemoModule, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		permission := util.CtxValuePermission(ctx.Context())
		if permission == "" {
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		if permission == enum.PermissionDemo {
			open := demoModule != "" && demoAccessor != nil && demoAccessor.IsModuleOpen(ctx.Context(), demoModule)
			if action, reason, audited := ClassifyDemoAccess(permission, open); audited && auditSubmitter != nil {
				fCtx := humafiber.Unwrap(ctx)
				_ = auditSubmitter.SubmitDemoAccessAuditTask(&dto.DemoAccessAuditTask{ //nolint:errcheck // best-effort audit
					Ctx:       util.CopyContextValues(ctx.Context()),
					Action:    action,
					Module:    demoModule,
					Path:      fCtx.Path(),
					IP:        util.CtxValueString(ctx.Context(), constant.CtxKeyClientIP),
					UserAgent: util.CtxValueString(ctx.Context(), constant.CtxKeyClientUA),
					Reason:    reason,
				})
			}
			if !open {
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
