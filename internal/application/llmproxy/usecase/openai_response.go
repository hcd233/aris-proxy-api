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

type nativeStreamHandler struct {
	u                *openAIUseCase
	ctx              context.Context
	req              *dto.OpenAICreateResponseRequest
	w                *bufio.Writer
	startTime        time.Time
	firstTokenTime   time.Time
	firstTokenLatencyMs int64
	streamDurationMs int64
	finalResponse    *dto.OpenAICreateResponseRsp
	accumulatedOutput []*dto.ResponseInputItem
	logger           *zap.Logger
}

func newNativeStreamHandler(ctx context.Context, u *openAIUseCase, req *dto.OpenAICreateResponseRequest, w *bufio.Writer) *nativeStreamHandler {
	return &nativeStreamHandler{
		u:                u,
		ctx:              ctx,
		req:              req,
		w:                w,
		startTime:        time.Now(),
		accumulatedOutput: make([]*dto.ResponseInputItem, 0),
		logger:           logger.WithCtx(ctx),
	}
}

func (h *nativeStreamHandler) onEvent(event string, data []byte) error {
	if h.firstTokenTime.IsZero() && proxyutil.IsResponseAPIDeltaEvent(event) {
		h.firstTokenTime = time.Now()
		h.firstTokenLatencyMs = h.firstTokenTime.Sub(h.startTime).Milliseconds()
		h.logger.Info("[OpenAIUseCase] Native response first delta event",
			zap.String("event", event),
			zap.Int64("firstTokenLatencyMs", h.firstTokenLatencyMs))
	}
	h.handleOutputItemDone(event, data)
	h.handleTerminalEvent(event, data)

	outgoingData := h.patchTerminalOutput(event, data)
	replaced := proxyutil.ReplaceModelInSSEData(outgoingData, lo.FromPtr(h.req.Body.Model))
	if _, writeErr := fmt.Fprintf(h.w, constant.SSEEventFrameTemplate, event, replaced); writeErr != nil {
		h.logger.Debug("[OpenAIUseCase] Failed to write SSE event frame", zap.Error(writeErr))
	}
	return h.w.Flush()
}

func (h *nativeStreamHandler) handleOutputItemDone(event string, data []byte) {
	if event != enum.ResponseStreamEventOutputItemDone {
		return
	}
	var ev dto.ResponseStreamOutputItemDoneEvent
	if err := sonic.Unmarshal(data, &ev); err != nil {
		h.logger.Debug("[OpenAIUseCase] Failed to parse output_item.done event", zap.Error(err))
		return
	}
	if ev.Item == nil {
		return
	}
	h.accumulatedOutput = append(h.accumulatedOutput, ev.Item)
	h.logger.Info("[OpenAIUseCase] Native response output_item.done",
		zap.Int("outputIndex", ev.OutputIndex),
		zap.Stringp("itemType", ev.Item.Type))
}

func (h *nativeStreamHandler) handleTerminalEvent(event string, data []byte) {
	if h.finalResponse != nil || !proxyutil.IsResponseAPITerminalEvent(event) {
		return
	}
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(data, &ev); err != nil {
		h.logger.Warn("[OpenAIUseCase] Failed to parse response terminal event",
			zap.String("event", event), zap.Error(err))
		return
	}
	h.finalResponse = ev.Response
	if h.finalResponse == nil {
		return
	}
	outputTypes := lo.FilterMap(h.finalResponse.Output, func(item *dto.ResponseInputItem, _ int) (string, bool) {
		if item == nil {
			return "", false
		}
		return lo.FromPtr(item.Type), true
	})
	h.logger.Info("[OpenAIUseCase] Native response terminal event parsed",
		zap.String("event", event),
		zap.String("responseID", h.finalResponse.ID),
		zap.String("status", h.finalResponse.Status),
		zap.Int("outputCount", len(h.finalResponse.Output)),
		zap.Strings("outputTypes", outputTypes),
		zap.Int("accumulatedCount", len(h.accumulatedOutput)))
}

func (h *nativeStreamHandler) patchTerminalOutput(event string, data []byte) []byte {
	if !proxyutil.IsResponseAPITerminalEvent(event) {
		return data
	}
	patched, changed, err := proxyutil.FillResponseTerminalOutput(data, h.accumulatedOutput)
	if err != nil {
		h.logger.Warn("[OpenAIUseCase] Failed to fill response terminal output", zap.String("event", event), zap.Error(err))
		return data
	}
	if !changed {
		return data
	}
	if h.finalResponse != nil {
		h.finalResponse.Output = h.accumulatedOutput
	}
	h.logger.Info("[OpenAIUseCase] Native response terminal output filled", zap.Int("count", len(h.accumulatedOutput)))
	return patched
}

func (h *nativeStreamHandler) finalize(w *bufio.Writer, proxyErr error, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint) {
	if !h.firstTokenTime.IsZero() {
		h.streamDurationMs = time.Since(h.firstTokenTime).Milliseconds()
	}
	if proxyErr != nil {
		h.logger.Error("[OpenAIUseCase] Native response stream error", zap.Error(proxyErr))
		proxyutil.WriteUpstreamSSEError(h.ctx, w, proxyErr)
	}
	if h.finalResponse != nil && len(h.finalResponse.Output) == 0 && len(h.accumulatedOutput) > 0 {
		h.logger.Info("[OpenAIUseCase] Using accumulated output items", zap.Int("count", len(h.accumulatedOutput)))
		h.finalResponse.Output = h.accumulatedOutput
	}
	h.u.storeResponseFromRsp(h.ctx, h.req, h.finalResponse, proxyErr, upstream.Model)

	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(h.ctx),
		ModelID:             m.AggregateID(),
		Model:               lo.FromPtr(h.req.Body.Model),
		Endpoint:            ep.Name(),
		UpstreamProtocol:    enum.ProtocolOpenAIResponse,
		APIProtocol:         enum.ProtocolOpenAIResponse,
		FirstTokenLatencyMs: h.firstTokenLatencyMs,
		StreamDurationMs:    h.streamDurationMs,
	}
	task.SetTokensFromResponseUsage(h.finalResponse)
	if h.finalResponse != nil && h.finalResponse.Usage != nil {
		reportTokenUsage(h.ctx, h.finalResponse.Usage.InputOutputTokens())
	}
	task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(proxyErr)
	task.SetErrorFromResponseStatus(h.finalResponse)
	_ = h.u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
}

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return apiutil.WrapStreamResponse(func(w *bufio.Writer) {
		h := newNativeStreamHandler(ctx, u, req, w)
		proxyErr := u.openAIProxy.ForwardCreateResponseStream(ctx, upstream, body, h.onEvent)
		h.finalize(w, proxyErr, m, ep, upstream)
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

type anthropicStreamHandler struct {
	u                 *openAIUseCase
	ctx               context.Context
	req               *dto.OpenAICreateResponseRequest
	responseConv      *converter.ResponseProtocolConverter
	anthropicConv     *converter.AnthropicProtocolConverter
	exposedModel      string
	responseID        string
	chunkID           string
	startTime         time.Time
	firstTokenTime    time.Time
	firstTokenLatencyMs int64
	streamDurationMs  int64
	allChunks         []*dto.OpenAIChatCompletionChunk
	chunkCount        int64
	initializedItems  map[int]bool
	logger            *zap.Logger
}

func newAnthropicStreamHandler(ctx context.Context, u *openAIUseCase, req *dto.OpenAICreateResponseRequest) *anthropicStreamHandler {
	exposedModel := lo.FromPtr(req.Body.Model)
	return &anthropicStreamHandler{
		u:               u,
		ctx:             ctx,
		req:             req,
		responseConv:    &converter.ResponseProtocolConverter{},
		anthropicConv:   &converter.AnthropicProtocolConverter{},
		exposedModel:    exposedModel,
		responseID:      fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String()),
		chunkID:         fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix),
		startTime:       time.Now(),
		initializedItems: make(map[int]bool),
		logger:          logger.WithCtx(ctx),
	}
}

func (h *anthropicStreamHandler) onAnthropicEvent(w *bufio.Writer, event dto.AnthropicSSEEvent) error {
	if h.firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
		h.firstTokenTime = time.Now()
		h.firstTokenLatencyMs = h.firstTokenTime.Sub(h.startTime).Milliseconds()
		h.logger.Info("[OpenAIUseCase] Via anthropic first delta event",
			zap.Int64("firstTokenLatencyMs", h.firstTokenLatencyMs))
	}
	chunks, convErr := h.anthropicConv.ToOpenAISSEResponse(event, h.exposedModel, h.chunkID)
	if convErr != nil {
		h.logger.Error("[OpenAIUseCase] Failed to convert anthropic SSE to chat chunk", zap.Error(convErr))
		return convErr
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		h.chunkCount++
		h.allChunks = append(h.allChunks, chunk)
		if _, writeErr := writeResponseDeltaFromChatChunk(w, chunk, h.initializedItems, h.responseID); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func (h *anthropicStreamHandler) finalize(w *bufio.Writer, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, anthropicMsg *dto.AnthropicMessage, err error) {
	if !h.firstTokenTime.IsZero() {
		h.streamDurationMs = time.Since(h.firstTokenTime).Milliseconds()
	}
	h.logger.Info("[OpenAIUseCase] Via anthropic stream completed",
		zap.Int64("chunkCount", h.chunkCount),
		zap.Int64("streamDurationMs", h.streamDurationMs),
		zap.Bool("hasError", err != nil))

	rsp := finalizeResponseFromAnthropicStream(h.ctx, w, err, h.allChunks, anthropicMsg, h.exposedModel, h.responseID, h.anthropicConv, h.responseConv)
	if rsp != nil {
		h.logger.Info("[OpenAIUseCase] Via anthropic response finalized",
			zap.String("responseID", rsp.ID),
			zap.String("status", rsp.Status),
			zap.Int("outputCount", len(rsp.Output)))
	}
	h.u.storeResponseFromRsp(h.ctx, h.req, rsp, err, upstream.Model)
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(h.ctx),
		ModelID:             m.AggregateID(),
		Model:               h.exposedModel,
		Endpoint:            endpoint,
		UpstreamProtocol:    enum.ProtocolAnthropicMessage,
		APIProtocol:         enum.ProtocolOpenAIResponse,
		FirstTokenLatencyMs: h.firstTokenLatencyMs,
		StreamDurationMs:    h.streamDurationMs,
	}
	task.SetTokensFromAnthropicUsage(anthropicMsg)
	if anthropicMsg != nil && anthropicMsg.Usage != nil {
		reportTokenUsage(h.ctx, anthropicMsg.Usage.InputOutputTokens())
	}
	task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
	_ = h.u.taskSubmitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
}

func (u *openAIUseCase) forwardResponseViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) func(w *bufio.Writer) {
	h := newAnthropicStreamHandler(ctx, u, req)
	return func(w *bufio.Writer) {
		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, h.exposedModel, h.responseID); err != nil {
			h.logger.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}
		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventInProgress, h.exposedModel, h.responseID); err != nil {
			h.logger.Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
		}
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			return h.onAnthropicEvent(w, event)
		})
		h.finalize(w, m, upstream, endpoint, anthropicMsg, err)
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

func writeResponseDeltaFromChatChunk(w *bufio.Writer, chunk *dto.OpenAIChatCompletionChunk, initializedItems map[int]bool, responseID string) (bool, error) {
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
		initErr := initOutputItem(w, initializedItems, itemID, outputIndex)
		if initErr != nil {
			return wroteDelta, initErr
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

func initOutputItem(w *bufio.Writer, initializedItems map[int]bool, itemID string, outputIndex int) error {
	if initializedItems[outputIndex] {
		return nil
	}
	initializedItems[outputIndex] = true
	if err := writeOutputItemAddedEvent(w, itemID, outputIndex); err != nil {
		return err
	}
	return writeContentPartAddedEvent(w, itemID, outputIndex)
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
		rsp.ID = responseID
		rsp.CreatedAt = time.Now().Unix()
		for _, choice := range completion.Choices {
			if choice == nil {
				continue
			}
			itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
			outputIndex := choice.Index

			var textContent string
			if choice.Message != nil && choice.Message.Content != nil {
				textContent = choice.Message.Content.Text
			}

			_ = writeOutputTextDoneEvent(w, itemID, outputIndex, textContent)  //nolint:errcheck // best-effort write on stream close
			_ = writeContentPartDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
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
		rsp.ID = responseID
		rsp.CreatedAt = time.Now().Unix()
		for _, choice := range chatCompletion.Choices {
			if choice == nil {
				continue
			}
			itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
			outputIndex := choice.Index

			var textContent string
			if choice.Message != nil && choice.Message.Content != nil {
				textContent = choice.Message.Content.Text
			}

			_ = writeOutputTextDoneEvent(w, itemID, outputIndex, textContent)  //nolint:errcheck // best-effort write on stream close
			_ = writeContentPartDoneEvent(w, itemID, outputIndex, textContent) //nolint:errcheck // best-effort write on stream close
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
