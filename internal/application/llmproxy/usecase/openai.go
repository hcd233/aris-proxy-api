// Package usecase LLMProxy 域用例层
//
// 负责装配 8 条转发路径（OpenAI ChatCompletion×4 + Response API×4），
// 通过 EndpointResolver + Transport + Converter + Pool 组合完成。
//
//	@author centonhuang
//	@update 2026-04-28 20:00:00
package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// openAIInternalErrorBody OpenAI 内部错误响应 body（预序列化，避免重复 marshal）
var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: constant.OpenAIInternalErrorMessage, Type: constant.OpenAIInternalErrorType, Code: constant.OpenAIInternalErrorCode},
}))

// OpenAIUseCase OpenAI 兼容接口的全部 UseCase（ChatCompletion + Response API + ListModels）
//
//	@author centonhuang
//	@update 2026-04-22 20:45:00
type OpenAIUseCase interface {
	ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    transport.OpenAIProxy
	anthropicProxy transport.AnthropicProxy
}

// NewOpenAIUseCase 构造 OpenAI UseCase
//
//	@param resolver service.EndpointResolver
//	@param modelsQuery ListOpenAIModels
//	@param openAIProxy transport.OpenAIProxy
//	@param anthropicProxy transport.AnthropicProxy
//	@return OpenAIUseCase
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy transport.OpenAIProxy,
	anthropicProxy transport.AnthropicProxy,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
	}
}

// ListModels 列出 OpenAI 兼容模型（走 Query 侧）
//
//	@receiver u *openAIUseCase
//	@param ctx context.Context
//	@return *dto.OpenAIListModelsRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func (u *openAIUseCase) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

// CreateChatCompletion 处理 /v1/chat/completions
//
//	@receiver u *openAIUseCase
//	@param ctx context.Context
//	@param req *dto.OpenAIChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	state := &openAIChatPipelineState{
		Req:    req,
		Log:    logger.WithCtx(ctx),
		Stream: req.Body.Stream != nil && *req.Body.Stream,
	}
	if err := u.buildOpenAIChatPipeline().Execute(ctx, state); err != nil {
		return nil, err
	}
	return state.HTTPResponse, nil
}

// CreateResponse 处理 /v1/responses (Response API)
//
//	@receiver u *openAIUseCase
//	@param ctx context.Context
//	@param req *dto.OpenAICreateResponseRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	state := &openAIResponsePipelineState{
		Req:    req,
		Log:    logger.WithCtx(ctx),
		Stream: req.Body.Stream != nil && *req.Body.Stream,
	}
	if err := u.buildOpenAIResponsePipeline().Execute(ctx, state); err != nil {
		return nil, err
	}
	return state.HTTPResponse, nil
}

// toTransportEndpoint Endpoint 聚合 → transport.UpstreamEndpoint
func toTransportEndpoint(ep *aggregate.Endpoint) transport.UpstreamEndpoint {
	creds := ep.Creds()
	return transport.UpstreamEndpoint{
		Model:   creds.Model(),
		APIKey:  creds.APIKey(),
		BaseURL: creds.BaseURL(),
	}
}
