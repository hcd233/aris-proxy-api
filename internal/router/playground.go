package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initPlaygroundRouter 注册 Playground 模型调试台路由（JWT 鉴权，权限 ≥ user）。
//
// 请求复用 OpenAI Chat 契约，走 LLM 转发全链路但不落 session（审计照常）。
// 调用归属到 query apiKeyID 指定的、当前用户名下的 Key：审计按该 Key 记账，
// 请求数与 token 两个令牌桶与 /api/openai/v1 共用同一 Key 维度配额（不另开额度）。
func initPlaygroundRouter(playgroundGroup huma.API, playgroundHandler handler.PlaygroundHandler, db *gorm.DB, cache *redis.Client) {
	huma.Register(playgroundGroup, huma.Operation{
		OperationID:  "playgroundChat",
		Method:       http.MethodPost,
		Path:         "/chat",
		Summary:      "PlaygroundChat",
		Description:  "Debug model routing via the LLM proxy chain (audit-only, no session storage)",
		Tags:         []string{constant.TagPlayground},
		MaxBodyBytes: constant.MaxLLMProxyBodyBytes,
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("playgroundChat", enum.PermissionUser),
			middleware.PlaygroundAPIKeyMiddleware(repository.NewAPIKeyRepository(db)),
			middleware.TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM),
			middleware.TokenBucketTokenRateLimiterMiddleware(cache, "callProxyLLMToken", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLMToken, constant.LimitCallProxyLLMToken),
		},
	}, playgroundHandler.HandleChatCompletion)
}
