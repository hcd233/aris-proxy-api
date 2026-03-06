package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// APIKeyMiddleware API Key 验证中间件
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-03-05 18:00:00
func APIKeyMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	// 启动时构建 apiKey -> name 的反向索引
	cfg := proxy.GetLLMProxyConfig()
	keyToName := lo.SliceToMap(lo.Keys(cfg.APIKeys), func(key string) (string, string) {
		return cfg.APIKeys[key], key
	})

	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := ctx.Header("Authorization")
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), "Bearer"))
		tokenString = strings.TrimSpace(tokenString)

		if tokenString == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrUnauthorized))
			return
		}

		name, ok := keyToName[tokenString]
		if !ok {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrUnauthorized))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, name)
		next(ctx)
	}
}
