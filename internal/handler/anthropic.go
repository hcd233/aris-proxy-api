package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AnthropicHandler Anthropic兼容接口处理器
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.AnthropicListModelsRsp], error)
	HandleCreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	HandleCountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.HTTPResponse[*dto.AnthropicTokensCount], error)
}

type anthropicHandler struct {
	svc service.AnthropicService
}

// NewAnthropicHandler 创建Anthropic兼容接口处理器
//
//	@return AnthropicHandler
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func NewAnthropicHandler() AnthropicHandler {
	return &anthropicHandler{
		svc: service.NewAnthropicService(),
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
//	@update 2026-03-17 10:00:00
func (h *anthropicHandler) HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.AnthropicListModelsRsp], error) {
	return util.WrapHTTPResponse(h.svc.ListModels(ctx, req))
}

// HandleCreateMessage 处理创建消息请求
//
//	@receiver h *anthropicHandler
//	@param ctx context.Context
//	@param req *dto.AnthropicCreateMessageRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func (h *anthropicHandler) HandleCreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	return h.svc.CreateMessage(ctx, req)
}

// HandleCountTokens 处理Token计数请求
//
//	@receiver h *anthropicHandler
//	@param ctx context.Context
//	@param req *dto.AnthropicCountTokensRequest
//	@return *dto.HTTPResponse[*dto.AnthropicTokensCount]
//	@return error
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func (h *anthropicHandler) HandleCountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.HTTPResponse[*dto.AnthropicTokensCount], error) {
	return util.WrapHTTPResponse(h.svc.CountTokens(ctx, req))
}
