// Package handler Anthropic兼容接口处理器
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

// anthropicInternalFallbackBody 是 application 返回非 ProxyError 错误时的 fallback 响应体。
// 正常路径下所有错误都由 usecase 包装为 *port.ProxyError；此 fallback 仅用于防御性兜底。
var anthropicInternalFallbackBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  constant.AnthropicInternalErrorBodyType,
	Error: &dto.AnthropicError{Type: constant.AnthropicInternalErrorType, Message: constant.AnthropicInternalErrorMessage},
}))

// AnthropicHandler Anthropic兼容接口处理器
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.AnthropicListModelsRsp], error)
	HandleCreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	HandleCountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.HTTPResponse[*dto.AnthropicTokensCount], error)
}

// AnthropicDependencies AnthropicHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type AnthropicDependencies struct {
	UseCase  port.AnthropicUseCase
	SSEGauge *metrics.SSEGauge
}

type anthropicHandler struct {
	uc       port.AnthropicUseCase
	sseGauge *metrics.SSEGauge
}

// NewAnthropicHandler 创建Anthropic兼容接口处理器
//
//	@param deps AnthropicDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return AnthropicHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewAnthropicHandler(deps AnthropicDependencies) AnthropicHandler {
	return &anthropicHandler{
		uc:       deps.UseCase,
		sseGauge: deps.SSEGauge,
	}
}

// HandleListModels 获取Anthropic模型列表
//
//	@receiver h *anthropicHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.AnthropicListModelsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func (h *anthropicHandler) HandleListModels(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.AnthropicListModelsRsp], error) {
	return apiutil.WrapHTTPResponse(h.uc.ListModels(ctx))
}

// HandleCreateMessage 处理创建消息请求
//
//	@receiver h *anthropicHandler
//	@param ctx context.Context
//	@param req *dto.AnthropicCreateMessageRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func (h *anthropicHandler) HandleCreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderAnthropic) },
		func() { h.sseGauge.Dec(constant.SSEProviderAnthropic) },
	)
	result, err := h.uc.CreateMessage(ctx, req)
	return apiutil.AdaptProxyResult(ctx, result, err, anthropicInternalFallbackBody)
}

// HandleCountTokens 处理Token计数请求
//
//	@receiver h *anthropicHandler
//	@param ctx context.Context
//	@param req *dto.AnthropicCountTokensRequest
//	@return *dto.HTTPResponse[*dto.AnthropicTokensCount]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 21:00:00
func (h *anthropicHandler) HandleCountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.HTTPResponse[*dto.AnthropicTokensCount], error) {
	return apiutil.WrapHTTPResponse(h.uc.CountTokens(ctx, req))
}
