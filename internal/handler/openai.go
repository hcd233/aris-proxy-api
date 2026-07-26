// Package handler OpenAI兼容接口处理器
package handler

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
)

// openAIInternalFallbackBody 是 application 返回非 ProxyError 错误时的 fallback 响应体。
// 正常路径下所有错误都由 usecase 包装为 *port.ProxyError；此 fallback 仅用于防御性兜底。
var openAIInternalFallbackBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: constant.OpenAIInternalErrorMessage, Type: constant.OpenAIInternalErrorType, Code: constant.OpenAIInternalErrorCode},
}))

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
//	@update 2026-07-25 10:00:00
func (h *openAIHandler) HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderOpenAI) },
		func() { h.sseGauge.Dec(constant.SSEProviderOpenAI) },
	)
	result, err := h.uc.CreateChatCompletion(ctx, req)
	return apiutil.AdaptProxyResult(ctx, result, err, openAIInternalFallbackBody)
}

// HandleCreateResponse 处理 Response API 请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.OpenAICreateResponseRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func (h *openAIHandler) HandleCreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderOpenAI) },
		func() { h.sseGauge.Dec(constant.SSEProviderOpenAI) },
	)
	result, err := h.uc.CreateResponse(ctx, req)
	return apiutil.AdaptProxyResult(ctx, result, err, openAIInternalFallbackBody)
}
