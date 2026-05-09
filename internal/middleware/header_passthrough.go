package middleware

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// passthroughExcludedHeaders 不透传到上游的请求头（使用 http.CanonicalHeaderKey 格式）
var passthroughExcludedHeaders = map[string]struct{}{
	constant.HTTPHeaderContentType:        {},
	constant.HTTPHeaderContentLength:      {},
	constant.HTTPHeaderAuthorization:      {},
	constant.HTTPHeaderAcceptEncoding:     {},
	constant.HTTPHeaderAPIKey:             {},
	constant.HTTPHeaderAnthropicVersion:   {},
	constant.HTTPHeaderHost:               {},
	constant.HTTPHeaderConnection:         {},
	constant.HTTPHeaderTransferEncoding:   {},
	constant.HTTPHeaderUpgrade:            {},
	constant.HTTPHeaderProxyAuthorization: {},
	constant.HTTPHeaderProxyAuthenticate:  {},
	constant.HTTPHeaderTE:                 {},
	constant.HTTPHeaderTrailer:            {},
	constant.HTTPHeaderTraceID:            {},
}

// HeaderPassthroughMiddleware 透传请求头到上游的中间件
//
// 从客户端请求头中排除代理自身管理的头后，存入 context，
// 供传输层在构建上游请求时使用。
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func HeaderPassthroughMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		passthroughHeaders := make(map[string]string, 8)
		ctx.EachHeader(func(name, value string) {
			canonical := http.CanonicalHeaderKey(name)
			if _, excluded := passthroughExcludedHeaders[canonical]; !excluded {
				passthroughHeaders[canonical] = value
			}
		})
		ctx = huma.WithValue(ctx, constant.CtxKeyPassthroughHeaders, passthroughHeaders)
		ctx = huma.WithValue(ctx, constant.CtxKeyPassthroughResponseHeaders, make(map[string]string, 4))
		next(ctx)
	}
}
