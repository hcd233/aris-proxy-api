package usecase

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
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
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func (u *openAIUseCase) forwardChatNative(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) (port.Result, error) {
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) (port.Result, error) {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromOpenAIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat request to anthropic", zap.Error(convErr))
		return nil, proxyutil.SendOpenAIModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	if stream {
		return u.forwardChatViaAnthropicStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardChatViaAnthropicUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &openAIChatNativeStream{
				u:      u,
				ctx:    ctx,
				req:    req,
				m:      m,
				ep:     ep,
				stream: stream,
				timer:  newStreamTimer(),
			}, nil
		},
	}, nil
}

func (u *openAIUseCase) forwardChatNativeUnary(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) (port.Result, error) {
	startTime := time.Now()
	completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, req.Body.Model, ep.Name(), enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	completion.Model = req.Body.Model
	bodyBytes := lo.Must1(sonic.Marshal(completion))

	u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, m.Alias().String())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		exposedModel:        req.Body.Model,
		endpoint:            ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolOpenAIChatCompletion,
		firstTokenLatencyMs: totalMs,
		usage:               openAITokenUsage{completion.Usage},
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

func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &openAIChatViaAnthropicStream{
				u:            u,
				ctx:          ctx,
				req:          req,
				m:            m,
				stream:       stream,
				exposedModel: exposedModel,
				endpoint:     endpoint,
				timer:        newStreamTimer(),
				conv:         &converter.AnthropicProtocolConverter{},
				chunkID:      fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix),
				allChunks:    make([]*dto.OpenAIChatCompletionChunk, 0),
			}, nil
		},
	}, nil
}

func (u *openAIUseCase) forwardChatViaAnthropicUnary(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	conv := &converter.AnthropicProtocolConverter{}
	startTime := time.Now()
	anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, enum.ProtocolOpenAIChatCompletion, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}
	completion, convErr := conv.ToOpenAIResponse(anthropicMsg)
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
	completion.Model = exposedModel
	bodyBytes := lo.Must1(sonic.Marshal(completion))

	u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, m.Alias().String())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		exposedModel:        exposedModel,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolOpenAIChatCompletion,
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

// openAIChatNativeStream 实现 port.Stream，消费 OpenAI Chat Completion 上游流。
type openAIChatNativeStream struct {
	u      *openAIUseCase
	ctx    context.Context
	req    *dto.OpenAIChatCompletionRequest
	m      *aggregate.Model
	ep     *aggregate.Endpoint
	stream io.ReadCloser
	timer  *streamTimer
}

func (s *openAIChatNativeStream) Read(ctx context.Context, sink port.EventSink) error {
	toolCallIDs := make(map[int]string)
	completion, err := s.u.openAIProxy.ReadChatCompletionStream(ctx, s.stream, func(chunk *dto.OpenAIChatCompletionChunk) error {
		if proxyutil.HasNonEmptyDelta(chunk) {
			s.timer.markFirstToken()
		}
		proxyutil.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
		chunk.Model = s.req.Body.Model
		chunkData, marshalErr := sonic.Marshal(chunk)
		if marshalErr != nil {
			logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
			return marshalErr
		}
		return sink.WriteEvent("", chunkData)
	})
	s.timer.finish()
	if err == nil {
		if writeErr := sink.WriteEvent("", []byte(constant.SSEDoneSignal)); writeErr != nil {
			logger.WithCtx(ctx).Debug("[OpenAIUseCase] Failed to write SSE done signal", zap.Error(writeErr))
		}
	} else {
		proxyutil.WriteUpstreamSSEError(ctx, sink, err)
	}

	s.u.storeOpenAIChatFromCompletion(ctx, s.req, completion, err, s.m.Alias().String())

	var usage *dto.OpenAICompletionUsage
	if completion != nil {
		usage = completion.Usage
	}
	recordModelCall(ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		exposedModel:        s.req.Body.Model,
		endpoint:            s.ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolOpenAIChatCompletion,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               openAITokenUsage{usage},
		err:                 err,
	})
	return nil
}

func (s *openAIChatNativeStream) Close() error {
	return nil // ReadChatCompletionStream 内部已经关闭 stream
}

// openAIChatViaAnthropicStream 实现 port.Stream，消费 Anthropic 上游流并转换为 OpenAI Chat chunk。
type openAIChatViaAnthropicStream struct {
	u            *openAIUseCase
	ctx          context.Context
	req          *dto.OpenAIChatCompletionRequest
	m            *aggregate.Model
	stream       io.ReadCloser
	exposedModel string
	endpoint     string
	timer        *streamTimer
	conv         *converter.AnthropicProtocolConverter
	chunkID      string
	allChunks    []*dto.OpenAIChatCompletionChunk
}

func (s *openAIChatViaAnthropicStream) Read(ctx context.Context, sink port.EventSink) error {
	anthropicMsg, err := s.u.anthropicProxy.ReadCreateMessageStream(ctx, s.stream, func(event dto.AnthropicSSEEvent) error {
		if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			s.timer.markFirstToken()
		}
		chunks, convErr := s.conv.ToOpenAISSEResponse(event, s.exposedModel, s.chunkID)
		if convErr != nil {
			return convErr
		}
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			s.allChunks = append(s.allChunks, chunk)
			chunkData, marshalErr := sonic.Marshal(chunk)
			if marshalErr != nil {
				return marshalErr
			}
			if writeErr := sink.WriteEvent("", chunkData); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
	s.timer.finish()
	s.finalizeOpenAIChatStream(ctx, sink, err)

	completion, _ := proxyutil.ConcatChatCompletionChunks(s.allChunks) //nolint:errcheck // store even if concat fails
	if completion != nil {
		completion.Model = s.exposedModel
	}
	s.u.storeOpenAIChatFromCompletion(ctx, s.req, completion, err, s.m.Alias().String())
	recordModelCall(ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		exposedModel:        s.exposedModel,
		endpoint:            s.endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolOpenAIChatCompletion,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               anthropicTokenUsage{anthropicMsg},
		err:                 err,
	})
	return nil
}

func (s *openAIChatViaAnthropicStream) Close() error {
	return nil // ReadCreateMessageStream 内部已经关闭 stream
}

func (s *openAIChatViaAnthropicStream) finalizeOpenAIChatStream(ctx context.Context, sink port.EventSink, err error) {
	if err != nil {
		proxyutil.WriteUpstreamSSEError(ctx, sink, err)
		return
	}
	_ = sink.WriteEvent("", []byte(constant.SSEDoneSignal)) //nolint:errcheck // best-effort write
}

// buildPassthroughHeaders 从 context 取出允许透传的上游响应 header。
// application 只收集 header map，不写入 HTTP context；由 adapter 写入 Huma response。
func buildPassthroughHeaders(ctx context.Context) map[string]string {
	headers := map[string]string{}
	if passthrough := util.GetPassthroughResponseHeaders(ctx); passthrough != nil {
		maps.Copy(headers, passthrough)
	}
	return headers
}
