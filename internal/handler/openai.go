// Package handler OpenAI兼容接口处理器
package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// OpenAIHandler OpenAI兼容接口处理器
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAIHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.OpenAIListModelsRsp], error)
	HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	HandleCreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

type openAIHandler struct {
	uc usecase.OpenAIUseCase
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func NewOpenAIHandler() OpenAIHandler {
	endpointRepo := repository.NewEndpointRepository()
	resolver := service.NewEndpointResolver(endpointRepo)
	modelsQuery := usecase.NewListOpenAIModels()

	return &openAIHandler{
		uc: usecase.NewOpenAIUseCase(
			resolver,
			modelsQuery,
			transport.NewOpenAIProxy(),
			transport.NewAnthropicProxy(),
		),
	}
}

// HandleListModels 获取模型列表
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.OpenAIListModelsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func (h *openAIHandler) HandleListModels(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.OpenAIListModelsRsp], error) {
	return util.WrapHTTPResponse(h.uc.ListModels(ctx))
}

// HandleChatCompletion 处理聊天补全请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.OpenAIChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func (h *openAIHandler) HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	return h.uc.CreateChatCompletion(ctx, req)
}

// HandleCreateResponse 处理 Response API 请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.OpenAICreateResponseRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func (h *openAIHandler) HandleCreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	return h.uc.CreateResponse(ctx, req)
}
