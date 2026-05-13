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

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func (u *openAIUseCase) forwardResponseNative(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := util.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardResponseNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardResponseNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardResponseNativeStream(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
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
			replaced := util.ReplaceModelInSSEData(data, lo.FromPtr(req.Body.Model))
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
			ModelID:             m.AggregateID(),
			Model:               lo.FromPtr(req.Body.Model),
			UpstreamProvider:    enum.ProviderOpenAI,
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

func (u *openAIUseCase) forwardResponseNativeUnary(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		respBody, err := u.openAIProxy.ForwardCreateResponse(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, lo.FromPtr(req.Body.Model), enum.ProviderOpenAI, totalMs, err)
			return
		}

		replaced := util.ReplaceModelInBody(respBody, lo.FromPtr(req.Body.Model))
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
			ModelID:             m.AggregateID(),
			Model:               lo.FromPtr(req.Body.Model),
			UpstreamProvider:    enum.ProviderOpenAI,
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
