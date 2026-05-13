package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
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

func (u *openAIUseCase) forwardChatNative(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := util.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)

	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
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
			if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, chunkData); writeErr != nil {
				log.Debug("[OpenAIUseCase] Failed to write SSE chunk", zap.Error(writeErr))
			}
			return w.Flush()
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

		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model)

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    enum.ProviderOpenAI,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromOpenAIUsage(usage)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardChatNativeUnary(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapJSONResponse(ctx, func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := u.openAIProxy.ForwardChatCompletion(ctx, upstream, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(writer, err, openAIInternalErrorBody)
			auditFailure(u.taskSubmitter, ctx, m, req.Body.Model, enum.ProviderOpenAI, totalMs, err)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		u.storeOpenAIChatFromCompletion(ctx, req, completion, nil, upstream.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             m.AggregateID(),
			Model:               req.Body.Model,
			UpstreamProvider:    enum.ProviderOpenAI,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
