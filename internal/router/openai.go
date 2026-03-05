package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initOpenAIRouter 初始化OpenAI兼容路由
//
//	@param openaiGroup huma.API
//	@author centonhuang
//	@update 2025-11-12 10:00:00
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
}
