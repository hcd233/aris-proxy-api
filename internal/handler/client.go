// Package handler Client 处理器
package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	llmproxyport "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ClientHandler 客户端接口处理器
type ClientHandler interface {
	HandleListModels(ctx context.Context, req *dto.ClientModelsReq) (*dto.HTTPResponse[*dto.ClientModelsRsp], error)
	HandleCheckArisClient(ctx context.Context, req *dto.CheckArisClientReq) (*huma.StreamResponse, error)
	HandleInstallScript(ctx context.Context, req *dto.InstallScriptReq) (*huma.StreamResponse, error)
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

// HandleCheckArisClient 校验 API Key；鉴权由 API Key 中间件完成，命中即 204。
func (h *clientHandler) HandleCheckArisClient(
	_ context.Context,
	_ *dto.CheckArisClientReq,
) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(ctx huma.Context) {
		ctx.SetStatus(fiber.StatusNoContent)
	}}, nil
}
