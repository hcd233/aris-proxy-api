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
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
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
	taskSubmitter  TaskSubmitter
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
	taskSubmitter TaskSubmitter,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
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
	log := logger.WithCtx(ctx)

	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(ep)

	if ep.Provider() == enum.ProviderAnthropic {
		return u.forwardChatViaAnthropic(ctx, req, ep, upstream, stream), nil
	}
	return u.forwardChatNative(ctx, req, ep, upstream, stream), nil
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
	log := logger.WithCtx(ctx)

	model := lo.FromPtr(req.Body.Model)
	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found", zap.String("model", model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(ep)

	if ep.Provider() == enum.ProviderAnthropic {
		return u.forwardResponseViaAnthropic(ctx, req, ep, upstream, stream), nil
	}
	return u.forwardResponseNative(ctx, req, ep, upstream, stream), nil
}

// toTransportEndpoint Endpoint 聚合 → vo.UpstreamEndpoint
func toTransportEndpoint(ep *aggregate.Endpoint) vo.UpstreamEndpoint {
	creds := ep.Creds()
	return vo.NewUpstreamEndpointFromCredential(creds.Model(), creds.APIKey(), creds.BaseURL())
}
