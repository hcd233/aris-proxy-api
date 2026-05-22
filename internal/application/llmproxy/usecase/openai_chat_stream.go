package usecase

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
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

func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		toolCallIDs := make(map[int]string)

		completion, err := u.openAIProxy.ForwardChatCompletionStream(ctx, upstream, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			proxyutil.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
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
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}

		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model)

		var usage *dto.OpenAICompletionUsage
		if completion != nil {
			usage = completion.Usage
		}
		task := newAuditTask(ctx, m, req.Body.Model, enum.ProviderOpenAI, enum.ProviderOpenAI, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		task.SetTokensFromOpenAIUsage(usage)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}

func (u *openAIUseCase) forwardChatViaAnthropicStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, upstream vo.UpstreamEndpoint, exposedModel string, body []byte) *huma.StreamResponse {
	log := logger.WithCtx(ctx)
	conv := &converter.AnthropicProtocolConverter{}
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, constant.ConvertedChunkIDSuffix)
	return apiutil.WrapStreamResponse(ctx, func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs, streamDurationMs int64
		var allChunks []*dto.OpenAIChatCompletionChunk

		anthropicMsg, err := u.anthropicProxy.ForwardCreateMessageStream(ctx, upstream, body, func(event dto.AnthropicSSEEvent) error {
			if firstTokenTime.IsZero() && event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			chunks, convErr := conv.ToOpenAISSEResponse(event, exposedModel, chunkID)
			if convErr != nil {
				log.Error("[OpenAIUseCase] Failed to convert anthropic SSE to openai chunk", zap.Error(convErr))
				return convErr
			}
			for _, chunk := range chunks {
				if chunk == nil {
					continue
				}
				allChunks = append(allChunks, chunk)
				chunkData, marshalErr := sonic.Marshal(chunk)
				if marshalErr != nil {
					log.Error("[OpenAIUseCase] Failed to marshal converted chunk", zap.Error(marshalErr))
					return marshalErr
				}
				if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, chunkData); writeErr != nil {
					log.Debug("[OpenAIUseCase] Failed to write converted SSE chunk", zap.Error(writeErr))
				}
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
			_ = w.Flush()
		} else {
			proxyutil.WriteUpstreamSSEError(ctx, w, err)
		}
		completion, _ := proxyutil.ConcatChatCompletionChunks(allChunks)
		if completion != nil {
			completion.Model = exposedModel
		}
		u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model)
		task := newAuditTask(ctx, m, exposedModel, enum.ProviderAnthropic, enum.ProviderOpenAI, firstTokenLatencyMs)
		task.StreamDurationMs = streamDurationMs
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(err)
		_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
	})
}
