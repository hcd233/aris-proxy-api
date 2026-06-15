package usecase

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var anthropicInternalErrorBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  constant.AnthropicInternalErrorBodyType,
	Error: &dto.AnthropicError{Type: constant.AnthropicInternalErrorType, Message: constant.AnthropicInternalErrorMessage},
}))

type AnthropicUseCase interface {
	ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	anthropicProxy   AnthropicProxyPort
	openAIProxy      OpenAIProxyPort
	taskSubmitter    TaskSubmitter
	blockedChecker   BlockedChecker
}

func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	anthropicProxy AnthropicProxyPort,
	openAIProxy OpenAIProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
) AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		anthropicProxy:   anthropicProxy,
		openAIProxy:      openAIProxy,
		taskSubmitter:    taskSubmitter,
		blockedChecker:   blockedChecker,
	}
}

func (u *anthropicUseCase) ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *anthropicUseCase) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return u.countTokensQuery.Handle(ctx, req)
}

func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	var compatRoute enum.CompatRoute
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIAnthropicMessage, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[AnthropicUseCase] Model not found or unsupported for messages API", zap.String("model", req.Body.Model), zap.Error(err))
		return proxyutil.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	if matched := u.checkContent(req); len(matched) > 0 {
		_ = u.blockedChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		var upstreamProtocol enum.ProtocolType
		switch compatRoute {
		case enum.CompatRouteNative:
			upstreamProtocol = enum.ProtocolAnthropicMessage
		case enum.CompatRouteViaOpenAIChat:
			upstreamProtocol = enum.ProtocolOpenAIChatCompletion
		}
		words := u.blockedChecker.MatchedWords(matched)
		auditTask := &dto.ModelCallAuditTask{
			Ctx:              util.CopyContextValues(ctx),
			ModelID:          m.AggregateID(),
			Model:            req.Body.Model,
			Endpoint:         ep.Name(),
			UpstreamProtocol: upstreamProtocol,
			APIProtocol:      enum.ProtocolAnthropicMessage,
			ErrorMessage:     fmt.Sprintf(constant.BlockedAuditRemarkTemplate, formatBlockedWords(words)),
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask)  //nolint:errcheck // best-effort audit
		return proxyutil.SendAnthropicContentBlockedError(), nil //nolint:nilerr // error returned in response body
	}

	exposedModel := req.Body.Model
	switch compatRoute {
	case enum.CompatRouteNative:
		stream := req.Body.Stream != nil && *req.Body.Stream
		upstream := toTransportEndpoint(m, ep, true)
		return u.forwardMessageNative(ctx, req, m, ep, upstream, exposedModel, stream), nil
	case enum.CompatRouteViaOpenAIChat:
		return u.forwardMessageViaChat(ctx, req, m, ep, exposedModel), nil
	default:
		log.Error("[AnthropicUseCase] Unsupported messages compatibility route", zap.String("model", req.Body.Model))
		return proxyutil.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}
}
