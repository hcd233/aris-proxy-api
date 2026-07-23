package usecase

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	u                 *openAIUseCase
	ctx               context.Context
	req               *dto.OpenAICreateResponseRequest
	timer             *streamTimer
	finalResponse     *dto.OpenAICreateResponseRsp
	accumulatedOutput []*dto.ResponseInputItem
	logger            *zap.Logger
}

func newNativeStreamHandler(ctx context.Context, u *openAIUseCase, req *dto.OpenAICreateResponseRequest) *nativeStreamHandler {
	return &nativeStreamHandler{
		u:                 u,
		ctx:               ctx,
		req:               req,
		timer:             newStreamTimer(),
		accumulatedOutput: make([]*dto.ResponseInputItem, 0),
		logger:            logger.WithCtx(ctx),
	}
}

func (h *nativeStreamHandler) onEvent(w *bufio.Writer, event string, data []byte) error {
	if proxyutil.IsResponseAPIDeltaEvent(event) {
		h.timer.markFirstToken()
	}
	h.handleOutputItemDone(event, data)
	h.handleTerminalEvent(event, data)

	outgoingData := h.patchTerminalOutput(event, data)
	replaced := proxyutil.ReplaceModelInSSEData(outgoingData, lo.FromPtr(h.req.Body.Model))
	if _, writeErr := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, replaced); writeErr != nil {
		h.logger.Debug("[OpenAIUseCase] Failed to write SSE event frame", zap.Error(writeErr))
	}
	return w.Flush()
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
	return patched
}

func (h *nativeStreamHandler) finalize(w *bufio.Writer, proxyErr error, m *aggregate.Model, ep *aggregate.Endpoint) {
	h.timer.finish()
	if proxyErr != nil {
		h.logger.Error("[OpenAIUseCase] Native response stream error", zap.Error(proxyErr))
		proxyutil.WriteUpstreamSSEError(h.ctx, w, proxyErr)
	}
	if h.finalResponse != nil && len(h.finalResponse.Output) == 0 && len(h.accumulatedOutput) > 0 {
		h.finalResponse.Output = h.accumulatedOutput
	}
	h.u.storeResponseFromRsp(h.ctx, h.req, h.finalResponse, proxyErr, m.Alias().String())

	recordModelCall(h.ctx, h.u.taskSubmitter, callOutcome{
		model:               m,
		exposedModel:        lo.FromPtr(h.req.Body.Model),
		endpoint:            ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIResponse,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: h.timer.firstLatencyMs,
		streamDurationMs:    h.timer.durationMs,
		usage:               responseTokenUsage{h.finalResponse},
		err:                 proxyErr,
		responseStatus:      h.finalResponse,
	})
}

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenCreateResponseStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, openAIInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		h := newNativeStreamHandler(ctx, u, req)
		proxyErr := u.openAIProxy.ReadCreateResponseStream(ctx, stream, func(event string, data []byte) error {
			return h.onEvent(w, event, data)
		})
		h.finalize(w, proxyErr, m, ep)
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
		out := callOutcome{
			model:               m,
			exposedModel:        lo.FromPtr(req.Body.Model),
			endpoint:            ep.Name(),
			upstreamProtocol:    enum.ProtocolOpenAIResponse,
			apiProtocol:         enum.ProtocolOpenAIResponse,
			firstTokenLatencyMs: totalMs,
			successStatus:       true,
		}
		if parseErr != nil {
			log.Debug("[OpenAIUseCase] Failed to parse Response API non-stream body", zap.Error(parseErr))
		} else {
			u.storeResponseFromRsp(ctx, req, &rsp, nil, m.Alias().String())
			out.usage = responseTokenUsage{&rsp}
			out.responseStatus = &rsp
		}
		recordModelCall(ctx, u.taskSubmitter, out)
	})
}

func (u *openAIUseCase) forwardResponseViaChatStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(conv, req)
	exposedModel := lo.FromPtr(req.Body.Model)
	responseID := fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String())
	itemState := converter.NewStreamItemState()
	startTime := time.Now()
	stream, openErr := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if openErr != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolOpenAIResponse, totalMs, openErr)
		return upstreamStreamErrorResponse(ctx, openErr, openAIInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		timer := newStreamTimer()

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}

		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventInProgress, exposedModel, responseID); err != nil {
			log.Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
		}

		completion, err := u.openAIProxy.ReadChatCompletionStream(ctx, stream, func(chunk *dto.OpenAIChatCompletionChunk) error {
			hasWritten, writeErr := converter.WriteResponseDeltaFromChatChunk(w, chunk, itemState, responseID, conv)
			if hasWritten {
				timer.markFirstToken()
			}
			return writeErr
		})
		timer.finish()

		var rsp *dto.OpenAICreateResponseRsp
		if err != nil {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		} else {
			rsp = converter.FinalizeResponseFromChatCompletion(w, completion, exposedModel, responseID, conv)
		}
		u.storeResponseFromRsp(ctx, req, rsp, err, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolOpenAIResponse,
			firstTokenLatencyMs: timer.firstLatencyMs,
			streamDurationMs:    timer.durationMs,
			usage:               responseTokenUsage{rsp},
			err:                 err,
		})
	})
}

func (u *openAIUseCase) forwardResponseViaChatUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	conv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(conv, req)
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
		u.storeResponseFromRsp(ctx, req, rsp, nil, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
			apiProtocol:         enum.ProtocolOpenAIResponse,
			firstTokenLatencyMs: totalMs,
			usage:               responseTokenUsage{rsp},
			successStatus:       true,
		})
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) *huma.StreamResponse {
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		exposedModel := lo.FromPtr(req.Body.Model)
		auditFailureWithProviders(ctx, m, u.taskSubmitter, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
		return upstreamStreamErrorResponse(ctx, err, openAIInternalErrorBody)
	}
	return apiutil.WrapStreamResponse(ctx, u.forwardResponseViaAnthropicStreamBody(ctx, req, m, stream, endpoint))
}

type anthropicStreamHandler struct {
	u             *openAIUseCase
	ctx           context.Context
	req           *dto.OpenAICreateResponseRequest
	responseConv  *converter.ResponseProtocolConverter
	anthropicConv *converter.AnthropicProtocolConverter
	exposedModel  string
	responseID    string
	chunkID       string
	timer         *streamTimer
	allChunks     []*dto.OpenAIChatCompletionChunk
	itemState     *converter.StreamItemState
	logger        *zap.Logger
}

func newAnthropicStreamHandler(ctx context.Context, u *openAIUseCase, req *dto.OpenAICreateResponseRequest) *anthropicStreamHandler {
	exposedModel := lo.FromPtr(req.Body.Model)
	h := &anthropicStreamHandler{
		u:             u,
		ctx:           ctx,
		req:           req,
		responseConv:  &converter.ResponseProtocolConverter{},
		anthropicConv: &converter.AnthropicProtocolConverter{},
		exposedModel:  exposedModel,
		responseID:    fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String()),
		chunkID:       fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix),
		itemState:     converter.NewStreamItemState(),
		logger:        logger.WithCtx(ctx),
	}
	assertRespConvInit(h.responseConv, req)
	return h
}

func (h *anthropicStreamHandler) onAnthropicEvent(w *bufio.Writer, event dto.AnthropicSSEEvent) error {
	if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
		h.timer.markFirstToken()
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
		h.allChunks = append(h.allChunks, chunk)
		if _, writeErr := converter.WriteResponseDeltaFromChatChunk(w, chunk, h.itemState, h.responseID, h.responseConv); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func (h *anthropicStreamHandler) finalize(w *bufio.Writer, m *aggregate.Model, endpoint string, anthropicMsg *dto.AnthropicMessage, err error) {
	h.timer.finish()

	rsp := finalizeResponseFromAnthropicStream(h.ctx, w, err, h.allChunks, anthropicMsg, h.exposedModel, h.responseID, h.anthropicConv, h.responseConv)
	h.u.storeResponseFromRsp(h.ctx, h.req, rsp, err, m.Alias().String())
	recordModelCall(h.ctx, h.u.taskSubmitter, callOutcome{
		model:               m,
		exposedModel:        h.exposedModel,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: h.timer.firstLatencyMs,
		streamDurationMs:    h.timer.durationMs,
		usage:               anthropicTokenUsage{anthropicMsg},
		err:                 err,
	})
}

func (u *openAIUseCase) forwardResponseViaAnthropicStreamBody(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, stream io.ReadCloser, endpoint string) func(w *bufio.Writer) {
	h := newAnthropicStreamHandler(ctx, u, req)
	return func(w *bufio.Writer) {
		h.timer = newStreamTimer()
		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventCreated, h.exposedModel, h.responseID); err != nil {
			h.logger.Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
		}
		if err := writeResponseLifecycleEvent(w, enum.ResponseStreamEventInProgress, h.exposedModel, h.responseID); err != nil {
			h.logger.Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
		}
		anthropicMsg, err := u.anthropicProxy.ReadCreateMessageStream(ctx, stream, func(event dto.AnthropicSSEEvent) error {
			return h.onAnthropicEvent(w, event)
		})
		h.finalize(w, m, endpoint, anthropicMsg, err)
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
		u.storeResponseFromRsp(ctx, req, rsp, nil, m.Alias().String())
		recordModelCall(ctx, u.taskSubmitter, callOutcome{
			model:               m,
			exposedModel:        exposedModel,
			endpoint:            endpoint,
			upstreamProtocol:    enum.ProtocolAnthropicMessage,
			apiProtocol:         enum.ProtocolOpenAIResponse,
			firstTokenLatencyMs: totalMs,
			usage:               anthropicTokenUsage{anthropicMsg},
			successStatus:       true,
		})
	})
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
	return converter.FinalizeResponseFromChatCompletion(w, chatCompletion, exposedModel, responseID, responseConv)
}

func assertRespConvInit(conv *converter.ResponseProtocolConverter, req *dto.OpenAICreateResponseRequest) {
	if req == nil || req.Body == nil || len(req.Body.Tools) == 0 {
		return
	}
	conv.SetToolTypeMap(converter.BuildToolTypeMap(req.Body.Tools))
	conv.SetNamespaceMap(converter.BuildNamespaceMap(req.Body.Tools))
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
