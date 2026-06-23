package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func (u *openAIUseCase) forwardChatNative(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromOpenAIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat request to anthropic", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	if stream {
		return u.forwardChatViaAnthropicStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardChatViaAnthropicUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		toolCallIDs := make(map[int]string)

		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && proxyutil.HasNonEmptyDelta(chunk) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			proxyutil.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
			chunk.Model = req.Body.Model
			chunkData, marshalErr := sonic.Marshal(chunk)
			if marshalErr != nil {
				log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
				return marshalErr
			}
			if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, chunkData); writeErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE chunk", zap.Error(writeErr))
			}
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			if _, doneErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, constant.SSEDoneSignal); doneErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE done signal", zap.Error(doneErr))
			}
			if flushErr := w.Flush(); flushErr != nil {
				log.Debug("[OpenAIUseCase] Failed to flush SSE writer", zap.Error(flushErr))
			}
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model)

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		task := newAuditTask(ctx, m, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, enum.ProtocolOpenAIChatCompletion, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		task.SetTokensFromOpenAIUsage(usage)
		if usage != nil {
			reportTokenUsage(ctx, usage.InputOutputTokens())
		}
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit
	})
}

func (u *openAIUseCase) forwardChatNativeUnary(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(ctx, m, u.taskSubmitter, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, upstream.Model)

		task := newAuditTask(ctx, m, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, enum.ProtocolOpenAIChatCompletion, totalMs)
		task.UpstreamStatusCode = fiber.StatusOK
		task.SetTokensFromOpenAIUsage(completion.Usage)
		if completion.Usage != nil {
			reportTokenUsage(ctx, completion.Usage.InputOutputTokens())
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}

func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	return apiutil.WrapStreamResponse(u.forwardChatViaAnthropicStreamBody(ctx, req, m, upstream, exposedModel, endpoint, body))
}

func (u *openAIUseCase) forwardChatViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) func(w *bufio.Writer) {
	conv := &converter.AnthropicProtocolConverter{}
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	return func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var allChunks []*dto.OpenAIChatCompletionChunk

		onEvent := u.buildOpenAIChatStreamCallback(conv, w, chunkID, exposedModel, startTime, &firstTokenTime, &firstTokenLatencyMs, &allChunks)
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, onEvent)
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		u.finalizeOpenAIChatStream(ctx, w, err)
		completion, _ := proxyutil.ConcatChatCompletionChunks(allChunks) //nolint:errcheck // store even if concat fails
		if completion != nil {
			completion.Model = exposedModel
		}
		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model)
		task := newAuditTask(ctx, m, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		if anthropicMsg != nil && anthropicMsg.Usage != nil {
			reportTokenUsage(ctx, anthropicMsg.Usage.InputOutputTokens())
		}
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	}
}

func (u *openAIUseCase) buildOpenAIChatStreamCallback(conv *converter.AnthropicProtocolConverter, w *bufio.Writer, chunkID, exposedModel string, startTime time.Time, firstTokenTime *time.Time, firstTokenLatencyMs *int64, allChunks *[]*dto.OpenAIChatCompletionChunk) func(dto.AnthropicSSEEvent) error {
	return func(event dto.AnthropicSSEEvent) error {
		if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			*firstTokenTime = time.Now()
			*firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
		}
		chunks, convErr := conv.ToOpenAISSEResponse(event, exposedModel, chunkID)
		if convErr != nil {
			return convErr
		}
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			*allChunks = append(*allChunks, chunk)
			chunkData, marshalErr := sonic.Marshal(chunk)
			if marshalErr != nil {
				return marshalErr
			}
			fmt.Fprintf(w, constant.SSEDataFrameTemplate, chunkData) //nolint:errcheck // best-effort write
		}
		return w.Flush()
	}
}

func (u *openAIUseCase) finalizeOpenAIChatStream(ctx context.Context, w *bufio.Writer, err error) {
	if err != nil {
		proxyutil.WriteUpstreamSSEError(ctx, w, err)
		return
	}
	fmt.Fprintf(w, constant.SSEDataFrameTemplate, constant.SSEDoneSignal) //nolint:errcheck // best-effort write
	_ = w.Flush()                                                         //nolint:errcheck // flush best effort on stream close
}

func (u *openAIUseCase) forwardChatViaAnthropicUnary(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, totalMs, err)
			return
		}
		completion, convErr := conv.ToOpenAIResponse(anthropicMsg)
		if convErr != nil {
			logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert anthropic message to chat completion", zap.Error(convErr))
			writer.WriteJSON(openAIInternalErrorBody)
			return
		}
		completion.Model = exposedModel
		writer.WriteJSON(completion)
		u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, upstream.Model)
		task := newAuditTask(ctx, m, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, totalMs)
		task.UpstreamStatusCode = fiber.StatusOK
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		if anthropicMsg != nil && anthropicMsg.Usage != nil {
			reportTokenUsage(ctx, anthropicMsg.Usage.InputOutputTokens())
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}
