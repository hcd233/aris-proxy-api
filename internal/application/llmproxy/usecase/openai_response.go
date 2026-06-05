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

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse { //nolint:gocognit // this function orchestrates streaming response forwarding which inherently involves multiple concerns
	log := logger.WithCtx(ctx)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var finalResponse *dto.OpenAICreateResponseRsp
		// 从流式事件中累积 output items，以应对上游终态事件 output 为空的情况
		accumulatedOutput := make([]*dto.ResponseInputItem, 0)

		proxyErr := u.openAIProxy.ForwardCreateResponseStream(ctx, upstream, body, func(event string, data []byte) error {
			if firstTokenTime.IsZero() && proxyutil.IsResponseAPIDeltaEvent(event) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			// 累积 output_item.done 事件中的完整 output item
			if event == enum.ResponseStreamEventOutputItemDone {
				var ev dto.ResponseStreamOutputItemDoneEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Debug("[OpenAIUseCase] Failed to parse output_item.done event", zap.Error(err))
				} else if ev.Item != nil {
					accumulatedOutput = append(accumulatedOutput, ev.Item)
				}
			}
			if finalResponse == nil && proxyutil.IsResponseAPITerminalEvent(event) { //nolint:nestif // streaming event processing naturally involves nested conditional logic
				var ev dto.ResponseStreamTerminalEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Warn("[OpenAIUseCase] Failed to parse response terminal event",
						zap.String("event", event), zap.Error(err))
				} else {
					finalResponse = ev.Response
					if finalResponse != nil {
						outputTypes := make([]string, 0, len(finalResponse.Output))
						for _, item := range finalResponse.Output {
							if item != nil {
								outputTypes = append(outputTypes, lo.FromPtr(item.Type))
							}
						}
						log.Info("[OpenAIUseCase] Terminal event parsed",
							zap.String("event", event),
							zap.Int("outputCount", len(finalResponse.Output)),
							zap.Strings("outputTypes", outputTypes),
							zap.Int("accumulatedCount", len(accumulatedOutput)))
					}
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

		// 如果终态事件的 output 为空但有累积的 output items，使用累积的数据
		if finalResponse != nil && len(finalResponse.Output) == 0 && len(accumulatedOutput) > 0 {
			log.Info("[OpenAIUseCase] Using accumulated output items",
				zap.Int("count", len(accumulatedOutput)))
			finalResponse.Output = accumulatedOutput
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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
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
			auditFailure(ctx, u.taskSubmitter, m, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
			return
		}

		replaced := proxyutil.ReplaceModelInBody(respBody, lo.FromPtr(req.Body.Model))
		if headers := util.GetPassthroughResponseHeaders(ctx); headers != nil {
			for k, v := range headers {
				writer.HumaCtx.SetHeader(k, v)
			}
		}
		writer.HumaCtx.SetStatus(fiber.StatusOK)
		writer.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write(replaced) //nolint:errcheck // best-effort write in stream response handler

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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}

func (u *openAIUseCase) forwardResponseViaChatStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(conv, req)
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, exposedModel); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}

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
		if err != nil {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		} else {
			rsp = finalizeResponseFromChatCompletion(ctx, w, completion, exposedModel, conv)
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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
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
			auditFailure(ctx, u.taskSubmitter, m, exposedModel, endpoint, enum.ProtocolOpenAIResponse, totalMs, err)
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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	return apiutil.WrapStreamResponse(u.forwardResponseViaAnthropicStreamBody(ctx, req, m, upstream, endpoint, body))
}

func (u *openAIUseCase) forwardResponseViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) func(w *bufio.Writer) {
	log := logger.WithCtx(ctx)
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(responseConv, req)
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	exposedModel := lo.FromPtr(req.Body.Model)
	return func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var allChunks []*dto.OpenAIChatCompletionChunk

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, exposedModel); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			chunks, convErr := anthropicConv.ToOpenAISSEResponse(event, exposedModel, chunkID)
			if convErr != nil {
				logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert anthropic SSE to chat chunk", zap.Error(convErr))
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
		rsp := finalizeResponseFromAnthropicStream(ctx, w, err, allChunks, anthropicMsg, exposedModel, anthropicConv, responseConv)
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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	}
}

func (u *openAIUseCase) forwardResponseViaAnthropicUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(responseConv, req)
	exposedModel := lo.FromPtr(req.Body.Model)
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailureWithProviders(ctx, u.taskSubmitter, m, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
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
		delta := choice.Delta
		wrote, err := writeDeltaField(w, enum.ResponseStreamEventOutputTextDelta, delta.Content)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		wrote, err = writeDeltaField(w, enum.ResponseStreamEventReasoningTextDelta, delta.ReasoningContent)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		wrote, err = writeToolCallDeltas(w, delta.ToolCalls)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote
	}
	if wroteDelta {
		return true, w.Flush()
	}
	return false, nil
}

func writeDeltaField(w *bufio.Writer, event enum.ResponseStreamEventType, value *string) (bool, error) {
	if value == nil || *value == "" {
		return false, nil
	}
	if err := writeResponseDeltaEvent(w, event, *value); err != nil {
		return false, err
	}
	return true, nil
}

func writeToolCallDeltas(w *bufio.Writer, toolCalls []*dto.OpenAIChatCompletionMessageToolCall) (bool, error) {
	wrote := false
	for _, tc := range toolCalls {
		if tc != nil && tc.Function != nil && tc.Function.Arguments != "" {
			wrote = true
			if err := writeResponseDeltaEvent(w, enum.ResponseStreamEventFunctionCallArgumentsDelta, tc.Function.Arguments); err != nil {
				return wrote, err
			}
		}
	}
	return wrote, nil
}

func writeResponseDeltaEvent(w *bufio.Writer, event enum.ResponseStreamEventType, delta string) error {
	payload := lo.Must1(sonic.Marshal(map[string]string{
		constant.ResponseStreamFieldType:  event,
		constant.ResponseStreamFieldDelta: delta,
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}

func writeResponseTerminalEvent(w *bufio.Writer, event enum.ResponseStreamEventType, rsp *dto.OpenAICreateResponseRsp) error {
	payload := lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: event, Response: rsp}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}

func finalizeResponseFromChatCompletion(ctx context.Context, w *bufio.Writer, completion *dto.OpenAIChatCompletion, exposedModel string, conv *converter.ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
	if completion == nil {
		return nil
	}
	completion.Model = exposedModel
	rsp, convErr := conv.ToResponseResponse(completion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
	}
	if rsp != nil {
		_ = writeResponseTerminalEvent(w, enum.ResponseStreamEventCompleted, rsp) //nolint:errcheck // best-effort write on stream close
	}
	_ = w.Flush() //nolint:errcheck // flush best effort on stream close
	return rsp
}

func finalizeResponseFromAnthropicStream(ctx context.Context, w *bufio.Writer, upstreamErr error, allChunks []*dto.OpenAIChatCompletionChunk, anthropicMsg *dto.AnthropicMessage, exposedModel string, anthropicConv *converter.AnthropicProtocolConverter, responseConv *converter.ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
	if upstreamErr != nil {
		proxyutil.WriteUpstreamSSEError(ctx, w, upstreamErr)
		return nil
	}
	chatCompletion, _ := proxyutil.ConcatChatCompletionChunks(allChunks) //nolint:errcheck // store even if concat fails
	if chatCompletion == nil && anthropicMsg != nil {
		chatCompletion, _ = anthropicConv.ToOpenAIResponse(anthropicMsg) //nolint:errcheck // best-effort fallback conversion
	}
	if chatCompletion == nil {
		return nil
	}
	chatCompletion.Model = exposedModel
	rsp, convErr := responseConv.ToResponseResponse(chatCompletion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
	}
	if rsp != nil {
		_ = writeResponseTerminalEvent(w, enum.ResponseStreamEventCompleted, rsp) //nolint:errcheck // best-effort write on stream close
	}
	_ = w.Flush() //nolint:errcheck // flush best effort on stream close
	return rsp
}

func assertRespConvInit(conv *converter.ResponseProtocolConverter, req *dto.OpenAICreateResponseRequest) {
	if req == nil || req.Body == nil || len(req.Body.Tools) == 0 {
		return
	}
	conv.SetToolTypeMap(converter.BuildToolTypeMap(req.Body.Tools))
}

func writeResponseLifecycleEvent(w *bufio.Writer, event enum.ResponseStreamEventType, model string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType: event,
		constant.ResponseStreamFieldResponse: map[string]any{
			constant.ResponseStreamFieldObject: enum.CompletionObjectResponse,
			constant.ResponseStreamFieldModel:  model,
			constant.ResponseStreamFieldStatus: enum.ResponseStatusInProgress,
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}
