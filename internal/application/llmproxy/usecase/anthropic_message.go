package usecase

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

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

func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardMessageNativeStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardMessageNativeUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *anthropicUseCase) forwardMessageViaChat(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	chatReq, convErr := conv.FromAnthropicRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert anthropic request to chat", zap.Error(convErr))
		return proxyutil.SendAnthropicModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	if stream {
		return u.forwardMessageViaChatStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardMessageViaChatUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *anthropicUseCase) forwardMessageNativeStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, anthropicInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		timer := newStreamTimer()

		anthropicMsg, err := u.anthropicProxy.ReadCreateMessageStream(ctx, stream, func(event dto.AnthropicSSEEvent) error {
			if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				timer.markFirstToken()
			}
			modifiedData := proxyutil.ReplaceModelInSSEData(event.Data, exposedModel)
			if _, writeErr := fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event); writeErr != nil {
				log.Debug("[AnthropicUseCase] Failed to write SSE event line", zap.Error(writeErr))
			}
			if _, dataErr := fmt.Fprintf(w, constant.SSEDataLineTemplate, modifiedData); dataErr != nil {
				log.Debug("[AnthropicUseCase] Failed to write SSE data line", zap.Error(dataErr))
			}
			return w.Flush()
		})
		timer.finish()
		if err == nil {
			_ = proxyutil.WriteAnthropicMessageStop(w) //nolint:errcheck // best-effort write // flush errors are not actionable at stream end
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, m.Alias().String())

		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolAnthropicMessage,
			apiProtocol:         enum.ProtocolAnthropicMessage,
			firstTokenLatencyMs: timer.firstLatencyMs,
			streamDurationMs:    timer.durationMs,
			usage:               anthropicTokenUsage{anthropicMsg},
			err:                 err,
		})
	})
}

func (u *anthropicUseCase) forwardMessageNativeUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
			auditFailure(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, totalMs, err)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, m.Alias().String())

		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolAnthropicMessage,
			apiProtocol:         enum.ProtocolAnthropicMessage,
			firstTokenLatencyMs: totalMs,
			usage:               anthropicTokenUsage{anthropicMsg},
			successStatus:       true,
		})
	})
}

func (u *anthropicUseCase) forwardMessageViaChatStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolAnthropicMessage, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, anthropicInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, u.forwardMessageViaChatStreamBody(ctx, req, m, stream, exposedModel, endpoint))
}

func (u *anthropicUseCase) forwardMessageViaChatStreamBody(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, stream io.ReadCloser, exposedModel, endpoint string) func(w *bufio.Writer) {
	conv := &converter.OpenAIProtocolConverter{}
	tracker := converter.NewSSEContentBlockTracker()
	return func(w *bufio.Writer) {
		timer := newStreamTimer()
		isFirst := true
		onChunk := u.buildAnthropicChatStreamCallback(conv, tracker, w, exposedModel, timer, &isFirst)
		completion, err := u.openAIProxy.ReadChatCompletionStream(ctx, stream, onChunk)
		timer.finish()
		anthropicMsg := u.finalizeAnthropicChatStream(ctx, conv, w, completion, err, exposedModel)
		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, m.Alias().String())

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolAnthropicMessage,
			firstTokenLatencyMs: timer.firstLatencyMs,
			streamDurationMs:    timer.durationMs,
			usage:               openAITokenUsage{usage},
			err:                 err,
		})
	}
}

func (u *anthropicUseCase) buildAnthropicChatStreamCallback(conv *converter.OpenAIProtocolConverter, tracker *converter.SSEContentBlockTracker, w *bufio.Writer, exposedModel string, timer *streamTimer, isFirst *bool) func(*dto.OpenAIChatCompletionChunk) error {
	return func(chunk *dto.OpenAIChatCompletionChunk) error {
		if proxyutil.HasNonEmptyDelta(chunk) {
			timer.markFirstToken()
		}
		events, convErr := conv.ToAnthropicSSEResponse(chunk, *isFirst, exposedModel, tracker)
		*isFirst = false
		if convErr != nil {
			return convErr
		}
		for _, event := range events {
			fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event) //nolint:errcheck // best-effort write
			fmt.Fprintf(w, constant.SSEDataLineTemplate, event.Data)   //nolint:errcheck // best-effort write
		}
		return w.Flush()
	}
}

func (u *anthropicUseCase) finalizeAnthropicChatStream(ctx context.Context, conv *converter.OpenAIProtocolConverter, w *bufio.Writer, completion *dto.OpenAIChatCompletion, upstreamErr error, exposedModel string) *dto.AnthropicMessage {
	if upstreamErr != nil {
		proxyutil.WriteUpstreamSSEError(ctx, w, upstreamErr)
		return nil
	}
	var anthropicMsg *dto.AnthropicMessage
	if completion != nil {
		anthropicMsg, _ = conv.ToAnthropicResponse(completion) //nolint:errcheck // best-effort conversion
		if anthropicMsg != nil {
			anthropicMsg.Model = exposedModel
		}
	}
	_ = proxyutil.WriteAnthropicMessageStop(w) //nolint:errcheck // best-effort write
	return anthropicMsg
}

func (u *anthropicUseCase) forwardMessageViaChatUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
			auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolAnthropicMessage, totalMs, err)
			return
		}
		anthropicMsg, convErr := conv.ToAnthropicResponse(completion)
		if convErr != nil {
			logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert chat completion to anthropic message", zap.Error(convErr))
			writer.WriteJSON(anthropicInternalErrorBody)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)
		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolAnthropicMessage,
			firstTokenLatencyMs: totalMs,
			usage:               openAITokenUsage{completion.Usage},
			successStatus:       true,
		})
	})
}
