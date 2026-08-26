// Package handler Client 处理器
package handler

import (
	"context"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	llmproxyport "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ClientHandler 客户端接口处理器
type ClientHandler interface {
	HandleListModels(ctx context.Context, req *dto.ClientModelsReq) (*dto.HTTPResponse[*dto.ClientModelsRsp], error)
}

// ClientDependencies ClientHandler 依赖项
type ClientDependencies struct {
	List llmproxyport.ListClientModelsHandler
}

type clientHandler struct {
	list llmproxyport.ListClientModelsHandler
}

// NewClientHandler 创建客户端接口处理器
func NewClientHandler(deps ClientDependencies) ClientHandler {
	return &clientHandler{list: deps.List}
}

// HandleListModels 返回启用中的模型列表（含能力与限制）
//
//	@receiver h *clientHandler
//	@param ctx context.Context
//	@param req *dto.ClientModelsReq
//	@return *dto.HTTPResponse[*dto.ClientModelsRsp]
//	@return error
func (h *clientHandler) HandleListModels(ctx context.Context, _ *dto.ClientModelsReq) (*dto.HTTPResponse[*dto.ClientModelsRsp], error) {
	return apiutil.WrapHTTPResponse(h.list.Handle(ctx))
}
