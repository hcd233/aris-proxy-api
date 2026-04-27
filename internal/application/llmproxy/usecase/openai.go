// Package usecase LLMProxy 域用例层
//
// 负责装配 8 条转发路径（OpenAI ChatCompletion×4 + Response API×4 + Anthropic Messages×4，
// 其中部分共享实现），通过 EndpointResolver + Transport + Converter + Pool 组合完成：
//
//   - 端点解析（含主/回退 provider）
//   - 请求转换（同协议透传 / 跨协议转换 / max_tokens 补丁）
//   - 上游传输（OpenAI / Anthropic proxy）
//   - 响应写入（SSE / JSON / Raw Bytes）
//   - 审计与消息存储（通过 pool 异步化，保留原字节级行为）
//
// 设计原则：**薄封装 + 结构对齐原 service**，最大限度保持字节级兼容；不引入
// 为每种组合单独建 Adapter/Writer/Observer struct 的过度抽象，闭包即可胜任。
//
//	@author centonhuang
//	@update 2026-04-22 20:45:00
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

// openAIInternalErrorBody OpenAI 内部错误响应 body（预序列化，避免重复 marshal）
var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: "Internal server error", Type: "server_error", Code: "internal_error"},
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
	log := logger.WithCtx(ctx)

	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(ep)

	if ep.Provider() == enum.ProviderAnthropic {
		return u.forwardChatViaAnthropic(ctx, log, req, ep, upstream, stream), nil
	}
	return u.forwardChatNative(ctx, log, req, ep, upstream, stream), nil
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

	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(ep)

	if ep.Provider() == enum.ProviderAnthropic {
		return u.forwardResponseViaAnthropic(ctx, log, req, ep, upstream, stream), nil
	}
	return u.forwardResponseNative(ctx, log, req, ep, upstream, stream), nil
}

// ==================== ChatCompletion Native ====================

// forwardChatNative OpenAI 原生协议转发（provider=openai）
func (u *openAIUseCase) forwardChatNative(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	// max_tokens → max_completion_tokens 兼容补丁（OpenAI 特定端点要求）
	if req.Body.MaxTokens != nil {
		req.Body.MaxCompletionTokens, req.Body.MaxTokens = lo.ToPtr(*req.Body.MaxTokens), nil
	}
	body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)

	if stream {
		return u.forwardChatNativeStream(ctx, log, req, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, log, req, ep, upstream, body)
}

// forwardChatNativeStream OpenAI 原生流式：SSE chunks → 客户端
func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		toolCallIDs := make(map[int]string)

		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			util.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
			chunk.Model = req.Body.Model
			chunkData, marshalErr := sonic.Marshal(chunk)
			if marshalErr != nil {
				log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
				return marshalErr
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			_ = w.Flush()
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, err, upstream.Model)

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromOpenAIUsage(usage)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardChatNativeUnary OpenAI 原生非流式：JSON → 客户端
func (u *openAIUseCase) forwardChatNativeUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== ChatCompletion via Anthropic ====================

// forwardChatViaAnthropic OpenAI 请求通过 Anthropic 上游转发
func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromOpenAIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError()
	}
	anthropicReq.Model = upstream.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return u.forwardChatViaAnthropicStream(ctx, log, req, ep, upstream, body, &conv)
	}
	return u.forwardChatViaAnthropicUnary(ctx, log, req, ep, upstream, body, &conv)
}

// forwardChatViaAnthropicStream Anthropic 上游流式 → OpenAI SSE
func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		chunkID := converter.GenerateOpenAIChunkID()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			chunks, convErr := conv.ToOpenAISSEResponse(event, req.Body.Model, chunkID)
			if convErr != nil {
				return convErr
			}
			for _, chunk := range chunks {
				if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
					firstTokenTime = time.Now()
					firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				chunkData, marshalErr := sonic.Marshal(chunk)
				if marshalErr != nil {
					log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
					return marshalErr
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
				if flushErr := w.Flush(); flushErr != nil {
					return flushErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			_ = w.Flush()
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeOpenAIChatFromAnthropicMsg(ctx, log, req, anthropicMsg, err, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardChatViaAnthropicUnary Anthropic 上游非流式 → OpenAI JSON
func (u *openAIUseCase) forwardChatViaAnthropicUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== Response API Native ====================

// forwardResponseNative OpenAI 原生 Response API 转发
func (u *openAIUseCase) forwardResponseNative(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
	if stream {
		return u.forwardResponseNativeStream(ctx, log, req, ep, upstream, body)
	}
	return u.forwardResponseNativeUnary(ctx, log, req, ep, upstream, body)
}

// forwardResponseNativeStream Response API 原生流式
func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var finalResponse *dto.OpenAICreateResponseRsp

		proxyErr := u.openAIProxy.ForwardCreateResponseStream(ctx, upstream, body, func(event string, data []byte) error {
			if firstTokenTime.IsZero() && util.IsResponseAPIDeltaEvent(event) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			if finalResponse == nil && util.IsResponseAPITerminalEvent(event) {
				var ev dto.ResponseStreamTerminalEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Warn("[OpenAIUseCase] Failed to parse response terminal event",
						zap.String("event", event), zap.Error(err))
				} else {
					finalResponse = ev.Response
				}
			}
			replaced := transport.ReplaceModelInSSEData(data, req.Body.Model)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, replaced)
			return w.Flush()
		})

		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIUseCase] Response API stream error", zap.Error(proxyErr))
			util.WriteUpstreamSSEError(log, w, proxyErr)
		}

		u.storeResponseFromRsp(ctx, log, req, finalResponse, proxyErr, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromResponseUsage(finalResponse)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(proxyErr)
		task.SetErrorFromResponseStatus(finalResponse)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardResponseNativeUnary Response API 原生非流式（直写 raw bytes）
func (u *openAIUseCase) forwardResponseNativeUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		respBody, err := u.openAIProxy.ForwardCreateResponse(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}

		replaced := transport.ReplaceModelInBody(respBody, req.Body.Model)
		writer.HumaCtx.SetStatus(fiber.StatusOK)
		writer.HumaCtx.SetHeader("Content-Type", "application/json")
		_, _ = writer.HumaCtx.BodyWriter().Write(replaced)

		var rsp dto.OpenAICreateResponseRsp
		parseErr := sonic.Unmarshal(respBody, &rsp)
		if parseErr != nil {
			log.Warn("[OpenAIUseCase] Failed to parse Response API non-stream body", zap.Error(parseErr))
		} else {
			u.storeResponseFromRsp(ctx, log, req, &rsp, nil, upstream.Model)
		}

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		if parseErr == nil {
			task.SetTokensFromResponseUsage(&rsp)
			task.SetErrorFromResponseStatus(&rsp)
		}
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== Response API via Anthropic ====================

// forwardResponseViaAnthropic Response API 通过 Anthropic 上游转发
func (u *openAIUseCase) forwardResponseViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromResponseAPIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError()
	}
	anthropicReq.Model = upstream.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return u.forwardResponseViaAnthropicStream(ctx, log, req, ep, upstream, body, &conv)
	}
	return u.forwardResponseViaAnthropicUnary(ctx, log, req, ep, upstream, body, &conv)
}

// forwardResponseViaAnthropicStream Anthropic 上游流式 → OpenAI chat chunk（Response API 跨协议变体）
func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		chunkID := converter.GenerateOpenAIChunkID()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			chunks, convErr := conv.ToOpenAISSEResponse(event, req.Body.Model, chunkID)
			if convErr != nil {
				return convErr
			}
			for _, chunk := range chunks {
				if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
					firstTokenTime = time.Now()
					firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				chunkData, marshalErr := sonic.Marshal(chunk)
				if marshalErr != nil {
					log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
					return marshalErr
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
				if flushErr := w.Flush(); flushErr != nil {
					return flushErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			_ = w.Flush()
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeResponseFromAnthropicMsg(ctx, log, req, anthropicMsg, err, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardResponseViaAnthropicUnary Anthropic 上游非流式 → OpenAI chat JSON（Response API 跨协议变体）
func (u *openAIUseCase) forwardResponseViaAnthropicUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeResponseFromAnthropicMsg(ctx, log, req, anthropicMsg, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// ==================== Store Helpers: ChatCompletion 路径 ====================

// storeOpenAIChatFromCompletion 原生 OpenAI 响应 → 消息存储
func (u *openAIUseCase) storeOpenAIChatFromCompletion(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, completion *dto.OpenAIChatCompletion, proxyErr error, upstreamModel string) {
	if proxyErr != nil || completion == nil || len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	u.storeOpenAIChatMessages(ctx, log, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeOpenAIChatFromAnthropicMsg Anthropic 响应先转 OpenAI 再落盘
func (u *openAIUseCase) storeOpenAIChatFromAnthropicMsg(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	conv := converter.AnthropicProtocolConverter{}
	completion, err := conv.ToOpenAIResponse(msg)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert for storage", zap.Error(err))
		return
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	u.storeOpenAIChatMessages(ctx, log, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeOpenAIChatMessages ChatCompletion 存储基元：req.Messages + assistantMsg → UnifiedMessage 列表
func (u *openAIUseCase) storeOpenAIChatMessages(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, assistantMsg *dto.OpenAIChatCompletionMessageParam, upstreamModel string, usage *dto.OpenAICompletionUsage) {
	unifiedMessages, unifiedTools, err := u.convertRequestMessages(log, req)
	if err != nil {
		return
	}

	aiMsg, err := dto.FromOpenAIMessage(assistantMsg)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert ai response message", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	var inputTokens, outputTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
	}

	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit message store task", zap.Error(err))
	}
}

// convertRequestMessages 将 OpenAI 请求消息和工具转换为统一格式
//
//	@receiver u *openAIUseCase
//	@param log *zap.Logger
//	@param req *dto.OpenAIChatCompletionRequest
//	@return []*dto.UnifiedMessage
//	@return []*dto.UnifiedTool
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (u *openAIUseCase) convertRequestMessages(log *zap.Logger, req *dto.OpenAIChatCompletionRequest) ([]*dto.UnifiedMessage, []*dto.UnifiedTool, error) {
	unifiedMessages := make([]*dto.UnifiedMessage, 0, len(req.Body.Messages))
	for _, msg := range req.Body.Messages {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert openai message", zap.Error(err))
			return nil, nil, err
		}
		unifiedMessages = append(unifiedMessages, um)
	}
	unifiedTools := lo.Map(req.Body.Tools, func(tool dto.OpenAIChatCompletionTool, _ int) *dto.UnifiedTool {
		return dto.FromOpenAITool(&tool)
	})
	return unifiedMessages, unifiedTools, nil
}

// ==================== Store Helpers: Response API 路径 ====================

// storeResponseFromRsp Response API 原生响应 → 消息存储
func (u *openAIUseCase) storeResponseFromRsp(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, rsp *dto.OpenAICreateResponseRsp, proxyErr error, upstreamModel string) {
	if proxyErr != nil || rsp == nil {
		return
	}
	switch rsp.Status {
	case "",
		enum.ResponseStatusCompleted,
		enum.ResponseStatusIncomplete:
		// persistable
	default:
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(log, req)
	if !ok {
		return
	}

	outputMsgs, ok := convertResponseOutput(log, rsp)
	if !ok {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if rsp.Usage != nil {
		inputTokens = rsp.Usage.InputTokens
		outputTokens = rsp.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, log, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// convertResponseOutput 将 Response API 响应输出项转换为统一消息格式
//
//	@param log *zap.Logger
//	@param rsp *dto.OpenAICreateResponseRsp
//	@return []*dto.UnifiedMessage 转换后的统一消息列表
//	@return bool 是否转换成功
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func convertResponseOutput(log *zap.Logger, rsp *dto.OpenAICreateResponseRsp) ([]*dto.UnifiedMessage, bool) {
	outputMsgs, err := dto.FromResponseAPIOutputItems(rsp.Output)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert response output items", zap.Error(err))
		return nil, false
	}
	if len(outputMsgs) == 0 {
		return nil, false
	}
	return outputMsgs, true
}

// storeResponseFromAnthropicMsg Response API Anthropic 变体 → 消息存储
func (u *openAIUseCase) storeResponseFromAnthropicMsg(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(log, req)
	if !ok {
		return
	}

	outputMsgs, ok := anthropicResponseContentToUnified(log, msg.Content)
	if !ok {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if msg.Usage != nil {
		inputTokens = msg.Usage.InputTokens
		outputTokens = msg.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, log, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// buildResponseRequestUnifiedMessages Response API 请求 → UnifiedMessage 前置列表
//
// 返回 (messages, ok)：ok=false 表示 input.Items 转换失败；ok=true 时 messages 可能为空。
func buildResponseRequestUnifiedMessages(log *zap.Logger, req *dto.OpenAICreateResponseRequest) ([]*dto.UnifiedMessage, bool) {
	var messages []*dto.UnifiedMessage

	if req.Body.Instructions != nil && *req.Body.Instructions != "" {
		messages = append(messages, &dto.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &dto.UnifiedContent{Text: *req.Body.Instructions},
		})
	}

	if req.Body.Input != nil {
		if len(req.Body.Input.Items) > 0 {
			inputMsgs, err := dto.FromResponseAPIInputItems(req.Body.Input.Items)
			if err != nil {
				log.Error("[OpenAIUseCase] Failed to convert response input items", zap.Error(err))
				return nil, false
			}
			messages = append(messages, inputMsgs...)
		} else if req.Body.Input.Text != "" {
			messages = append(messages, &dto.UnifiedMessage{
				Role:    enum.RoleUser,
				Content: &dto.UnifiedContent{Text: req.Body.Input.Text},
			})
		}
	}

	return messages, true
}

// buildResponseUnifiedTools Response API 请求 tools → UnifiedTool
func buildResponseUnifiedTools(tools []*dto.ResponseTool) []*dto.UnifiedTool {
	result := make([]*dto.UnifiedTool, 0, len(tools))
	for _, tool := range tools {
		if ut := dto.FromResponseAPITool(tool); ut != nil {
			result = append(result, ut)
		}
	}
	return result
}

// anthropicResponseContentToUnified Anthropic content blocks → UnifiedMessage 列表
//
// 任何 tool_use 块 marshal 失败 → 放弃整条响应落盘（避免残缺消息写入）。
func anthropicResponseContentToUnified(log *zap.Logger, blocks []*dto.AnthropicContentBlock) ([]*dto.UnifiedMessage, bool) {
	var messages []*dto.UnifiedMessage
	for _, block := range blocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			messages = append(messages, &dto.UnifiedMessage{
				Role:    enum.RoleAssistant,
				Content: &dto.UnifiedContent{Text: block.Text},
			})
		case enum.AnthropicContentBlockTypeThinking:
			messages = append(messages, &dto.UnifiedMessage{
				Role:             enum.RoleAssistant,
				ReasoningContent: lo.FromPtr(block.Thinking),
			})
		case enum.AnthropicContentBlockTypeToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				log.Error("[OpenAIUseCase] Failed to marshal tool_use input, abort storage to avoid partial conversation",
					zap.String("toolID", block.ID), zap.String("toolName", block.Name), zap.Error(err))
				return nil, false
			}
			messages = append(messages, &dto.UnifiedMessage{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.UnifiedToolCall{{
					ID:   block.ID,
					Name: block.Name,
				}},
				Content: &dto.UnifiedContent{Text: args},
			})
		}
	}
	return messages, true
}

// submitResponseMessageStoreTask Response API 路径统一的消息存储投递
func submitResponseMessageStoreTask(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, upstreamModel string, messages []*dto.UnifiedMessage, inputTokens, outputTokens int) {
	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     messages,
		Tools:        buildResponseUnifiedTools(req.Body.Tools),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit response message store task", zap.Error(err))
	}
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
