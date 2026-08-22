package middleware

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// InjectRequestMetaMiddleware 将客户端 IP 与 User-Agent 注入请求 context
//
// 供 demo 登录等需要记录访问来源的场景读取；JWT 中间件不注入这两项。
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func InjectRequestMetaMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		fCtx := humafiber.Unwrap(ctx)
		ctx = huma.WithValue(ctx, constant.CtxKeyClientIP, fCtx.IP())
		ctx = huma.WithValue(ctx, constant.CtxKeyClientUA, fCtx.Get(constant.HTTPHeaderUserAgent))
		next(ctx)
	}
}
