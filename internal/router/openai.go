package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initOpenAIRouter 初始化OpenAI兼容路由
//
//	@param openaiGroup huma.API
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func initOpenAIRouter(openaiGroup huma.API) {
	openaiHandler := handler.NewOpenAIHandler()

	openaiGroup.UseMiddleware(middleware.APIKeyMiddleware())

	huma.Register(openaiGroup, huma.Operation{
		OperationID: "listModels",
		Method:      http.MethodGet,
		Path:        "/models",
		Summary:     "List models",
		Description: "Lists the currently available models, and provides basic information about each one such as the owner and availability.",
		Tags:        []string{"OpenAI"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, openaiHandler.HandleListModels)

	huma.Register(openaiGroup, huma.Operation{
		OperationID: "createChatCompletion",
		Method:      http.MethodPost,
		Path:        "/chat/completions",
		Summary:     "Create chat completion",
		Description: "Creates a model response for the given chat conversation.",
		Tags:        []string{"OpenAI"},
		Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware("callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM)},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, openaiHandler.HandleChatCompletion)

	huma.Register(openaiGroup, huma.Operation{
		OperationID: "createResponse",
		Method:      http.MethodPost,
		Path:        "/responses",
		Summary:     "Create response",
		Description: "Creates a model response for the given input.",
		Tags:        []string{"OpenAI"},
		Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware("callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM)},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, openaiHandler.HandleCreateResponse)
}
