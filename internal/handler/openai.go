// Package handler OpenAI兼容接口处理器
package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
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

// OpenAIDependencies OpenAIHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type OpenAIDependencies struct {
	UseCase  port.OpenAIUseCase
	SSEGauge *metrics.SSEGauge
}

type openAIHandler struct {
	uc       port.OpenAIUseCase
	sseGauge *metrics.SSEGauge
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@param deps OpenAIDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewOpenAIHandler(deps OpenAIDependencies) OpenAIHandler {
	return &openAIHandler{
		uc:       deps.UseCase,
		sseGauge: deps.SSEGauge,
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
	return apiutil.WrapHTTPResponse(h.uc.ListModels(ctx))
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
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderOpenAI) },
		func() { h.sseGauge.Dec(constant.SSEProviderOpenAI) },
	)
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
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderOpenAI) },
		func() { h.sseGauge.Dec(constant.SSEProviderOpenAI) },
	)
	return h.uc.CreateResponse(ctx, req)
}
