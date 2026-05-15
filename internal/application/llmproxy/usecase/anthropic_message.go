package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	body := util.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardMessageNativeStream(ctx, req, m, upstream, exposedModel, body)
	}
	return u.forwardMessageNativeUnary(ctx, req, m, upstream, exposedModel, body)
}

func (u *anthropicUseCase) forwardMessageViaChat(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	chatReq, convErr := conv.FromAnthropicRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert anthropic request to chat", zap.Error(convErr))
		return util.SendAnthropicModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false)
	body := util.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	if stream {
		return u.forwardMessageViaChatStream(ctx, req, m, upstream, exposedModel, body)
	}
	return u.forwardMessageViaChatUnary(ctx, req, m, upstream, exposedModel, body)
}

func (u *anthropicUseCase) forwardMessageNativeStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			modifiedData := util.ReplaceModelInSSEData(event.Data, exposedModel)
			if _, writeErr := fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event); writeErr != nil {
				log.Debug("[AnthropicUseCase] Failed to write SSE event line", zap.Error(writeErr))
			}
			if _, dataErr := fmt.Fprintf(w, constant.SSEDataLineTemplate, modifiedData); dataErr != nil {
				log.Debug("[AnthropicUseCase] Failed to write SSE data line", zap.Error(dataErr))
			}
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_ = util.WriteAnthropicMessageStop(w)
		} else {
			util.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    enum.ProviderAnthropic,
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *anthropicUseCase) forwardMessageNativeUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, exposedModel, enum.ProviderAnthropic, totalMs, err)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    enum.ProviderAnthropic,
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *anthropicUseCase) forwardMessageViaChatStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.OpenAIProtocolConverter{}
	tracker := converter.NewSSEContentBlockTracker()
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		isFirst := true
		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			events, convErr := conv.ToAnthropicSSEResponse(chunk, isFirst, exposedModel, tracker)
			isFirst = false
			if convErr != nil {
				log.Debug("[AnthropicUseCase] Failed to convert chat chunk to anthropic SSE", zap.Error(convErr))
				return nil
			}
			for _, event := range events {
				if _, writeErr := fmt.Fprintf(w, constant.SSEEventLineTemplate, event.Event); writeErr != nil {
					log.Debug("[AnthropicUseCase] Failed to write converted SSE event", zap.Error(writeErr))
				}
				if _, dataErr := fmt.Fprintf(w, constant.SSEDataLineTemplate, event.Data); dataErr != nil {
					log.Debug("[AnthropicUseCase] Failed to write converted SSE data", zap.Error(dataErr))
				}
			}
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		var anthropicMsg *dto.AnthropicMessage
		if err == nil {
			if completion != nil {
				anthropicMsg, _ = conv.ToAnthropicResponse(completion)
				if anthropicMsg != nil {
					anthropicMsg.Model = exposedModel
				}
			}
			_ = util.WriteAnthropicMessageStop(w)
		} else {
			util.WriteUpstreamSSEError(ctx, w, err)
		}
		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    enum.ProviderOpenAI,
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		if completion != nil {
			task.SetTokensFromOpenAIUsage(completion.Usage)
		}
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *anthropicUseCase) forwardMessageViaChatUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
			auditFailureWithProviders(u.taskSubmitter, ctx, m, exposedModel, enum.ProviderOpenAI, enum.ProviderAnthropic, totalMs, err)
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
		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, upstream.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               exposedModel,
			UpstreamProvider:    enum.ProviderOpenAI,
			APIProvider:         enum.ProviderAnthropic,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
