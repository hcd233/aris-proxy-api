// Package handler OpenAI兼容接口处理器
package handler

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// OpenAIHandler OpenAI兼容接口处理器
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAIHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.OpenAIListModelsRsp], error)
	HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	HandleChatCompletionV2(ctx context.Context, req *dto.EmptyReq) (*huma.StreamResponse, error)
	HandleCreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

// OpenAIDependencies OpenAIHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type OpenAIDependencies struct {
	UseCase usecase.OpenAIUseCase
}

type openAIHandler struct {
	uc usecase.OpenAIUseCase
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@param deps OpenAIDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewOpenAIHandler(deps OpenAIDependencies) OpenAIHandler {
	return &openAIHandler{
		uc: deps.UseCase,
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
	return h.uc.CreateChatCompletion(ctx, req)
}

// HandleChatCompletionV2 处理 v2 聊天补全请求
func (h *openAIHandler) HandleChatCompletionV2(ctx context.Context, _ *dto.EmptyReq) (*huma.StreamResponse, error) {
	body := &dto.OpenAIChatCompletionReq{}
	if err := sonic.Unmarshal(util.GetRawRequestBody(ctx), body); err != nil {
		return &huma.StreamResponse{Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(fiber.StatusBadRequest)
			humaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
			rsp, _ := sonic.Marshal(&dto.OpenAIErrorResponse{Error: &dto.OpenAIError{
				Message: constant.OpenAIInvalidRequestBodyMessage,
				Type:    constant.OpenAIInvalidRequestErrorType,
				Code:    constant.OpenAIInternalErrorCode,
			}})
			_, _ = humaCtx.BodyWriter().Write(rsp)
		}}, nil
	}
	return h.uc.CreateChatCompletionV2(ctx, &dto.OpenAIChatCompletionRequest{Body: body})
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
