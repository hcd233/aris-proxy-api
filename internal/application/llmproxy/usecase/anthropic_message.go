package usecase

import (
	"context"
	"io"
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
)

func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, exposedModel string, stream bool) (port.Result, error) {
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardMessageNativeStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardMessageNativeUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *anthropicUseCase) forwardMessageViaChat(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) (port.Result, error) {
	conv := &converter.OpenAIProtocolConverter{}
	chatReq, convErr := conv.FromAnthropicRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert anthropic request to chat", zap.Error(convErr))
		return nil, proxyutil.SendAnthropicModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	if stream {
		return u.forwardMessageViaChatStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardMessageViaChatUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}

func (u *anthropicUseCase) forwardMessageNativeStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.anthropicProxy.OpenCreateMessageStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindAnthropic, anthropicInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindAnthropic,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &anthropicMessageNativeStream{
				u:        u,
				ctx:      ctx,
				req:      req,
				m:        m,
				endpoint: endpoint,
				stream:   stream,
				timer:    newStreamTimer(),
			}, nil
		},
	}, nil
}

func (u *anthropicUseCase) forwardMessageNativeUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	startTime := time.Now()
	anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolAnthropicMessage, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindAnthropic, anthropicInternalErrorBody)
	}
	anthropicMsg.Model = exposedModel
	bodyBytes := lo.Must1(sonic.Marshal(anthropicMsg))

	u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, m.ModelID())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolAnthropicMessage,
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
		Protocol:   enum.ProtocolKindAnthropic,
	}, nil
}

func (u *anthropicUseCase) forwardMessageViaChatStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	startTime := time.Now()
	stream, err := u.openAIProxy.OpenChatCompletionStream(ctx, upstream, body)
	if err != nil {
		totalMs := time.Since(startTime).Milliseconds()
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolAnthropicMessage, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindAnthropic, anthropicInternalErrorBody)
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindAnthropic,
		Open: func(ctx context.Context) (port.Stream, error) {
			return &anthropicMessageViaChatStream{
				u:        u,
				ctx:      ctx,
				req:      req,
				m:        m,
				endpoint: endpoint,
				stream:   stream,
				timer:    newStreamTimer(),
				conv:     &converter.OpenAIProtocolConverter{},
				tracker:  converter.NewSSEContentBlockTracker(),
			}, nil
		},
	}, nil
}

func (u *anthropicUseCase) forwardMessageViaChatUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel, endpoint string, body []byte) (port.Result, error) {
	conv := &converter.OpenAIProtocolConverter{}
	startTime := time.Now()
	completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailureWithProviders(ctx, m, u.taskSubmitter, u.tokenMetrics, exposedModel, endpoint, enum.ProtocolOpenAIChatCompletion, enum.ProtocolAnthropicMessage, totalMs, err)
		return nil, upstreamProxyError(err, enum.ProtocolKindAnthropic, anthropicInternalErrorBody)
	}
	anthropicMsg, convErr := conv.ToAnthropicResponse(completion)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert chat completion to anthropic message", zap.Error(convErr))
		return nil, &port.ProxyError{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
			Body:       anthropicInternalErrorBody,
			Cause:      convErr,
			Protocol:   enum.ProtocolKindAnthropic,
		}
	}
	anthropicMsg.Model = exposedModel
	bodyBytes := lo.Must1(sonic.Marshal(anthropicMsg))

	u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, m.ModelID())
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, callOutcome{
		model:               m,
		endpoint:            endpoint,
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolAnthropicMessage,
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
		Protocol:   enum.ProtocolKindAnthropic,
	}, nil
}

// anthropicMessageNativeStream 实现 port.Stream，消费 Anthropic Messages 上游流。
type anthropicMessageNativeStream struct {
	u            *anthropicUseCase
	ctx          context.Context
	req          *dto.AnthropicCreateMessageRequest
	m            *aggregate.Model
	endpoint     string
	exposedModel string
	stream       io.ReadCloser
	timer        *streamTimer
}

func (s *anthropicMessageNativeStream) Read(ctx context.Context, sink port.EventSink) error {
	anthropicMsg, err := s.u.anthropicProxy.ReadCreateMessageStream(ctx, s.stream, func(event dto.AnthropicSSEEvent) error {
		if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			s.timer.markFirstToken()
		}
		modifiedData := proxyutil.ReplaceModelInSSEData(event.Data, s.exposedModel)
		return sink.WriteEvent(event.Event, modifiedData)
	})
	s.timer.finish()
	if err == nil {
		if writeErr := proxyutil.WriteAnthropicMessageStop(sink); writeErr != nil {
			logger.WithCtx(ctx).Debug("[AnthropicUseCase] Failed to write message_stop", zap.Error(writeErr))
		}
	} else {
		proxyutil.WriteUpstreamSSEError(ctx, sink, err, enum.ProtocolKindAnthropic)
	}

	s.u.storeAnthropicFromMsg(ctx, s.req, anthropicMsg, err, s.m.ModelID())
	recordModelCall(ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		endpoint:            s.endpoint,
		upstreamProtocol:    enum.ProtocolAnthropicMessage,
		apiProtocol:         enum.ProtocolAnthropicMessage,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               anthropicTokenUsage{anthropicMsg},
		err:                 err,
	})
	return nil
}

func (s *anthropicMessageNativeStream) Close() error {
	return nil // ReadCreateMessageStream 内部已经关闭 stream
}

// anthropicMessageViaChatStream 实现 port.Stream，消费 OpenAI Chat 上游流并转换为 Anthropic 事件。
type anthropicMessageViaChatStream struct {
	u            *anthropicUseCase
	ctx          context.Context
	req          *dto.AnthropicCreateMessageRequest
	m            *aggregate.Model
	endpoint     string
	exposedModel string
	stream       io.ReadCloser
	timer        *streamTimer
	conv         *converter.OpenAIProtocolConverter
	tracker      *converter.SSEContentBlockTracker
	isFirst      bool
}

func (s *anthropicMessageViaChatStream) Read(ctx context.Context, sink port.EventSink) error {
	s.isFirst = true
	completion, err := s.u.openAIProxy.ReadChatCompletionStream(ctx, s.stream, func(chunk *dto.OpenAIChatCompletionChunk) error {
		if proxyutil.HasNonEmptyDelta(chunk) {
			s.timer.markFirstToken()
		}
		events, convErr := s.conv.ToAnthropicSSEResponse(chunk, s.isFirst, s.exposedModel, s.tracker)
		s.isFirst = false
		if convErr != nil {
			return convErr
		}
		for _, event := range events {
			if writeErr := sink.WriteEvent(event.Event, event.Data); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
	s.timer.finish()
	anthropicMsg := s.finalizeAnthropicChatStream(ctx, sink, completion, err)

	s.u.storeAnthropicFromMsg(ctx, s.req, anthropicMsg, err, s.m.ModelID())

	var usage *dto.OpenAICompletionUsage
	if completion != nil {
		usage = completion.Usage
	}
	recordModelCall(ctx, s.u.taskSubmitter, s.u.tokenMetrics, callOutcome{
		model:               s.m,
		endpoint:            s.endpoint,
		upstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		apiProtocol:         enum.ProtocolAnthropicMessage,
		firstTokenLatencyMs: s.timer.firstLatencyMs,
		streamDurationMs:    s.timer.durationMs,
		usage:               openAITokenUsage{usage},
		err:                 err,
	})
	return nil
}

func (s *anthropicMessageViaChatStream) Close() error {
	return nil // ReadChatCompletionStream 内部已经关闭 stream
}

func (s *anthropicMessageViaChatStream) finalizeAnthropicChatStream(ctx context.Context, sink port.EventSink, completion *dto.OpenAIChatCompletion, upstreamErr error) *dto.AnthropicMessage {
	if upstreamErr != nil {
		proxyutil.WriteUpstreamSSEError(ctx, sink, upstreamErr, enum.ProtocolKindAnthropic)
		return nil
	}
	var anthropicMsg *dto.AnthropicMessage
	if completion != nil {
		anthropicMsg, _ = s.conv.ToAnthropicResponse(completion) //nolint:errcheck // best-effort conversion
		if anthropicMsg != nil {
			anthropicMsg.Model = s.exposedModel
		}
	}
	_ = proxyutil.WriteAnthropicMessageStop(sink) //nolint:errcheck // best-effort write
	return anthropicMsg
}
