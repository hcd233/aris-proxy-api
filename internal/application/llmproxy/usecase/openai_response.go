package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"
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
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func (u *openAIUseCase) forwardResponseNative(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardResponseNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardResponseNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardResponseViaChat(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint) *huma.StreamResponse {
	conv := &converter.ResponseProtocolConverter{}
	chatReq, convErr := conv.FromResponseRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert response request to chat", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(lo.FromPtr(req.Body.Model))
	}
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	stream := req.Body.Stream != nil && *req.Body.Stream
	if stream {
		return u.forwardResponseViaChatStream(ctx, req, m, upstream, ep.Name(), body)
	}
	return u.forwardResponseViaChatUnary(ctx, req, m, upstream, ep.Name(), body)
}

func (u *openAIUseCase) forwardResponseViaAnthropic(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromResponseAPIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert response request to anthropic", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(lo.FromPtr(req.Body.Model))
	}
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	stream := req.Body.Stream != nil && *req.Body.Stream
	if stream {
		return u.forwardResponseViaAnthropicStream(ctx, req, m, upstream, ep.Name(), body)
	}
	return u.forwardResponseViaAnthropicUnary(ctx, req, m, upstream, ep.Name(), body)
}

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var finalResponse *dto.OpenAICreateResponseRsp

		proxyErr := u.openAIProxy.ForwardCreateResponseStream(ctx, upstream, body, func(event string, data []byte) error {
			if firstTokenTime.IsZero() && proxyutil.IsResponseAPIDeltaEvent(event) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			if finalResponse == nil && proxyutil.IsResponseAPITerminalEvent(event) {
				var ev dto.ResponseStreamTerminalEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Warn("[OpenAIUseCase] Failed to parse response terminal event",
						zap.String("event", event), zap.Error(err))
				} else {
					finalResponse = ev.Response
				}
			}
			replaced := proxyutil.ReplaceModelInSSEData(data, lo.FromPtr(req.Body.Model))
			if _, writeErr := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, replaced); writeErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE event frame", zap.Error(writeErr))
			}
			return w.Flush()
		})

		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIUseCase] Response API stream error", zap.Error(proxyErr))
			proxyutil.WriteUpstreamSSEError(ctx, w, proxyErr)
		}

		u.storeResponseFromRsp(ctx, req, finalResponse, proxyErr, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               lo.FromPtr(req.Body.Model),
			Endpoint:            ep.Name(),
			UpstreamProtocol:    enum.ProtocolOpenAIResponse,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromResponseUsage(finalResponse)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(proxyErr)
		task.SetErrorFromResponseStatus(finalResponse)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardResponseNativeUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		respBody, err := u.openAIProxy.ForwardCreateResponse(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
			return
		}

		replaced := proxyutil.ReplaceModelInBody(respBody, lo.FromPtr(req.Body.Model))
		if headers := util.GetPassthroughResponseHeaders(ctx); headers != nil {
			for k, v := range headers {
				writer.HumaCtx.SetHeader(k, v)
			}
		}
		writer.HumaCtx.SetStatus(fiber.StatusOK)
		writer.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write(replaced)

		var rsp dto.OpenAICreateResponseRsp
		parseErr := sonic.Unmarshal(respBody, &rsp)
		if parseErr != nil {
			log.Warn("[OpenAIUseCase] Failed to parse Response API non-stream body", zap.Error(parseErr))
		} else {
			u.storeResponseFromRsp(ctx, req, &rsp, nil, upstream.Model)
		}

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               lo.FromPtr(req.Body.Model),
			Endpoint:            ep.Name(),
			UpstreamProtocol:    enum.ProtocolOpenAIResponse,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		if parseErr == nil {
			task.SetTokensFromResponseUsage(&rsp)
			task.SetErrorFromResponseStatus(&rsp)
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardResponseViaChatStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.ResponseProtocolConverter{}
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			hasWritten, writeErr := writeResponseDeltaFromChatChunk(w, chunk)
			if firstTokenTime.IsZero() && hasWritten {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			return writeErr
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		var rsp *dto.OpenAICreateResponseRsp
		if err == nil {
			if completion != nil {
				completion.Model = exposedModel
				var convErr error
				rsp, convErr = conv.ToResponseResponse(completion)
				if convErr != nil {
					log.Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
				}
			}
			if rsp != nil {
				_ = writeResponseTerminalEvent(w, enum.ResponseStreamEventCompleted, rsp)
			}
			_ = w.Flush()
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}
		u.storeResponseFromRsp(ctx, req, rsp, err, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			Endpoint:            endpoint,
			UpstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromResponseUsage(rsp)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardResponseViaChatUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	conv := &converter.ResponseProtocolConverter{}
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, exposedModel, endpoint, enum.ProtocolOpenAIResponse, totalMs, err)
			return
		}
		completion.Model = exposedModel
		rsp, convErr := conv.ToResponseResponse(completion)
		if convErr != nil {
			logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
			writer.WriteJSON(openAIInternalErrorBody)
			return
		}
		writer.WriteJSON(rsp)
		u.storeResponseFromRsp(ctx, req, rsp, nil, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			Endpoint:            endpoint,
			UpstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromResponseUsage(rsp)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var allChunks []*dto.OpenAIChatCompletionChunk
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			chunks, convErr := anthropicConv.ToOpenAISSEResponse(event, exposedModel, chunkID)
			if convErr != nil {
				log.Error("[OpenAIUseCase] Failed to convert anthropic SSE to chat chunk", zap.Error(convErr))
				return convErr
			}
			for _, chunk := range chunks {
				if chunk == nil {
					continue
				}
				allChunks = append(allChunks, chunk)
				if _, writeErr := writeResponseDeltaFromChatChunk(w, chunk); writeErr != nil {
					return writeErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		var rsp *dto.OpenAICreateResponseRsp
		if err == nil {
			chatCompletion, _ := proxyutil.ConcatChatCompletionChunks(allChunks)
			if chatCompletion == nil && anthropicMsg != nil {
				chatCompletion, _ = anthropicConv.ToOpenAIResponse(anthropicMsg)
			}
			if chatCompletion != nil {
				chatCompletion.Model = exposedModel
				var convErr error
				rsp, convErr = responseConv.ToResponseResponse(chatCompletion)
				if convErr != nil {
					log.Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
				}
			}
			if rsp != nil {
				_ = writeResponseTerminalEvent(w, enum.ResponseStreamEventCompleted, rsp)
			}
			_ = w.Flush()
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}
		u.storeResponseFromRsp(ctx, req, rsp, err, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			Endpoint:            endpoint,
			UpstreamProtocol:    enum.ProtocolAnthropicMessage,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailureWithProviders(u.taskSubmitter, ctx, m, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
			return
		}
		chatCompletion, convErr := anthropicConv.ToOpenAIResponse(anthropicMsg)
		if convErr != nil {
			logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert anthropic message to chat completion", zap.Error(convErr))
			writer.WriteJSON(openAIInternalErrorBody)
			return
		}
		chatCompletion.Model = exposedModel
		rsp, convErr := responseConv.ToResponseResponse(chatCompletion)
		if convErr != nil {
			logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
			writer.WriteJSON(openAIInternalErrorBody)
			return
		}
		writer.WriteJSON(rsp)
		u.storeResponseFromRsp(ctx, req, rsp, nil, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			Endpoint:            endpoint,
			UpstreamProtocol:    enum.ProtocolAnthropicMessage,
			APIProtocol:         enum.ProtocolOpenAIResponse,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func writeResponseDeltaFromChatChunk(w *bufio.Writer, chunk *dto.OpenAIChatCompletionChunk) (bool, error) {
	if chunk == nil {
		return false, nil
	}
	wroteDelta := false
	for _, choice := range chunk.Choices {
		if choice == nil || choice.Delta == nil {
			continue
		}
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			wroteDelta = true
			if err := writeResponseDeltaEvent(w, enum.ResponseStreamEventOutputTextDelta, *choice.Delta.Content); err != nil {
				return wroteDelta, err
			}
		}
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			wroteDelta = true
			if err := writeResponseDeltaEvent(w, enum.ResponseStreamEventReasoningTextDelta, *choice.Delta.ReasoningContent); err != nil {
				return wroteDelta, err
			}
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			if toolCall != nil && toolCall.Function != nil && toolCall.Function.Arguments != "" {
				wroteDelta = true
				if err := writeResponseDeltaEvent(w, enum.ResponseStreamEventFunctionCallArgumentsDelta, toolCall.Function.Arguments); err != nil {
					return wroteDelta, err
				}
			}
		}
	}
	if wroteDelta {
		return true, w.Flush()
	}
	return false, nil
}

func writeResponseDeltaEvent(w *bufio.Writer, event enum.ResponseStreamEventType, delta string) error {
	payload := lo.Must1(sonic.Marshal(map[string]string{
		constant.ResponseStreamFieldType:  string(event),
		constant.ResponseStreamFieldDelta: delta,
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}

func writeResponseTerminalEvent(w *bufio.Writer, event enum.ResponseStreamEventType, rsp *dto.OpenAICreateResponseRsp) error {
	payload := lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: string(event), Response: rsp}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}
