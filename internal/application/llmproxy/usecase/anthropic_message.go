package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

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
