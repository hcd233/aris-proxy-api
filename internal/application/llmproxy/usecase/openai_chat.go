// Package usecase LLMProxy 域用例层 — ChatCompletion 原生协议转发
//
//	@author centonhuang
//	@update 2026-04-28 20:00:00
package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// forwardChatNative OpenAI 原生协议转发（provider=openai）
//
// 注意：此前实现会将 req.Body.MaxTokens 无条件改写为 MaxCompletionTokens，这会导致
//  两类问题：
//   1. 第三方 OpenAI 协议聚合网关（如 api.chatanywhere.tech）只认识 max_tokens，
//      对 max_completion_tokens 直接忽略，相当于把用户设置的生成上限抹掉，从而触发
//      上游模型跑飞/超长（如 503 "模型无返回结果"）；
//   2. 即便上游是官方 OpenAI，max_tokens 对 GPT-4 / GPT-3.5 系列仍被官方接受，
//      把字段 silent 改名会违背透传语义。
//
// 因此这里保持用户请求的原样透传，由调用方自行选择 max_tokens 或 max_completion_tokens。
func (u *openAIUseCase) forwardChatNative(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
	body = util.EnsureAssistantMessageReasoningContent(body)

	if stream {
		return u.forwardChatNativeStream(ctx, log, req, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, log, req, ep, upstream, body)
}

// forwardChatNativeStream OpenAI 原生流式：SSE chunks → 客户端
func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		toolCallIDs := make(map[int]string)

		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			util.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
			chunk.Model = req.Body.Model
			chunkData, marshalErr := sonic.Marshal(chunk)
			if marshalErr != nil {
				log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
				return marshalErr
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
			return w.Flush()
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			_ = w.Flush()
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, err, upstream.Model)

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromOpenAIUsage(usage)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardChatNativeUnary OpenAI 原生非流式：JSON → 客户端
func (u *openAIUseCase) forwardChatNativeUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardChatViaAnthropic OpenAI 请求通过 Anthropic 上游转发
func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromOpenAIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError()
	}
	anthropicReq.Model = upstream.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return u.forwardChatViaAnthropicStream(ctx, log, req, ep, upstream, body, &conv)
	}
	return u.forwardChatViaAnthropicUnary(ctx, log, req, ep, upstream, body, &conv)
}

// forwardChatViaAnthropicStream Anthropic 上游流式 → OpenAI SSE
func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64

		chunkID := converter.GenerateOpenAIChunkID()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			chunks, convErr := conv.ToOpenAISSEResponse(event, req.Body.Model, chunkID)
			if convErr != nil {
				return convErr
			}
			for _, chunk := range chunks {
				if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
					firstTokenTime = time.Now()
					firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				chunkData, marshalErr := sonic.Marshal(chunk)
				if marshalErr != nil {
					log.Error("[OpenAIUseCase] Failed to marshal chunk", zap.Error(marshalErr))
					return marshalErr
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
				if flushErr := w.Flush(); flushErr != nil {
					return flushErr
				}
			}
			return nil
		})
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if err == nil {
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			_ = w.Flush()
		} else {
			util.WriteUpstreamSSEError(log, w, err)
		}

		u.storeOpenAIChatFromAnthropicMsg(ctx, log, req, anthropicMsg, err, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardChatViaAnthropicUnary Anthropic 上游非流式 → OpenAI JSON
func (u *openAIUseCase) forwardChatViaAnthropicUnary(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, openAIInternalErrorBody)
			auditFailure(ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, log, req, completion, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}
