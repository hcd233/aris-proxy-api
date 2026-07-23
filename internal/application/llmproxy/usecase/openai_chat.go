package usecase

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
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
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, openAIInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		timer := newStreamTimer()
		toolCallIDs := make(map[int]string)

		completion, err := u.openAIProxy.ReadChatCompletionStream(ctx, stream, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if proxyutil.HasNonEmptyDelta(chunk) {
				timer.markFirstToken()
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
		timer.finish()
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

		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, m.Alias().String())

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        req.Body.Model,
			endpoint:            ep.Name(),
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolOpenAIChatCompletion,
			firstTokenLatencyMs: timer.firstLatencyMs,
			streamDurationMs:    timer.durationMs,
			usage:               openAITokenUsage{usage},
			err:                 err,
		})
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

		u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, m.Alias().String())

		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        req.Body.Model,
			endpoint:            ep.Name(),
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolOpenAIChatCompletion,
			firstTokenLatencyMs: totalMs,
			usage:               openAITokenUsage{completion.Usage},
			successStatus:       true,
		})
	})
}

func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, openAIInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, u.forwardChatViaAnthropicStreamBody(ctx, req, m, stream, exposedModel, endpoint))
}

func (u *openAIUseCase) forwardChatViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, stream io.ReadCloser, exposedModel, endpoint string) func(w *bufio.Writer) {
	conv := &converter.AnthropicProtocolConverter{}
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	return func(w *bufio.Writer) {
		timer := newStreamTimer()
		var allChunks []*dto.OpenAIChatCompletionChunk

		onEvent := u.buildOpenAIChatStreamCallback(conv, w, chunkID, exposedModel, timer, &allChunks)
		anthropicMsg, err := u.anthropicProxy.ReadCreateMessageStream(ctx, stream, onEvent)
		timer.finish()
		u.finalizeOpenAIChatStream(ctx, w, err)
		completion, _ := proxyutil.ConcatChatCompletionChunks(allChunks) //nolint:errcheck // store even if concat fails
		if completion != nil {
			completion.Model = exposedModel
		}
		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolAnthropicMessage,
			apiProtocol:         enum.ProtocolOpenAIChatCompletion,
			firstTokenLatencyMs: timer.firstLatencyMs,
			streamDurationMs:    timer.durationMs,
			usage:               anthropicTokenUsage{anthropicMsg},
			err:                 err,
		})
	}
}

func (u *openAIUseCase) buildOpenAIChatStreamCallback(conv *converter.AnthropicProtocolConverter, w *bufio.Writer, chunkID, exposedModel string, timer *streamTimer, allChunks *[]*dto.OpenAIChatCompletionChunk) func(dto.AnthropicSSEEvent) error {
	return func(event dto.AnthropicSSEEvent) error {
		if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			timer.markFirstToken()
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
		u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolAnthropicMessage,
			apiProtocol:         enum.ProtocolOpenAIChatCompletion,
			firstTokenLatencyMs: totalMs,
			usage:               anthropicTokenUsage{anthropicMsg},
			successStatus:       true,
		})
	})
}
