package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func (u *anthropicUseCase) forwardMessageNativeStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
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
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_ = proxyutil.WriteAnthropicMessageStop(w)
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, upstream.Model)

		task := newAuditTask(ctx, m, exposedModel, enum.ProviderAnthropic, enum.ProviderAnthropic, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *anthropicUseCase) forwardMessageViaChatStream(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.OpenAIProtocolConverter{}
	tracker := converter.NewSSEContentBlockTracker()
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
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
				log.Error("[AnthropicUseCase] Failed to convert chat chunk to anthropic SSE", zap.Error(convErr))
				return convErr
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
			_ = proxyutil.WriteAnthropicMessageStop(w)
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}
		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, upstream.Model)
		task := newAuditTask(ctx, m, exposedModel, enum.ProviderOpenAI, enum.ProviderAnthropic, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		if completion != nil {
			task.SetTokensFromOpenAIUsage(completion.Usage)
		}
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
