package usecase

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

var anthropicInternalErrorBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  constant.AnthropicInternalErrorBodyType,
	Error: &dto.AnthropicError{Type: constant.AnthropicInternalErrorType, Message: constant.AnthropicInternalErrorMessage},
}))

type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	anthropicProxy   AnthropicProxyPort
	openAIProxy      OpenAIProxyPort
	taskSubmitter    TaskSubmitter
	triggerChecker   TriggerChecker
	tokenMetrics     *metrics.TokenUsageCounter
}

func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	anthropicProxy AnthropicProxyPort,
	openAIProxy OpenAIProxyPort,
	taskSubmitter TaskSubmitter,
	triggerChecker TriggerChecker,
	tokenMetrics *metrics.TokenUsageCounter,
) port.AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		anthropicProxy:   anthropicProxy,
		openAIProxy:      openAIProxy,
		taskSubmitter:    taskSubmitter,
		triggerChecker:   triggerChecker,
		tokenMetrics:     tokenMetrics,
	}
}

func (u *anthropicUseCase) ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *anthropicUseCase) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return u.countTokensQuery.Handle(ctx, req)
}

func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	var compatRoute enum.CompatRoute
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	ep, m, err := u.resolver.Resolve(ctx, userID, vo.EndpointAlias(req.Body.Model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIAnthropicMessage, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[AnthropicUseCase] Model not found or unsupported for messages API", zap.String("model", req.Body.Model), zap.Error(err))
		return nil, proxyutil.SendAnthropicModelNotFoundError(req.Body.Model)
	}

	if matched := u.checkContent(req); len(matched) > 0 {
		_ = u.triggerChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		if denyIDs := u.triggerChecker.DenyIDs(matched); len(denyIDs) > 0 {
			upstreamProtocol := anthropicRouteUpstreamProtocol(compatRoute)
			words := u.triggerChecker.MatchedWords(denyIDs)
			auditTask := &dto.ModelCallAuditTask{
				Ctx:              util.CopyContextValues(ctx),
				ModelID:          m.ModelID(),
				Endpoint:         ep.Name(),
				UpstreamProtocol: upstreamProtocol,
				APIProtocol:      enum.ProtocolAnthropicMessage,
				ErrorMessage:     fmt.Sprintf(constant.TriggerAuditRemarkTemplate, formatTriggerWords(words)),
			}
			_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
			// 内容拦截：返回协议原生 refusal 消息（200），替代 403
			return proxyutil.BuildAnthropicContentFilter(req.Body.Model, req.Body.Stream != nil && *req.Body.Stream), nil
		}

		if result := u.interceptAnthropicCapture(ctx, req, m, ep, anthropicRouteUpstreamProtocol(compatRoute), matched, req.Body.Stream != nil && *req.Body.Stream); result != nil {
			return result, nil
		}

		// 同时命中 omit 与 capture 词（capture 词未落在最后一条用户提问中、未短路）时：旁路保存上下文。
		if hit := u.omitAndCaptureMessageHit(matched, req); hit != nil {
			submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), anthropicRouteUpstreamProtocol(compatRoute), enum.ProtocolAnthropicMessage, hit.words)
			u.storeAnthropicHistory(ctx, req, m, hit.lastIdx)
		}

		if len(u.triggerChecker.OmitIDs(matched)) > 0 {
			ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
		}
	}

	exposedModel := req.Body.Model
	switch compatRoute {
	case enum.CompatRouteNative:
		stream := req.Body.Stream != nil && *req.Body.Stream
		upstream := toTransportEndpoint(m, ep, true)
		return u.forwardMessageNative(ctx, req, m, ep, upstream, exposedModel, stream)
	case enum.CompatRouteViaOpenAIChat:
		return u.forwardMessageViaChat(ctx, req, m, ep, exposedModel)
	default:
		log.Error("[AnthropicUseCase] Unsupported messages compatibility route", zap.String("model", req.Body.Model))
		return nil, proxyutil.SendAnthropicModelNotFoundError(req.Body.Model)
	}
}
