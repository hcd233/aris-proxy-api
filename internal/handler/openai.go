package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// OpenAIHandler OpenAI兼容接口处理器
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type OpenAIHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.OpenAIListModelsRsp], error)
	HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
}

type openAIHandler struct {
	svc service.OpenAIService
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func NewOpenAIHandler() OpenAIHandler {
	return &openAIHandler{
		svc: service.NewOpenAIService(),
	}
}

// HandleListModels 获取模型列表
//
//	@receiver h *openAIHandler
//	@param _ context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.ListModelsResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (h *openAIHandler) HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.OpenAIListModelsRsp], error) {
	return util.WrapHTTPResponse(h.svc.ListModels(ctx, req))
}

// HandleChatCompletion 处理聊天补全请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.OpenAIChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (h *openAIHandler) HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	return h.svc.CreateChatCompletion(ctx, req)
}
