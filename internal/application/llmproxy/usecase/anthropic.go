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
	convvo "github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

var anthropicInternalErrorBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  constant.AnthropicInternalErrorBodyType,
	Error: &dto.AnthropicError{Type: constant.AnthropicInternalErrorType, Message: constant.AnthropicInternalErrorMessage},
}))

type AnthropicUseCase interface {
	ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	openAIProxy      transport.OpenAIProxy
	anthropicProxy   transport.AnthropicProxy
	taskSubmitter    TaskSubmitter
}

func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	openAIProxy transport.OpenAIProxy,
	anthropicProxy transport.AnthropicProxy,
	taskSubmitter TaskSubmitter,
) AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		openAIProxy:      openAIProxy,
		anthropicProxy:   anthropicProxy,
		taskSubmitter:    taskSubmitter,
	}
}

func (u *anthropicUseCase) ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *anthropicUseCase) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	return u.countTokensQuery.Handle(ctx, req)
}

func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model))
	if err != nil {
		log.Error("[AnthropicUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}
	if !ep.SupportAnthropicMessage() {
		log.Error("[AnthropicUseCase] Endpoint does not support messages API", zap.String("model", req.Body.Model))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	exposedModel := req.Body.Model
	upstream := toTransportEndpoint(m, ep, true)
	return u.forwardMessageNative(ctx, req, m, ep, upstream, exposedModel, stream), nil
}

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

func (u *anthropicUseCase) storeAnthropicFromMsg(ctx context.Context, req *dto.AnthropicCreateMessageRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	u.storeAnthropicMessages(ctx, req, msg, upstreamModel)
}

func (u *anthropicUseCase) storeAnthropicMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage, upstreamModel string) {
	log := logger.WithCtx(ctx)
	unifiedMessages, unifiedTools, inputTokens, outputTokens, err := u.convertAnthropicRequestMessages(ctx, req, assistantMsg)
	if err != nil {
		return
	}

	if err := u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     util.ExtractAnthropicMetadata(req.Body.Metadata),
	}); err != nil {
		log.Error("[AnthropicUseCase] Failed to submit message store task", zap.Error(err))
	}
}

func (u *anthropicUseCase) convertAnthropicRequestMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage) ([]*convvo.UnifiedMessage, []*convvo.UnifiedTool, int, int, error) {
	log := logger.WithCtx(ctx)
	unifiedMessages := make([]*convvo.UnifiedMessage, 0, len(req.Body.Messages)+1)
	for _, msg := range req.Body.Messages {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			log.Error("[AnthropicUseCase] Failed to convert anthropic message", zap.Error(err))
			return nil, nil, 0, 0, err
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromAnthropicResponse(assistantMsg)
	if err != nil {
		log.Error("[AnthropicUseCase] Failed to convert anthropic response", zap.Error(err))
		return nil, nil, 0, 0, err
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := make([]*convvo.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		unifiedTools = append(unifiedTools, dto.FromAnthropicTool(tool))
	}

	var inputTokens, outputTokens int
	if assistantMsg.Usage != nil {
		inputTokens = assistantMsg.Usage.InputTokens
		outputTokens = assistantMsg.Usage.OutputTokens
	}

	return unifiedMessages, unifiedTools, inputTokens, outputTokens, nil
}
