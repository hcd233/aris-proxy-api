package usecase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func (u *openAIUseCase) forwardResponseNative(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) (port.Result, error) {
	body := proxyutil.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardResponseNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardResponseNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardResponseViaChat(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint) (port.Result, error) {
	conv := &converter.ResponseProtocolConverter{}
	chatReq, convErr := conv.FromResponseRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert response request to chat", zap.Error(convErr))
		return nil, proxyutil.SendOpenAIModelNotFoundError(lo.FromPtr(req.Body.Model))
	}
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	stream := req.Body.Stream != nil && *req.Body.Stream
	if stream {
		return u.forwardResponseViaChatStream(ctx, req, m, upstream, ep.Name(), body)
	}
	return u.forwardResponseViaChatUnary(ctx, req, m, upstream, ep.Name(), body)
}

func (u *openAIUseCase) forwardResponseViaAnthropic(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint) (port.Result, error) {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromResponseAPIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert response request to anthropic", zap.Error(convErr))
		return nil, proxyutil.SendOpenAIModelNotFoundError(lo.FromPtr(req.Body.Model))
	}
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	stream := req.Body.Stream != nil && *req.Body.Stream
	if stream {
		return u.forwardResponseViaAnthropicStream(ctx, req, m, upstream, ep.Name(), body)
	}
	return u.forwardResponseViaAnthropicUnary(ctx, req, m, upstream, ep.Name(), body)
}

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenCreateResponseStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &responseNativeStream{
				u:                 u,
				ctx:               ctx,
				req:               req,
				m:                 m,
				ep:                ep,
				stream:            stream,
				timer:             newStreamTimer(),
				accumulatedOutput: make([]*dto.ResponseInputItem, 0),
			}, nil
		},
	}, nil
}

func (u *openAIUseCase) forwardResponseNativeUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) (port.Result, error) {
	log := logger.WithCtx(ctx)
	startTime := time.Now()
	respBody, err := u.openAIProxy.ForwardCreateResponse(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, lo.FromPtr(req.Body.Model), ep.Name(), enum.ProtocolOpenAIResponse, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}

	replaced := proxyutil.ReplaceModelInBody(respBody, lo.FromPtr(req.Body.Model))
	headers := buildPassthroughHeaders(ctx)
	headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON

	var rsp dto.OpenAICreateResponseRsp
	out := callOutcome{
		model:               m,
		endpoint:            ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIResponse,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: totalMs,
		successStatus:       true,
	}
	if parseErr := sonic.Unmarshal(replaced, &rsp); parseErr != nil {
		log.Debug("[OpenAIUseCase] Failed to parse Response API non-stream body", zap.Error(parseErr))
	} else {
		u.storeResponseFromRsp(ctx, req, &rsp, nil, m.ModelID())
		out.usage = responseTokenUsage{&rsp}
		out.responseStatus = &rsp
	}
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, out)

	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       replaced,
		Protocol:   enum.ProtocolKindOpenAI,
	}, nil
}

func (u *openAIUseCase) forwardResponseViaChatStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) (port.Result, error) {
	exposedModel := lo.FromPtr(req.Body.Model)
	responseID := fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String())
	startTime := time.Now()
	stream, openErr := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if openErr != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolOpenAIResponse, totalMs, openErr)
		return nil, upstreamProxyError(openErr, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &responseViaChatStream{
				u:          u,
				ctx:        ctx,
				req:        req,
				m:          m,
				endpoint:   endpoint,
				responseID: responseID,
				stream:     stream,
				timer:      newStreamTimer(),
				conv:       &converter.ResponseProtocolConverter{},
				itemState:  converter.NewStreamItemState(),
			}, nil
		},
	}, nil
}

func (u *openAIUseCase) forwardResponseViaChatUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) (port.Result, error) {
	conv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(conv, req)
	exposedModel := lo.FromPtr(req.Body.Model)
	startTime := time.Now()
	completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolOpenAIResponse, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	completion.Model = exposedModel
	rsp, convErr := conv.ToResponseResponse(completion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
		return nil, &port.ProxyError{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
			Body:       openAIInternalErrorBody,
			Cause:      convErr,
			Protocol:   enum.ProtocolKindOpenAI,
		}
	}
	bodyBytes := lo.Must1(sonic.Marshal(rsp))

	u.storeResponseFromRsp(ctx, req, rsp, nil, m.ModelID())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: totalMs,
		usage:               responseTokenUsage{rsp},
		successStatus:       true,
	})

	headers := buildPassthroughHeaders(ctx)
	headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       bodyBytes,
		Protocol:   enum.ProtocolKindOpenAI,
	}, nil
}

func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		exposedModel := lo.FromPtr(req.Body.Model)
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(ctx context.Context) (port.Stream, error) {
			return newResponseViaAnthropicStream(ctx, u, req, m, stream, endpoint), nil
		},
	}, nil
}

func (u *openAIUseCase) forwardResponseViaAnthropicUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, endpoint string, body []byte) (port.Result, error) {
	anthropicConv := &converter.AnthropicProtocolConverter{}
	responseConv := &converter.ResponseProtocolConverter{}
	assertRespConvInit(responseConv, req)
	exposedModel := lo.FromPtr(req.Body.Model)
	startTime := time.Now()
	anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIResponse, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	chatCompletion, convErr := anthropicConv.ToOpenAIResponse(anthropicMsg)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert anthropic message to chat completion", zap.Error(convErr))
		return nil, &port.ProxyError{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
			Body:       openAIInternalErrorBody,
			Cause:      convErr,
			Protocol:   enum.ProtocolKindOpenAI,
		}
	}
	chatCompletion.Model = exposedModel
	rsp, convErr := responseConv.ToResponseResponse(chatCompletion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat completion to response", zap.Error(convErr))
		return nil, &port.ProxyError{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
			Body:       openAIInternalErrorBody,
			Cause:      convErr,
			Protocol:   enum.ProtocolKindOpenAI,
		}
	}
	bodyBytes := lo.Must1(sonic.Marshal(rsp))

	u.storeResponseFromRsp(ctx, req, rsp, nil, m.ModelID())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: totalMs,
		usage:               anthropicTokenUsage{anthropicMsg},
		successStatus:       true,
	})

	headers := buildPassthroughHeaders(ctx)
	headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       bodyBytes,
		Protocol:   enum.ProtocolKindOpenAI,
	}, nil
}

// responseNativeStream 实现 port.Stream，消费 OpenAI Responses API 上游流。
type responseNativeStream struct {
	u                 *openAIUseCase
	ctx               context.Context
	req               *dto.OpenAICreateResponseRequest
	m                 *aggregate.Model
	ep                *aggregate.Endpoint
	stream            io.ReadCloser
	timer             *streamTimer
	finalResponse     *dto.OpenAICreateResponseRsp
	accumulatedOutput []*dto.ResponseInputItem
}

func (s *responseNativeStream) Read(ctx context.Context, sink port.EventSink) error {
	proxyErr := s.u.openAIProxy.ReadCreateResponseStream(ctx, s.stream, func(event string, data []byte) error {
		return s.onEvent(sink, event, data)
	})
	s.finalize(sink, proxyErr)
	return nil
}

func (s *responseNativeStream) Close() error {
	return nil // ReadCreateResponseStream 内部已经关闭 stream
}

func (s *responseNativeStream) onEvent(sink port.EventSink, event string, data []byte) error {
	if proxyutil.IsResponseAPIDeltaEvent(event) {
		s.timer.markFirstToken()
	}
	s.handleOutputItemDone(event, data)
	s.handleTerminalEvent(event, data)

	outgoingData := s.patchTerminalOutput(event, data)
	replaced := proxyutil.ReplaceModelInSSEData(outgoingData, lo.FromPtr(s.req.Body.Model))
	return sink.WriteEvent(event, replaced)
}

func (s *responseNativeStream) handleOutputItemDone(event string, data []byte) {
	if event != enum.ResponseStreamEventOutputItemDone {
		return
	}
	var ev dto.ResponseStreamOutputItemDoneEvent
	if err := sonic.Unmarshal(data, &ev); err != nil {
		logger.WithCtx(s.ctx).Debug("[OpenAIUseCase] Failed to parse output_item.done event", zap.Error(err))
		return
	}
	if ev.Item == nil {
		return
	}
	s.accumulatedOutput = append(s.accumulatedOutput, ev.Item)
}

func (s *responseNativeStream) handleTerminalEvent(event string, data []byte) {
	if s.finalResponse != nil || !proxyutil.IsResponseAPITerminalEvent(event) {
		return
	}
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(data, &ev); err != nil {
		logger.WithCtx(s.ctx).Warn("[OpenAIUseCase] Failed to parse response terminal event",
			zap.String("event", event), zap.Error(err))
		return
	}
	s.finalResponse = ev.Response
	if s.finalResponse == nil {
		return
	}
}

func (s *responseNativeStream) patchTerminalOutput(event string, data []byte) []byte {
	if !proxyutil.IsResponseAPITerminalEvent(event) {
		return data
	}
	patched, changed, err := proxyutil.FillResponseTerminalOutput(data, s.accumulatedOutput)
	if err != nil {
		logger.WithCtx(s.ctx).Warn("[OpenAIUseCase] Failed to fill response terminal output", zap.String("event", event), zap.Error(err))
		return data
	}
	if !changed {
		return data
	}
	if s.finalResponse != nil {
		s.finalResponse.Output = s.accumulatedOutput
	}
	return patched
}

func (s *responseNativeStream) finalize(sink port.EventSink, proxyErr error) {
	s.timer.finish()
	if proxyErr != nil {
		logger.WithCtx(s.ctx).Error("[OpenAIUseCase] Native response stream error", zap.Error(proxyErr))
		proxyutil.WriteUpstreamSSEError(s.ctx, sink, proxyErr, enum.ProtocolKindOpenAI)
	}
	if s.finalResponse != nil && len(s.finalResponse.Output) == 0 && len(s.accumulatedOutput) > 0 {
		s.finalResponse.Output = s.accumulatedOutput
	}
	s.u.storeResponseFromRsp(s.ctx, s.req, s.finalResponse, proxyErr, s.m.ModelID())

	recordModelCall(s.ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		endpoint:            s.ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIResponse,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               responseTokenUsage{s.finalResponse},
		err:                 proxyErr,
		responseStatus:      s.finalResponse,
	})
}

// responseViaChatStream 实现 port.Stream，消费 OpenAI Chat 上游流并转换为 Responses API 事件。
type responseViaChatStream struct {
	u            *openAIUseCase
	ctx          context.Context
	req          *dto.OpenAICreateResponseRequest
	m            *aggregate.Model
	endpoint     string
	exposedModel string
	responseID   string
	stream       io.ReadCloser
	timer        *streamTimer
	conv         *converter.ResponseProtocolConverter
	itemState    *converter.StreamItemState
}

func (s *responseViaChatStream) Read(ctx context.Context, sink port.EventSink) error {
	if err := writeResponseLifecycleEvent(sink, enum.ResponseStreamEventCreated, s.exposedModel, s.responseID); err != nil {
		logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
	}
	if err := writeResponseLifecycleEvent(sink, enum.ResponseStreamEventInProgress, s.exposedModel, s.responseID); err != nil {
		logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
	}

	completion, err := s.u.openAIProxy.ReadChatCompletionStream(ctx, s.stream, func(chunk *dto.OpenAIChatCompletionChunk) error {
		hasWritten, writeErr := converter.WriteResponseDeltaFromChatChunk(sink, chunk, s.itemState, s.responseID, s.conv)
		if hasWritten {
			s.timer.markFirstToken()
		}
		return writeErr
	})
	s.timer.finish()

	var rsp *dto.OpenAICreateResponseRsp
	if err != nil {
		proxyutil.WriteUpstreamSSEError(ctx, sink, err, enum.ProtocolKindOpenAI)
	} else {
		rsp = converter.FinalizeResponseFromChatCompletion(sink, completion, s.exposedModel, s.responseID, s.conv)
	}
	s.u.storeResponseFromRsp(ctx, s.req, rsp, err, s.m.ModelID())
	recordModelCall(ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		endpoint:            s.endpoint,
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               responseTokenUsage{rsp},
		err:                 err,
	})
	return nil
}

func (s *responseViaChatStream) Close() error {
	return nil // ReadChatCompletionStream 内部已经关闭 stream
}

// responseViaAnthropicStream 实现 port.Stream，消费 Anthropic 上游流并转换为 Responses API 事件。
type responseViaAnthropicStream struct {
	u             *openAIUseCase
	ctx           context.Context
	req           *dto.OpenAICreateResponseRequest
	m             *aggregate.Model
	endpoint      string
	exposedModel  string
	responseID    string
	chunkID       string
	stream        io.ReadCloser
	timer         *streamTimer
	responseConv  *converter.ResponseProtocolConverter
	anthropicConv *converter.AnthropicProtocolConverter
	itemState     *converter.StreamItemState
	agg           *proxyutil.ChatCompletionStreamAggregator
}

func newResponseViaAnthropicStream(ctx context.Context, u *openAIUseCase, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, stream io.ReadCloser, endpoint string) *responseViaAnthropicStream {
	h := &responseViaAnthropicStream{
		u:             u,
		ctx:           ctx,
		req:           req,
		m:             m,
		endpoint:      endpoint,
		responseID:    fmt.Sprintf(constant.ResponseIDTemplate, uuid.New().String()),
		chunkID:       fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix),
		stream:        stream,
		timer:         newStreamTimer(),
		responseConv:  &converter.ResponseProtocolConverter{},
		anthropicConv: &converter.AnthropicProtocolConverter{},
		itemState:     converter.NewStreamItemState(),
		agg:           proxyutil.NewChatCompletionStreamAggregator(),
	}
	assertRespConvInit(h.responseConv, req)
	return h
}

func (s *responseViaAnthropicStream) Read(ctx context.Context, sink port.EventSink) error {
	if err := writeResponseLifecycleEvent(sink, enum.ResponseStreamEventCreated, s.exposedModel, s.responseID); err != nil {
		logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to write response.created", zap.Error(err))
	}
	if err := writeResponseLifecycleEvent(sink, enum.ResponseStreamEventInProgress, s.exposedModel, s.responseID); err != nil {
		logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to write response.in_progress", zap.Error(err))
	}
	anthropicMsg, err := s.u.anthropicProxy.ReadCreateMessageStream(ctx, s.stream, func(event dto.AnthropicSSEEvent) error {
		return s.onAnthropicEvent(sink, event)
	})
	s.finalize(sink, anthropicMsg, err)
	return nil
}

func (s *responseViaAnthropicStream) Close() error {
	return nil // ReadCreateMessageStream 内部已经关闭 stream
}

func (s *responseViaAnthropicStream) onAnthropicEvent(sink port.EventSink, event dto.AnthropicSSEEvent) error {
	if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
		s.timer.markFirstToken()
	}
	chunks, convErr := s.anthropicConv.ToOpenAISSEResponse(event, s.exposedModel, s.chunkID)
	if convErr != nil {
		logger.WithCtx(s.ctx).Error("[OpenAIUseCase] Failed to convert anthropic SSE to chat chunk", zap.Error(convErr))
		return convErr
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		s.agg.Add(chunk)
		if _, writeErr := converter.WriteResponseDeltaFromChatChunk(sink, chunk, s.itemState, s.responseID, s.responseConv); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func (s *responseViaAnthropicStream) finalize(sink port.EventSink, anthropicMsg *dto.AnthropicMessage, err error) {
	s.timer.finish()
	rsp := finalizeResponseFromAnthropicStream(s.ctx, sink, err, s.agg.Completion(), anthropicMsg, s.exposedModel, s.responseID, s.anthropicConv, s.responseConv)
	s.u.storeResponseFromRsp(s.ctx, s.req, rsp, err, s.m.ModelID())
	recordModelCall(s.ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		endpoint:            s.endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolOpenAIResponse,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               anthropicTokenUsage{anthropicMsg},
		err:                 err,
	})
}

func finalizeResponseFromAnthropicStream(ctx context.Context, sink port.EventSink, upstreamErr error, chatCompletion *dto.OpenAIChatCompletion, anthropicMsg *dto.AnthropicMessage, exposedModel, responseID string, anthropicConv *converter.AnthropicProtocolConverter, responseConv *converter.ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
	if upstreamErr != nil {
		proxyutil.WriteUpstreamSSEError(ctx, sink, upstreamErr, enum.ProtocolKindOpenAI)
		return nil
	}
	if chatCompletion == nil && anthropicMsg != nil {
		chatCompletion, _ = anthropicConv.ToOpenAIResponse(anthropicMsg) //nolint:errcheck // best-effort fallback conversion
	}
	if chatCompletion == nil {
		return nil
	}
	return converter.FinalizeResponseFromChatCompletion(sink, chatCompletion, exposedModel, responseID, responseConv)
}

func assertRespConvInit(conv *converter.ResponseProtocolConverter, req *dto.OpenAICreateResponseRequest) {
	if req == nil || req.Body == nil || len(req.Body.Tools) == 0 {
		return
	}
	conv.SetToolTypeMap(converter.BuildToolTypeMap(req.Body.Tools))
	conv.SetNamespaceMap(converter.BuildNamespaceMap(req.Body.Tools))
}

func writeResponseLifecycleEvent(sink port.EventSink, event enum.ResponseStreamEventType, model, responseID string) error {
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
	return sink.WriteEvent(event, payload)
}
