// Package usecase LLMProxy 域用例层 — Response API 协议转发
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
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// forwardResponseNative OpenAI 原生 Response API 转发
func (u *openAIUseCase) forwardResponseNative(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := util.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardResponseNativeStream(ctx, req, ep, upstream, body)
	}
	return u.forwardResponseNativeUnary(ctx, req, ep, upstream, body)
}

// forwardResponseNativeStream Response API 原生流式
func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var finalResponse *dto.OpenAICreateResponseRsp

		proxyErr := u.openAIProxy.ForwardCreateResponseStream(ctx, upstream, body, func(event string, data []byte) error {
			if firstTokenTime.IsZero() && util.IsResponseAPIDeltaEvent(event) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			if finalResponse == nil && util.IsResponseAPITerminalEvent(event) {
				var ev dto.ResponseStreamTerminalEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Warn("[OpenAIUseCase] Failed to parse response terminal event",
						zap.String("event", event), zap.Error(err))
				} else {
					finalResponse = ev.Response
				}
			}
			replaced := util.ReplaceModelInSSEData(data, req.Body.Model)
			if _, writeErr := fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, replaced); writeErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE event frame", zap.Error(writeErr))
			}
			return w.Flush()
		})

		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIUseCase] Response API stream error", zap.Error(proxyErr))
			util.WriteUpstreamSSEError(ctx, w, proxyErr)
		}

		u.storeResponseFromRsp(ctx, req, finalResponse, proxyErr, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromResponseUsage(finalResponse)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(proxyErr)
		task.SetErrorFromResponseStatus(finalResponse)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

// forwardResponseNativeUnary Response API 原生非流式（直写 raw bytes）
func (u *openAIUseCase) forwardResponseNativeUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		respBody, err := u.openAIProxy.ForwardCreateResponse(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}

		replaced := util.ReplaceModelInBody(respBody, req.Body.Model)
		if headers := util.GetPassthroughResponseHeaders(ctx); headers != nil {
			for k, v := range headers {
				writer.HumaCtx.SetHeader(k, v)
			}
		}
		writer.HumaCtx.SetStatus(fiber.StatusOK)
		writer.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write(replaced)

		var rsp dto.OpenAICreateResponseRsp
		parseErr := sonic.Unmarshal(respBody, &rsp)
		if parseErr != nil {
			log.Warn("[OpenAIUseCase] Failed to parse Response API non-stream body", zap.Error(parseErr))
		} else {
			u.storeResponseFromRsp(ctx, req, &rsp, nil, upstream.Model)
		}

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             ep.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    ep.Provider(),
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		if parseErr == nil {
			task.SetTokensFromResponseUsage(&rsp)
			task.SetErrorFromResponseStatus(&rsp)
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

// forwardResponseViaAnthropic Response API 通过 Anthropic 上游转发
func (u *openAIUseCase) forwardResponseViaAnthropic(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromResponseAPIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError()
	}
	anthropicReq.Model = upstream.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return u.forwardResponseViaAnthropicStream(ctx, req, ep, upstream, body, &conv)
	}
	return u.forwardResponseViaAnthropicUnary(ctx, req, ep, upstream, body, &conv)
}

// forwardResponseViaAnthropicStream Anthropic 上游流式 → OpenAI chat chunk（Response API 跨协议变体）
func (u *openAIUseCase) forwardResponseViaAnthropicStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
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
				if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, chunkData); writeErr != nil {
					log.Debug("[OpenAIUseCase] Failed to write SSE chunk", zap.Error(writeErr))
				}
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
			if _, doneErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, constant.SSEDoneSignal); doneErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE done signal", zap.Error(doneErr))
			}
			if flushErr := w.Flush(); flushErr != nil {
				log.Debug("[OpenAIUseCase] Failed to flush SSE writer", zap.Error(flushErr))
			}
		} else {
			util.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeResponseFromAnthropicMsg(ctx, req, anthropicMsg, err, upstream.Model)

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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

// forwardResponseViaAnthropicUnary Anthropic 上游非流式 → OpenAI chat JSON（Response API 跨协议变体）
func (u *openAIUseCase) forwardResponseViaAnthropicUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessage(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, ep, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeResponseFromAnthropicMsg(ctx, req, anthropicMsg, nil, upstream.Model)

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
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
