// Package handler API Key 处理器
package handler

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// APIKeyHandler API Key 处理器
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type APIKeyHandler interface {
	HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error)
	HandleListAPIKeys(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeysRsp], error)
	HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type apiKeyHandler struct {
	svc service.APIKeyService
}

// NewAPIKeyHandler 创建 API Key 处理器
//
//	@return APIKeyHandler
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func NewAPIKeyHandler() APIKeyHandler {
	return &apiKeyHandler{
		svc: service.NewAPIKeyService(),
	}
}

func (h *apiKeyHandler) HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error) {
	return util.WrapHTTPResponse(h.svc.CreateAPIKey(ctx, req))
}

func (h *apiKeyHandler) HandleListAPIKeys(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeysRsp], error) {
	return util.WrapHTTPResponse(h.svc.ListAPIKeys(ctx))
}

func (h *apiKeyHandler) HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	return util.WrapHTTPResponse(h.svc.DeleteAPIKey(ctx, req))
}
