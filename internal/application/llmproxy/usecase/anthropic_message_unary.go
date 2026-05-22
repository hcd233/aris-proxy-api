package usecase

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func (u *anthropicUseCase) forwardMessageNativeUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, exposedModel, enum.ProviderAnthropic, totalMs, err)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		u.storeAnthropicFromMsg(ctx, req, anthropicMsg, nil, upstream.Model)

		task := newAuditTask(ctx, m, exposedModel, enum.ProviderAnthropic, enum.ProviderAnthropic, totalMs)
		task.UpstreamStatusCode = fiber.StatusOK
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *anthropicUseCase) forwardMessageViaChatUnary(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	return apiutil.WrapJSONResponse(ctx, func(writer apiutil.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			apiutil.WriteUpstreamError(writer, err, anthropicInternalErrorBody)
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
		task := newAuditTask(ctx, m, exposedModel, enum.ProviderOpenAI, enum.ProviderAnthropic, totalMs)
		task.UpstreamStatusCode = fiber.StatusOK
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
