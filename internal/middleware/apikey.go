package middleware

import (
	"strings"
	"sync/atomic"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// apiKeyIndex apiKey -> name 的反向索引，使用 atomic.Pointer 支持热加载重建
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
var apiKeyIndex atomic.Pointer[map[string]string]

// buildAPIKeyIndex 从当前代理配置构建 apiKey -> name 的反向索引
//
//	@return *map[string]string
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func buildAPIKeyIndex() *map[string]string {
	cfg := proxy.GetLLMProxyConfig()
	index := lo.SliceToMap(lo.Keys(cfg.APIKeys), func(key string) (string, string) {
		return cfg.APIKeys[key], key
	})
	return &index
}

// RebuildAPIKeyIndex 重建 API Key 反向索引，用于配置热加载后刷新
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func RebuildAPIKeyIndex() {
	apiKeyIndex.Store(buildAPIKeyIndex())
	logger.Logger().Info("[APIKeyMiddleware] API key index rebuilt")
}

// APIKeyMiddleware API Key 验证中间件
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func APIKeyMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	// 首次启动时构建索引
	apiKeyIndex.Store(buildAPIKeyIndex())

	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := ctx.Header("Authorization")
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), "Bearer"))
		tokenString = strings.TrimSpace(tokenString)

		if tokenString == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrUnauthorized))
			return
		}

		keyToName := apiKeyIndex.Load()
		if keyToName == nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] API key index not initialized")
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrInternalError))
			return
		}

		name, ok := (*keyToName)[tokenString]
		if !ok {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), constant.ErrUnauthorized))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, name)
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header("User-Agent"))
		next(ctx)
	}
}
