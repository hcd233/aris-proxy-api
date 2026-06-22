package usecase

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: constant.OpenAIInternalErrorMessage, Type: constant.OpenAIInternalErrorType, Code: constant.OpenAIInternalErrorCode},
}))

type OpenAIUseCase interface {
	ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    OpenAIProxyPort
	anthropicProxy AnthropicProxyPort
	taskSubmitter  TaskSubmitter
	blockedChecker BlockedChecker
	dispatcher     *compression.Dispatcher
}

func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
	dispatcher *compression.Dispatcher,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
		blockedChecker: blockedChecker,
		dispatcher:     dispatcher,
	}
}

func (u *openAIUseCase) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	var compatRoute enum.CompatRoute
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found or unsupported for chat completion", zap.String("model", req.Body.Model), zap.Error(err))
		return proxyutil.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	if matched := u.checkContent(req); len(matched) > 0 {
		_ = u.blockedChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		var upstreamProtocol enum.ProtocolType
		switch compatRoute {
		case enum.CompatRouteNative:
			upstreamProtocol = enum.ProtocolOpenAIChatCompletion
		case enum.CompatRouteViaAnthropicMessage:
			upstreamProtocol = enum.ProtocolAnthropicMessage
		}
		words := u.blockedChecker.MatchedWords(matched)
		auditTask := &dto.ModelCallAuditTask{
			Ctx:              util.CopyContextValues(ctx),
			ModelID:          m.AggregateID(),
			Model:            req.Body.Model,
			Endpoint:         ep.Name(),
			UpstreamProtocol: upstreamProtocol,
			APIProtocol:      enum.ProtocolOpenAIChatCompletion,
			ErrorMessage:     fmt.Sprintf(constant.BlockedAuditRemarkTemplate, formatBlockedWords(words)),
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
		return proxyutil.SendOpenAIContentBlockedError(), nil   //nolint:nilerr // error returned in response body
	}

	switch compatRoute {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardChatNative(ctx, req, m, ep, upstream, stream), nil
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardChatViaAnthropic(ctx, req, m, ep, req.Body.Model), nil
	default:
		log.Error("[OpenAIUseCase] Unsupported chat compatibility route", zap.String("model", req.Body.Model))
		return proxyutil.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}
}

func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	model := lo.FromPtr(req.Body.Model)
	var compatRoute enum.CompatRoute
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIResponse, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found or unsupported", zap.String("model", model), zap.Error(err))
		return proxyutil.SendOpenAIModelNotFoundError(model), nil
	}

	switch compatRoute {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardResponseNative(ctx, req, m, ep, upstream, stream), nil
	case enum.CompatRouteViaOpenAIChat:
		return u.forwardResponseViaChat(ctx, req, m, ep), nil
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardResponseViaAnthropic(ctx, req, m, ep), nil
	default:
		log.Error("[OpenAIUseCase] Unsupported response compatibility route", zap.String("model", model))
		return proxyutil.SendOpenAIModelNotFoundError(model), nil
	}
}

func toTransportEndpoint(m *aggregate.Model, ep *aggregate.Endpoint, isAnthropic bool) vo.UpstreamEndpoint {
	baseURL := lo.Ternary(isAnthropic, ep.AnthropicBaseURL(), ep.OpenaiBaseURL())
	return vo.NewUpstreamEndpointFromCredential(m.ModelName(), ep.APIKey(), baseURL)
}

func (u *openAIUseCase) compressChatMessagesIfNeeded(messages []*dto.OpenAIChatCompletionMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressOpenAIChat(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}

func (u *openAIUseCase) compressAnthropicMessagesIfNeeded(messages []*dto.AnthropicMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressAnthropicMessages(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}

func (u *openAIUseCase) compressResponseItemsIfNeeded(items []*dto.ResponseInputItem) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressOpenAIResponses(items, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}
