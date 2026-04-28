// Package usecase LLMProxy 域用例层 - Anthropic 接口
package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	convvo "github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// anthropicInternalErrorBody Anthropic 内部错误响应 body（预序列化）
var anthropicInternalErrorBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  constant.AnthropicInternalErrorBodyType,
	Error: &dto.AnthropicError{Type: constant.AnthropicInternalErrorType, Message: constant.AnthropicInternalErrorMessage},
}))

// AnthropicUseCase Anthropic 兼容接口的全部 UseCase
//
//	@author centonhuang
//	@update 2026-04-22 20:45:00
type AnthropicUseCase interface {
	ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	openAIProxy      transport.OpenAIProxy
	anthropicProxy   transport.AnthropicProxy
}

// NewAnthropicUseCase 构造 Anthropic UseCase
//
//	@param resolver service.EndpointResolver
//	@param modelsQuery ListAnthropicModels
//	@param countTokensQuery CountTokens
//	@param openAIProxy transport.OpenAIProxy
//	@param anthropicProxy transport.AnthropicProxy
//	@return AnthropicUseCase
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	openAIProxy transport.OpenAIProxy,
	anthropicProxy transport.AnthropicProxy,
) AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		openAIProxy:      openAIProxy,
		anthropicProxy:   anthropicProxy,
	}
}

// ListModels 列出 Anthropic 兼容模型（走 Query 侧）
func (u *anthropicUseCase) ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

// CountTokens 调用上游 count_tokens（错误时返回空结果，与旧行为一致）
func (u *anthropicUseCase) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return u.countTokensQuery.Handle(ctx, req)
}

// CreateMessage 处理 /v1/messages
//
//	@receiver u *anthropicUseCase
//	@param ctx context.Context
//	@param req *dto.AnthropicCreateMessageRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), enum.ProviderAnthropic, enum.ProviderOpenAI)
	if err != nil {
		log.Error("[AnthropicUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	exposedModel := req.Body.Model
	upstream := toTransportEndpoint(ep)

	if ep.Provider() == enum.ProviderOpenAI {
		return u.forwardMessageViaOpenAI(ctx, log, req, ep, upstream, exposedModel, stream), nil
	}
	return u.forwardMessageNative(ctx, log, req, ep, upstream, exposedModel, stream), nil
}

// ==================== Anthropic Native ====================

// forwardMessageNative Anthropic 原生协议转发
func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
	if stream {
		return u.forwardMessageNativeStream(ctx, log, req, ep, upstream, exposedModel, body)
	}
	return u.forwardMessageNativeUnary(ctx, log, req, ep, upstream, exposedModel, body)
}

// forwardMessageNativeStream Anthropic 原生流式
func (u *anthropicUseCase) forwardMessageNativeStream(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			modifiedData := transport.ReplaceModelInSSEData(event.Data, exposedModel)
			_, _ = fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event)
			_, _ = fmt.Fprintf(w, constant.SSEDataLineTemplate, modifiedData)
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_ = util.WriteAnthropicMessageStop(w)
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeAnthropicFromMsg(ctx, log, req, anthropicMsg, err, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardMessageNativeUnary Anthropic 原生非流式
func (u *anthropicUseCase) forwardMessageNativeUnary(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, anthropicInternalErrorBody)
			auditFailure(ctx, ep, exposedModel, enum.ProviderAnthropic, totalMs, err)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		u.storeAnthropicFromMsg(ctx, log, req, anthropicMsg, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== Anthropic via OpenAI ====================

// forwardMessageViaOpenAI Anthropic 请求通过 OpenAI 上游转发
func (u *anthropicUseCase) forwardMessageViaOpenAI(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	conv := converter.OpenAIProtocolConverter{}
	openAIReq, err := conv.FromAnthropicRequest(req.Body)
	if err != nil {
		log.Error("[AnthropicUseCase] Failed to convert request to OpenAI format", zap.Error(err))
		return util.SendAnthropicInternalError()
	}
	openAIReq.Model = upstream.Model
	body := lo.Must1(sonic.Marshal(openAIReq))
	body = util.EnsureAssistantMessageReasoningContent(body)

	if stream {
		return u.forwardMessageViaOpenAIStream(ctx, log, req, ep, upstream, exposedModel, body, &conv)
	}
	return u.forwardMessageViaOpenAIUnary(ctx, log, req, ep, upstream, exposedModel, body, &conv)
}

// forwardMessageViaOpenAIStream OpenAI 上游流式 → Anthropic SSE
func (u *anthropicUseCase) forwardMessageViaOpenAIStream(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, body []byte, conv *converter.OpenAIProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		isFirst := true
		tracker := converter.NewSSEContentBlockTracker()

		completion, proxyErr := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			events, convErr := conv.ToAnthropicSSEResponse(chunk, isFirst, exposedModel, tracker)
			if convErr != nil {
				return convErr
			}
			isFirst = false
			for _, event := range events {
				if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockStart {
					firstTokenTime = time.Now()
					firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				_, _ = fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event)
				_, _ = fmt.Fprintf(w, constant.SSEDataLineTemplate, string(event.Data))
				if flushErr := w.Flush(); flushErr != nil {
					return flushErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr == nil {
			_ = util.WriteAnthropicMessageStop(w)
		}

		if proxyErr != nil || completion == nil {
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             ep.AggregateID(),
				Model:               exposedModel,
				UpstreamProvider:    ep.Provider(),
				APIProvider:         enum.ProviderAnthropic,
				FirstTokenLatencyMs: firstTokenLatencyMs,
				StreamDurationMs:    streamDurationMs,
			}
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(proxyErr)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
			return
		}
		anthropicMsg, err := conv.ToAnthropicResponse(completion)
		if err != nil {
			log.Error("[AnthropicUseCase] Failed to convert for storage", zap.Error(err))
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             ep.AggregateID(),
				Model:               exposedModel,
				UpstreamProvider:    ep.Provider(),
				APIProvider:         enum.ProviderAnthropic,
				FirstTokenLatencyMs: firstTokenLatencyMs,
				StreamDurationMs:    streamDurationMs,
			}
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
			return
		}
		u.storeAnthropicFromMsg(ctx, log, req, anthropicMsg, nil, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardMessageViaOpenAIUnary OpenAI 上游非流式 → Anthropic JSON
func (u *anthropicUseCase) forwardMessageViaOpenAIUnary(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, body []byte, conv *converter.OpenAIProtocolConverter) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, anthropicInternalErrorBody)
			auditFailure(ctx, ep, exposedModel, enum.ProviderAnthropic, totalMs, err)
			return
		}
		anthropicMsg, err := conv.ToAnthropicResponse(completion)
		if err != nil {
			log.Error("[AnthropicUseCase] Failed to convert OpenAI response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, anthropicInternalErrorBody)
			auditFailure(ctx, ep, exposedModel, enum.ProviderAnthropic, totalMs, err)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		u.storeAnthropicFromMsg(ctx, log, req, anthropicMsg, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== Anthropic audit/store helpers ====================

func (u *anthropicUseCase) storeAnthropicFromMsg(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	u.storeAnthropicMessages(ctx, log, req, msg, upstreamModel)
}

func (u *anthropicUseCase) storeAnthropicMessages(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage, upstreamModel string) {
	unifiedMessages, unifiedTools, inputTokens, outputTokens, err := u.convertAnthropicRequestMessages(log, req, assistantMsg)
	if err != nil {
		return
	}

	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     util.ExtractAnthropicMetadata(req.Body.Metadata),
	}); err != nil {
		log.Error("[AnthropicUseCase] Failed to submit message store task", zap.Error(err))
	}
}

// convertAnthropicRequestMessages 将 Anthropic 请求消息和响应转换为统一格式
//
//	@receiver u *anthropicUseCase
//	@param log *zap.Logger
//	@param req *dto.AnthropicCreateMessageRequest
//	@param assistantMsg *dto.AnthropicMessage
//	@return []*convvo.UnifiedMessage 统一消息列表
//	@return []*convvo.UnifiedTool 统一工具列表
//	@return int 输入 token 数
//	@return int 输出 token 数
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (u *anthropicUseCase) convertAnthropicRequestMessages(log *zap.Logger, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage) ([]*convvo.UnifiedMessage, []*convvo.UnifiedTool, int, int, error) {
	unifiedMessages := make([]*convvo.UnifiedMessage, 0, len(req.Body.Messages)+1)
	for _, msg := range req.Body.Messages {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			log.Error("[AnthropicUseCase] Failed to convert anthropic message", zap.Error(err))
			return nil, nil, 0, 0, err
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromAnthropicResponse(assistantMsg)
	if err != nil {
		log.Error("[AnthropicUseCase] Failed to convert anthropic response", zap.Error(err))
		return nil, nil, 0, 0, err
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := make([]*convvo.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		unifiedTools = append(unifiedTools, dto.FromAnthropicTool(tool))
	}

	var inputTokens, outputTokens int
	if assistantMsg.Usage != nil {
		inputTokens = assistantMsg.Usage.InputTokens
		outputTokens = assistantMsg.Usage.OutputTokens
	}

	return unifiedMessages, unifiedTools, inputTokens, outputTokens, nil
}
