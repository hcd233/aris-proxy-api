package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initAnthropicRouter 初始化Anthropic兼容路由
//
//	@param anthropicGroup huma.API
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func initAnthropicRouter(anthropicGroup huma.API, anthropicHandler handler.AnthropicHandler) {
	anthropicGroup.UseMiddleware(middleware.APIKeyMiddleware(), middleware.HeaderPassthroughMiddleware())

	huma.Register(anthropicGroup, huma.Operation{
		OperationID: "anthropicListModels",
		Method:      http.MethodGet,
		Path:        "/models",
		Summary:     "List models",
		Description: "List available Anthropic models.",
		Tags:        []string{"Anthropic"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, anthropicHandler.HandleListModels)

	huma.Register(anthropicGroup, huma.Operation{
		OperationID: "anthropicCreateMessage",
		Method:      http.MethodPost,
		Path:        "/messages",
		Summary:     "Create a Message",
		Description: "Send a structured list of input messages and the model will return the next message in the conversation.",
		Tags:        []string{"Anthropic"},
		Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware("callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM)},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, anthropicHandler.HandleCreateMessage)

	huma.Register(anthropicGroup, huma.Operation{
		OperationID: "anthropicCountTokens",
		Method:      http.MethodPost,
		Path:        "/messages/count_tokens",
		Summary:     "Count Tokens",
		Description: "Count the number of tokens in a Message, including tools, images, and documents, without creating it.",
		Tags:        []string{"Anthropic"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, anthropicHandler.HandleCountTokens)
}
