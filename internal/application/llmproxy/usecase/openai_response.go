package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
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
				log.Info("[OpenAIUseCase] Native response first delta event",
					zap.String("event", event),
					zap.Int64("firstTokenLatencyMs", firstTokenLatencyMs))
			}
			// 累积 output_item.done 事件中的完整 output item
			if event == enum.ResponseStreamEventOutputItemDone {
				var ev dto.ResponseStreamOutputItemDoneEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Debug("[OpenAIUseCase] Failed to parse output_item.done event", zap.Error(err))
				} else if ev.Item != nil {
					accumulatedOutput = append(accumulatedOutput, ev.Item)
					log.Info("[OpenAIUseCase] Native response output_item.done",
						zap.Int("outputIndex", ev.OutputIndex),
						zap.Stringp("itemType", ev.Item.Type))
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
						outputTypes := lo.FilterMap(finalResponse.Output, func(item *dto.ResponseInputItem, _ int) (string, bool) {
							if item == nil {
								return "", false
							}
							return lo.FromPtr(item.Type), true
						})
						log.Info("[OpenAIUseCase] Native response terminal event parsed",
							zap.String("event", event),
							zap.String("responseID", finalResponse.ID),
							zap.String("status", finalResponse.Status),
							zap.Int("outputCount", len(finalResponse.Output)),
							zap.Strings("outputTypes", outputTypes),
							zap.Int("accumulatedCount", len(accumulatedOutput)))
					}
				}
			}
			outgoingData := data
			if proxyutil.IsResponseAPITerminalEvent(event) {
				patched, changed, err := proxyutil.FillResponseTerminalOutput(data, accumulatedOutput)
				if err != nil {
					log.Warn("[OpenAIUseCase] Failed to fill response terminal output", zap.String("event", event), zap.Error(err))
				} else if changed {
					outgoingData = patched
					if finalResponse != nil {
						finalResponse.Output = accumulatedOutput
					}
					log.Info("[OpenAIUseCase] Native response terminal output filled", zap.Int("count", len(accumulatedOutput)))
				}
			}
			replaced := proxyutil.ReplaceModelInSSEData(outgoingData, lo.FromPtr(req.Body.Model))
			if _, writeErr := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, replaced); writeErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE event frame", zap.Error(writeErr))
			}
			return w.Flush()
		})

		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIUseCase] Native response stream error", zap.Error(proxyErr))
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
		if finalResponse != nil && finalResponse.Usage != nil {
			reportTokenUsage(ctx, finalResponse.Usage.InputOutputTokens())
		}
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
			auditFailure(ctx, m, u.taskSubmitter, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
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
			if rsp.Usage != nil {
				reportTokenUsage(ctx, rsp.Usage.InputOutputTokens())
			}
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
	responseID := fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String())
	initializedItems := make(map[int]bool)
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var chunkCount int64

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}
		log.Info("[OpenAIUseCase] Via chat response.created written",
			zap.String("responseID", responseID),
			zap.String("model", exposedModel))

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventInProgress, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
		}

		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			chunkCount++
			hasWritten, writeErr := writeResponseDeltaFromChatChunk(w, chunk, initializedItems, responseID)
			if firstTokenTime.IsZero() && hasWritten {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				log.Info("[OpenAIUseCase] Via chat first delta event",
					zap.Int64("firstTokenLatencyMs", firstTokenLatencyMs),
					zap.Int64("chunkCount", chunkCount))
			}
			return writeErr
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		log.Info("[OpenAIUseCase] Via chat stream completed",
			zap.Int64("chunkCount", chunkCount),
			zap.Int64("streamDurationMs", streamDurationMs),
			zap.Bool("hasError", err != nil))

		var rsp *dto.OpenAICreateResponseRsp
		if err != nil {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		} else {
			rsp = finalizeResponseFromChatCompletion(ctx, w, completion, exposedModel, responseID, conv)
			if rsp != nil {
				log.Info("[OpenAIUseCase] Via chat response finalized",
					zap.String("responseID", rsp.ID),
					zap.String("status", rsp.Status),
					zap.Int("outputCount", len(rsp.Output)))
			}
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
		if rsp != nil && rsp.Usage != nil {
			reportTokenUsage(ctx, rsp.Usage.InputOutputTokens())
		}
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
			auditFailure(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolOpenAIResponse, totalMs, err)
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
		if rsp != nil && rsp.Usage != nil {
			reportTokenUsage(ctx, rsp.Usage.InputOutputTokens())
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	return apiutil.WrapStreamResponse(u.forwardResponseViaAnthropicStreamBody(ctx, req, m, upstream, endpoint, body))
}

func (u *openAIUseCase) forwardResponseViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) func(w *bufio.Writer) { //nolint:gocognit // streaming response forwarding naturally involves multiple concerns
	log := logger.WithCtx(ctx)
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(responseConv, req)
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	exposedModel := lo.FromPtr(req.Body.Model)
	responseID := fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String())
	initializedItems := make(map[int]bool)
	return func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var allChunks []*dto.OpenAIChatCompletionChunk
		var chunkCount int64

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}
		log.Info("[OpenAIUseCase] Via anthropic response.created written",
			zap.String("responseID", responseID),
			zap.String("model", exposedModel))

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventInProgress, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
		}

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				log.Info("[OpenAIUseCase] Via anthropic first delta event",
					zap.Int64("firstTokenLatencyMs", firstTokenLatencyMs))
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
				chunkCount++
				allChunks = append(allChunks, chunk)
				if _, writeErr := writeResponseDeltaFromChatChunk(w, chunk, initializedItems, responseID); writeErr != nil {
					return writeErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		log.Info("[OpenAIUseCase] Via anthropic stream completed",
			zap.Int64("chunkCount", chunkCount),
			zap.Int64("streamDurationMs", streamDurationMs),
			zap.Bool("hasError", err != nil))

		rsp := finalizeResponseFromAnthropicStream(ctx, w, err, allChunks, anthropicMsg, exposedModel, responseID, anthropicConv, responseConv)
		if rsp != nil {
			log.Info("[OpenAIUseCase] Via anthropic response finalized",
				zap.String("responseID", rsp.ID),
				zap.String("status", rsp.Status),
				zap.Int("outputCount", len(rsp.Output)))
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
		if anthropicMsg != nil && anthropicMsg.Usage != nil {
			reportTokenUsage(ctx, anthropicMsg.Usage.InputOutputTokens())
		}
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
			auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
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
		if anthropicMsg != nil && anthropicMsg.Usage != nil {
			reportTokenUsage(ctx, anthropicMsg.Usage.InputOutputTokens())
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
	})
}

func writeResponseDeltaFromChatChunk(w *bufio.Writer, chunk *dto.OpenAIChatCompletionChunk, initializedItems map[int]bool, responseID string) (bool, error) { //nolint:gocognit // streaming event processing naturally involves multiple concerns
	if chunk == nil {
		return false, nil
	}
	wroteDelta := false
	for _, choice := range chunk.Choices {
		if choice == nil || choice.Delta == nil {
			continue
		}
		delta := choice.Delta
		itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
		outputIndex := choice.Index

		// 在发送第一个 delta 之前，发送 output_item.added 和 content_part.added
		if !initializedItems[outputIndex] {
			initializedItems[outputIndex] = true
			if err := writeOutputItemAddedEvent(w, itemID, outputIndex); err != nil {
				return wroteDelta, err
			}
			if err := writeContentPartAddedEvent(w, itemID, outputIndex); err != nil {
				return wroteDelta, err
			}
		}

		wrote, err := writeDeltaField(w, enum.ResponseStreamEventOutputTextDelta, delta.Content, itemID, outputIndex, 0)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		wrote, err = writeDeltaField(w, enum.ResponseStreamEventReasoningTextDelta, delta.ReasoningContent, itemID, outputIndex, 0)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		wrote, err = writeToolCallDeltas(w, delta.ToolCalls, itemID, outputIndex)
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

func writeDeltaField(w *bufio.Writer, event enum.ResponseStreamEventType, value *string, itemID string, outputIndex, contentIndex int) (bool, error) {
	if value == nil || *value == "" {
		return false, nil
	}
	if err := writeResponseDeltaEvent(w, event, *value, itemID, outputIndex, contentIndex); err != nil {
		return false, err
	}
	return true, nil
}

func writeToolCallDeltas(w *bufio.Writer, toolCalls []*dto.OpenAIChatCompletionMessageToolCall, itemID string, outputIndex int) (bool, error) {
	wrote := false
	for _, tc := range toolCalls {
		if tc != nil && tc.Function != nil && tc.Function.Arguments != "" {
			wrote = true
			if err := writeResponseDeltaEvent(w, enum.ResponseStreamEventFunctionCallArgumentsDelta, tc.Function.Arguments, itemID, outputIndex, 0); err != nil {
				return wrote, err
			}
		}
	}
	return wrote, nil
}

func writeResponseDeltaEvent(w *bufio.Writer, event enum.ResponseStreamEventType, delta, itemID string, outputIndex, contentIndex int) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         event,
		constant.ResponseStreamFieldDelta:        delta,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: contentIndex,
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}

func writeOutputItemAddedEvent(w *bufio.Writer, itemID string, outputIndex int) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem: map[string]any{
			constant.ResponseStreamFieldID:      itemID,
			constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
			constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusInProgress,
			constant.ResponseStreamFieldRole:    enum.RoleAssistant,
			constant.ResponseStreamFieldContent: []any{},
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, enum.ResponseStreamEventOutputItemAdded, payload)
	return err
}

func writeContentPartAddedEvent(w *bufio.Writer, itemID string, outputIndex int) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartAdded,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldPart: map[string]any{
			constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
			constant.ResponseStreamFieldText:        "",
			constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, enum.ResponseStreamEventContentPartAdded, payload)
	return err
}

func writeOutputTextDoneEvent(w *bufio.Writer, itemID string, outputIndex int, text string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventOutputTextDone,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldText:         text,
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, enum.ResponseStreamEventOutputTextDone, payload)
	return err
}

func writeContentPartDoneEvent(w *bufio.Writer, itemID string, outputIndex int, text string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartDone,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldPart: map[string]any{
			constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
			constant.ResponseStreamFieldText:        text,
			constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, enum.ResponseStreamEventContentPartDone, payload)
	return err
}

func writeOutputItemDoneEvent(w *bufio.Writer, itemID string, outputIndex int, content []map[string]any) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem: map[string]any{
			constant.ResponseStreamFieldID:      itemID,
			constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
			constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusCompleted,
			constant.ResponseStreamFieldRole:    enum.RoleAssistant,
			constant.ResponseStreamFieldContent: content,
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, enum.ResponseStreamEventOutputItemDone, payload)
	return err
}

func writeResponseTerminalEvent(w *bufio.Writer, event enum.ResponseStreamEventType, rsp *dto.OpenAICreateResponseRsp) error {
	payload := lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: event, Response: rsp}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}

func finalizeResponseFromChatCompletion(ctx context.Context, w *bufio.Writer, completion *dto.OpenAIChatCompletion, exposedModel, responseID string, conv *converter.ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
	if completion == nil {
		return nil
	}
	completion.Model = exposedModel
	rsp, convErr := conv.ToResponseResponse(completion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
	}
	if rsp != nil {
		// 使用传入的 responseID 确保 response.created 和 response.completed 的 ID 一致
		rsp.ID = responseID
		rsp.CreatedAt = time.Now().Unix()
		// 发送完成事件序列
		for _, choice := range completion.Choices {
			if choice == nil {
				continue
			}
			itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
			outputIndex := choice.Index

			// 获取文本内容
			var textContent string
			if choice.Message != nil && choice.Message.Content != nil {
				textContent = choice.Message.Content.Text
			}

			// 发送 output_text.done
			_ = writeOutputTextDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
			// 发送 content_part.done
			_ = writeContentPartDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
			// 发送 output_item.done
			content := []map[string]any{{
				constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
				constant.ResponseStreamFieldText:        textContent,
				constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
			}}
			_ = writeOutputItemDoneEvent(w, itemID, outputIndex, content) //nolint:errcheck // best-effort write on stream close
		}
		_ = writeResponseTerminalEvent(w, enum.ResponseStreamEventCompleted, rsp) //nolint:errcheck // best-effort write on stream close
	}
	_ = w.Flush() //nolint:errcheck // flush best effort on stream close
	return rsp
}

func finalizeResponseFromAnthropicStream(ctx context.Context, w *bufio.Writer, upstreamErr error, allChunks []*dto.OpenAIChatCompletionChunk, anthropicMsg *dto.AnthropicMessage, exposedModel, responseID string, anthropicConv *converter.AnthropicProtocolConverter, responseConv *converter.ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
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
		// 使用传入的 responseID 确保 response.created 和 response.completed 的 ID 一致
		rsp.ID = responseID
		rsp.CreatedAt = time.Now().Unix()
		// 发送完成事件序列
		for _, choice := range chatCompletion.Choices {
			if choice == nil {
				continue
			}
			itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
			outputIndex := choice.Index

			// 获取文本内容
			var textContent string
			if choice.Message != nil && choice.Message.Content != nil {
				textContent = choice.Message.Content.Text
			}

			// 发送 output_text.done
			_ = writeOutputTextDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
			// 发送 content_part.done
			_ = writeContentPartDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
			// 发送 output_item.done
			content := []map[string]any{{
				constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
				constant.ResponseStreamFieldText:        textContent,
				constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
			}}
			_ = writeOutputItemDoneEvent(w, itemID, outputIndex, content) //nolint:errcheck // best-effort write on stream close
		}
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

func writeResponseLifecycleEvent(w *bufio.Writer, event enum.ResponseStreamEventType, model, responseID string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType: event,
		constant.ResponseStreamFieldResponse: map[string]any{
			constant.ResponseStreamFieldID:        responseID,
			constant.ResponseStreamFieldObject:    enum.CompletionObjectResponse,
			constant.ResponseStreamFieldModel:     model,
			constant.ResponseStreamFieldStatus:    constant.ResponseStreamFieldStatusInProgress,
			constant.ResponseStreamFieldCreatedAt: time.Now().Unix(),
			constant.ResponseStreamFieldOutput:    []any{},
		},
	}))
	_, err := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload)
	return err
}
