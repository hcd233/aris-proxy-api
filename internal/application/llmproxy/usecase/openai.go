package usecase

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
	affinity       service.EndpointAffinity
}

func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	triggerChecker TriggerChecker,
	tokenMetrics *metrics.TokenUsageCounter,
	affinity service.EndpointAffinity,
) port.OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
		triggerChecker: triggerChecker,
		tokenMetrics:   tokenMetrics,
		affinity:       affinity,
	}
}

func (u *openAIUseCase) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

// CreateDecision 处理 OpenAI Decision API 请求（native-only，无流式形态）。
func (u *openAIUseCase) CreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	model := req.Body.Model
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	// Decision 无多轮文本，亲和仅会话头维度（指纹退化为空）
	affKey, _ := AffinityKey(ctx, model, "")
	cands, err := u.resolver.ResolveCandidatesWithAffinity(ctx, userID, vo.EndpointAlias(model), affKey, func(ep *aggregate.Endpoint) bool {
		return SelectCompatRoute(enum.ProxyAPIOpenAIDecision, ep) != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Decision API model not found or unsupported", zap.String("model", model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}
	ep, m := cands[0].Endpoint, cands[0].Model

	if matched := u.checkDecisionContent(req); len(matched) > 0 {
		_ = u.triggerChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		if denyIDs := u.triggerChecker.DenyIDs(matched); len(denyIDs) > 0 {
			words := u.triggerChecker.MatchedWords(denyIDs)
			auditTask := &dto.ModelCallAuditTask{
				Ctx:              util.CopyContextValues(ctx),
				ModelID:          m.ModelID(),
				Endpoint:         ep.Name(),
				UpstreamProtocol: enum.ProtocolOpenAIDecision,
				APIProtocol:      enum.ProtocolOpenAIDecision,
				ErrorMessage:     fmt.Sprintf(constant.TriggerAuditRemarkTemplate, formatTriggerWords(words)),
			}
			_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
			return proxyutil.BuildDecisionRefusalBody(model, req.Body.Questions), nil
		}

		// capture 短路未实现：Decision 的 input 是待评估文本而非多轮对话，
		// 不存在「最后一条用户提问」概念（见设计文档 §2.2）。
		if len(u.triggerChecker.OmitIDs(matched)) > 0 {
			ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
		}
	}

	return runWithFallback(ctx, constant.ModuleOpenAIUseCase, model, cands, func(actx context.Context, cand service.Candidate) (port.Result, error) {
		result, ferr := u.forwardDecision(actx, req, cand.Model, cand.Endpoint)
		if ferr == nil {
			rememberAffinity(actx, u.affinity, userID, model, affKey, cand.Endpoint.AggregateID())
		}
		return result, ferr
	})
}

// forwardDecision 向单个候选端点转发 Decision 请求（单端点单次尝试）。
func (u *openAIUseCase) forwardDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest, m *aggregate.Model, ep *aggregate.Endpoint) (port.Result, error) {
	model := req.Body.Model
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIDecisionBodyForModel(req.Body, upstream.Model)

	startTime := time.Now()
	respBody, err := u.openAIProxy.ForwardCreateDecision(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, model, ep.Name(), enum.ProtocolOpenAIDecision, totalMs, err)
		return nil, ProxyErrorFromUpstream(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}

	replaced := proxyutil.ReplaceModelInBody(respBody, model)
	headers := buildPassthroughHeaders(ctx)
	headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON

	out := callOutcome{
		model:               m,
		endpoint:            ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIDecision,
		apiProtocol:         enum.ProtocolOpenAIDecision,
		firstTokenLatencyMs: totalMs,
		successStatus:       true,
	}
	var rsp dto.OpenAIDecisionRsp
	if parseErr := sonic.Unmarshal(replaced, &rsp); parseErr != nil {
		logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to parse Decision API response body", zap.Error(parseErr))
	} else {
		u.storeDecisionSession(ctx, req, &rsp, m.ModelID())
		out.usage = decisionTokenUsage{&rsp}
	}
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, out)

	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       replaced,
		Protocol:   enum.ProtocolKindOpenAI,
	}, nil
}

func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	var compatRoute enum.CompatRoute
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	affKey, _ := AffinityKey(ctx, req.Body.Model, firstUserTextOpenAIChat(req.Body.Messages))
	cands, err := u.resolver.ResolveCandidatesWithAffinity(ctx, userID, vo.EndpointAlias(req.Body.Model), affKey, func(ep *aggregate.Endpoint) bool {
		return SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep) != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found or unsupported for chat completion", zap.String("model", req.Body.Model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(req.Body.Model)
	}
	ep, m := cands[0].Endpoint, cands[0].Model
	compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep)

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

		if len(u.triggerChecker.OmitIDs(matched)) > 0 {
			ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
		}
	}

	return runWithFallback(ctx, constant.ModuleOpenAIUseCase, req.Body.Model, cands, func(actx context.Context, cand service.Candidate) (port.Result, error) {
		result, ferr := u.dispatchChat(actx, req, cand.Model, cand.Endpoint)
		if ferr == nil {
			rememberAffinity(actx, u.affinity, userID, req.Body.Model, affKey, cand.Endpoint.AggregateID())
		}
		return result, ferr
	})
}

// dispatchChat 按候选端点的兼容路由分发 chat 转发（单端点单次尝试）。
func (u *openAIUseCase) dispatchChat(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint) (port.Result, error) {
	switch SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep) {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardChatNative(ctx, req, m, ep, upstream, stream)
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardChatViaAnthropic(ctx, req, m, ep, req.Body.Model)
	default:
		return nil, proxyutil.SendOpenAIModelNotFoundError(req.Body.Model)
	}
}

func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	model := lo.FromPtr(req.Body.Model)
	var compatRoute enum.CompatRoute
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	affKey, _ := AffinityKey(ctx, model, firstUserTextResponse(req.Body.Input))
	cands, err := u.resolver.ResolveCandidatesWithAffinity(ctx, userID, vo.EndpointAlias(model), affKey, func(ep *aggregate.Endpoint) bool {
		return SelectCompatRoute(enum.ProxyAPIOpenAIResponse, ep) != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found or unsupported", zap.String("model", model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}
	ep, m := cands[0].Endpoint, cands[0].Model
	compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIResponse, ep)

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

		if len(u.triggerChecker.OmitIDs(matched)) > 0 {
			ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
		}
	}

	return runWithFallback(ctx, constant.ModuleOpenAIUseCase, model, cands, func(actx context.Context, cand service.Candidate) (port.Result, error) {
		result, ferr := u.dispatchResponse(actx, req, cand.Model, cand.Endpoint)
		if ferr == nil {
			rememberAffinity(actx, u.affinity, userID, model, affKey, cand.Endpoint.AggregateID())
		}
		return result, ferr
	})
}

// dispatchResponse 按候选端点的兼容路由分发 response 转发（单端点单次尝试）。
func (u *openAIUseCase) dispatchResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint) (port.Result, error) {
	model := lo.FromPtr(req.Body.Model)
	switch SelectCompatRoute(enum.ProxyAPIOpenAIResponse, ep) {
	case enum.CompatRouteNative:
		stream := lo.FromPtr(req.Body.Stream)
		upstream := toTransportEndpoint(m, ep, false)
		return u.forwardResponseNative(ctx, req, m, ep, upstream, stream)
	case enum.CompatRouteViaOpenAIChat:
		return u.forwardResponseViaChat(ctx, req, m, ep)
	case enum.CompatRouteViaAnthropicMessage:
		return u.forwardResponseViaAnthropic(ctx, req, m, ep)
	default:
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}
}

func toTransportEndpoint(m *aggregate.Model, ep *aggregate.Endpoint, isAnthropic bool) vo.UpstreamEndpoint {
	baseURL := lo.Ternary(isAnthropic, ep.AnthropicBaseURL(), ep.OpenaiBaseURL())
	return vo.NewUpstreamEndpointFromCredential(m.UpstreamModel(), ep.APIKey(), baseURL)
}
