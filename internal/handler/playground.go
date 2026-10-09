// Package handler Playground 模型调试台处理器
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

// PlaygroundHandler Playground 模型调试台处理器
//
//	@author centonhuang
//	@update 2026-10-09 10:00:00
type PlaygroundHandler interface {
	HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
}

// PlaygroundDependencies PlaygroundHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-10-09 10:00:00
type PlaygroundDependencies struct {
	UseCase  port.OpenAIUseCase
	SSEGauge *metrics.SSEGauge
}

type playgroundHandler struct {
	uc       port.OpenAIUseCase
	sseGauge *metrics.SSEGauge
}

// NewPlaygroundHandler 创建 Playground 处理器
//
//	@param deps PlaygroundDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return PlaygroundHandler
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func NewPlaygroundHandler(deps PlaygroundDependencies) PlaygroundHandler {
	return &playgroundHandler{
		uc:       deps.UseCase,
		sseGauge: deps.SSEGauge,
	}
}

// HandleChatCompletion Playground 调试请求：走 LLM 转发全链路（别名解析、跨协议转换、Guard、触发词），
// 存储分流复用 CtxKeySkipStore（store 路径跳过 session/message/tool 沉淀，审计照常）——
// 调试流量不污染会话数据集，但用量可观测。
//
//	@receiver h *playgroundHandler
//	@param ctx context.Context
//	@param req *dto.OpenAIChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (h *playgroundHandler) HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
	ctx = apiutil.WithStreamLifecycle(ctx,
		func() { h.sseGauge.Inc(constant.SSEProviderOpenAI) }, // ponytail: label 复用 OpenAI，升级路径：SSEProviderPlayground 独立计量
		func() { h.sseGauge.Dec(constant.SSEProviderOpenAI) },
	)
	result, err := h.uc.CreateChatCompletion(ctx, req)
	return apiutil.AdaptProxyResult(ctx, result, err, openAIInternalFallbackBody)
}
