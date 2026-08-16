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

var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: constant.OpenAIInternalErrorMessage, Type: constant.OpenAIInternalErrorType, Code: constant.OpenAIInternalErrorCode},
}))

type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    OpenAIProxyPort
	anthropicProxy AnthropicProxyPort
	taskSubmitter  TaskSubmitter
	triggerChecker TriggerChecker
	tokenMetrics   *metrics.TokenUsageCounter
}

func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	triggerChecker TriggerChecker,
	tokenMetrics *metrics.TokenUsageCounter,
) port.OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
		triggerChecker: triggerChecker,
		tokenMetrics:   tokenMetrics,
	}
}

func (u *openAIUseCase) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	var compatRoute enum.CompatRoute
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found or unsupported for chat completion", zap.String("model", req.Body.Model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(req.Body.Model)
	}

	if matched := u.checkContent(req); len(matched) > 0 {
		_ = u.triggerChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		if denyIDs := u.triggerChecker.DenyIDs(matched); len(denyIDs) > 0 {
			upstreamProtocol := openAIChatRouteUpstreamProtocol(compatRoute)
			words := u.triggerChecker.MatchedWords(denyIDs)
			auditTask := &dto.ModelCallAuditTask{
				Ctx:              util.CopyContextValues(ctx),
				ModelID:          m.ModelID(),
				Endpoint:         ep.Name(),
				UpstreamProtocol: upstreamProtocol,
				APIProtocol:      enum.ProtocolOpenAIChatCompletion,
				ErrorMessage:     fmt.Sprintf(constant.TriggerAuditRemarkTemplate, formatTriggerWords(words)),
			}
			_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
			// 内容拦截：返回协议原生 content_filter 消息（200），替代 403
			return proxyutil.BuildOpenAIChatContentFilter(req.Body.Model, lo.FromPtr(req.Body.Stream)), nil
		}

		if result := u.interceptChatCapture(ctx, req, m, ep, openAIChatRouteUpstreamProtocol(compatRoute), matched, lo.FromPtr(req.Body.Stream)); result != nil {
			return result, nil
		}

		// 同时命中 omit 与 capture 词（capture 词未落在最后一条用户提问中、未短路）时：
		// capture 的上下文保存照常执行，omit 的跳过存储照常生效，请求继续转发——两个逻辑都跑。
		if hit := u.omitAndCaptureChatHit(matched, req); hit != nil {
			submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), openAIChatRouteUpstreamProtocol(compatRoute), enum.ProtocolOpenAIChatCompletion, hit.words)
			u.storeOpenAIChatHistory(ctx, req, m, hit.lastIdx)
		}

		// 全部命中词为 allow：放行转发，但跳过 session/message/tool 存储（audit 正常记录）
		ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
	}

	switch compatRoute {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardChatNative(ctx, req, m, ep, upstream, stream)
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardChatViaAnthropic(ctx, req, m, ep, req.Body.Model)
	default:
		log.Error("[OpenAIUseCase] Unsupported chat compatibility route", zap.String("model", req.Body.Model))
		return nil, proxyutil.SendOpenAIModelNotFoundError(req.Body.Model)
	}
}

func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	model := lo.FromPtr(req.Body.Model)
	var compatRoute enum.CompatRoute
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIResponse, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found or unsupported", zap.String("model", model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}

	if matched := u.checkResponseContent(req); len(matched) > 0 {
		_ = u.triggerChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		if denyIDs := u.triggerChecker.DenyIDs(matched); len(denyIDs) > 0 {
			upstreamProtocol := openAIResponseRouteUpstreamProtocol(compatRoute)
			words := u.triggerChecker.MatchedWords(denyIDs)
			auditTask := &dto.ModelCallAuditTask{
				Ctx:              util.CopyContextValues(ctx),
				ModelID:          m.ModelID(),
				Endpoint:         ep.Name(),
				UpstreamProtocol: upstreamProtocol,
				APIProtocol:      enum.ProtocolOpenAIResponse,
				ErrorMessage:     fmt.Sprintf(constant.TriggerAuditRemarkTemplate, formatTriggerWords(words)),
			}
			_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
			// 内容拦截：返回协议原生 content_filter 消息（200），替代 403
			return proxyutil.BuildOpenAIResponseContentFilter(model, lo.FromPtr(req.Body.Stream)), nil
		}

		if result := u.interceptResponseCapture(ctx, req, m, ep, openAIResponseRouteUpstreamProtocol(compatRoute), matched, lo.FromPtr(req.Body.Stream)); result != nil {
			return result, nil
		}

		// 同时命中 omit 与 capture 词（capture 词未落在最后一条用户提问中、未短路）时：旁路保存上下文。
		if hit := u.omitAndCaptureResponseHit(matched, req); hit != nil {
			submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), openAIResponseRouteUpstreamProtocol(compatRoute), enum.ProtocolOpenAIResponse, hit.words)
			unified := captureResponseHistory(ctx, req, hit.lastIdx)
			tools := dto.FromResponseAPITools(req.Body.Tools)
			submitCaptureStore(ctx, u.taskSubmitter, m.ModelID(), unified, tools, req.Body.Metadata)
		}

		// 全部命中词为 allow：放行转发，但跳过 session/message/tool 存储（audit 正常记录）
		ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
	}

	switch compatRoute {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardResponseNative(ctx, req, m, ep, upstream, stream)
	case enum.CompatRouteViaOpenAIChat:
		return u.forwardResponseViaChat(ctx, req, m, ep)
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardResponseViaAnthropic(ctx, req, m, ep)
	default:
		log.Error("[OpenAIUseCase] Unsupported response compatibility route", zap.String("model", model))
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}
}

func toTransportEndpoint(m *aggregate.Model, ep *aggregate.Endpoint, isAnthropic bool) vo.UpstreamEndpoint {
	baseURL := lo.Ternary(isAnthropic, ep.AnthropicBaseURL(), ep.OpenaiBaseURL())
	return vo.NewUpstreamEndpointFromCredential(m.UpstreamModel(), ep.APIKey(), baseURL)
}
