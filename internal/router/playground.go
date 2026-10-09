package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initPlaygroundRouter 注册 Playground 模型调试台路由（JWT 鉴权，权限 ≥ user）。
//
// 请求复用 OpenAI Chat 契约，走 LLM 转发全链路但不落 session（审计照常）。
// 限流按 userID 维度（playground 无 API Key）。
func initPlaygroundRouter(playgroundGroup huma.API, playgroundHandler handler.PlaygroundHandler, cache *redis.Client) {
	playgroundGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "playgroundChat", constant.CtxKeyUserID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM,
	))

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
		},
	}, playgroundHandler.HandleChatCompletion)
}
