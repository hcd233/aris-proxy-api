package service

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/converter"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// endpointFields 查询端点时统一使用的字段列表
var endpointFields = []string{"id", "model", "api_key", "base_url", "provider"}

// openAIInternalErrorBody OpenAI 内部错误响应 body（预序列化，避免重复 marshal）
var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: "Internal server error", Type: "server_error", Code: "internal_error"},
}))

// findEndpoint 按优先 provider 查找端点，未找到则回退到另一个 provider
func findEndpoint(ctx context.Context, modelEndpointDAO *dao.ModelEndpointDAO, alias string, primary, fallback enum.ProviderType) (*dbmodel.ModelEndpoint, error) {
	db := database.GetDBInstance(ctx)
	ep, err := modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{Alias: alias, Provider: primary}, endpointFields)
	if err == nil {
		return ep, nil
	}
	return modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{Alias: alias, Provider: fallback}, endpointFields)
}

// toUpstream 将数据库模型转换为 proxy 层端点
func toUpstream(ep *dbmodel.ModelEndpoint) proxy.UpstreamEndpoint {
	return proxy.UpstreamEndpoint{Model: ep.Model, APIKey: ep.APIKey, BaseURL: ep.BaseURL}
}

// OpenAIService OpenAI服务
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAIService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

type openAIService struct {
	modelEndpointDAO *dao.ModelEndpointDAO
	openAIProxy      proxy.OpenAIProxy
	anthropicProxy   proxy.AnthropicProxy
}

// NewOpenAIService 创建OpenAI服务
//
//	@return OpenAIService
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func NewOpenAIService() OpenAIService {
	return &openAIService{
		modelEndpointDAO: dao.GetModelEndpointDAO(),
		openAIProxy:      proxy.NewOpenAIProxy(),
		anthropicProxy:   proxy.NewAnthropicProxy(),
	}
}

// ListModels 获取模型列表
//
//	@receiver s *openAIService
//	@param ctx context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.OpenAIListModelsRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func (s *openAIService) ListModels(ctx context.Context, _ *dto.EmptyReq) (*dto.OpenAIListModelsRsp, error) {
	db := database.GetDBInstance(ctx)

	endpoints, err := s.modelEndpointDAO.BatchGet(db, &dbmodel.ModelEndpoint{Provider: enum.ProviderOpenAI}, []string{"alias"})
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIService] Failed to query model endpoints", zap.Error(err))
		return &dto.OpenAIListModelsRsp{Object: "list", Data: []*dto.OpenAIModel{}}, nil
	}

	return &dto.OpenAIListModelsRsp{
		Object: "list",
		Data: lo.Map(endpoints, func(ep *dbmodel.ModelEndpoint, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      ep.Alias,
				Created: time.Now().Unix(),
				Object:  "model",
				OwnedBy: "openai",
			}
		}),
	}, nil
}

// CreateChatCompletion 创建聊天补全
//
//	@receiver s *openAIService
//	@param ctx context.Context
//	@param req *dto.OpenAIChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (s *openAIService) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	endpoint, err := findEndpoint(ctx, s.modelEndpointDAO, req.Body.Model, enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIService] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream

	if endpoint.Provider == enum.ProviderAnthropic {
		return s.forwardViaAnthropic(ctx, log, req, endpoint, stream)
	}
	return s.forwardNative(ctx, log, req, endpoint, stream)
}

// forwardNative 原生 OpenAI 协议转发
func (s *openAIService) forwardNative(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, stream bool) (*huma.StreamResponse, error) {
	ep := toUpstream(endpoint)
	if req.Body.MaxTokens != nil {
		req.Body.MaxCompletionTokens, req.Body.MaxTokens = lo.ToPtr(*req.Body.MaxTokens), nil
	}

	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			startTime := time.Now()
			var firstTokenTime time.Time
			var streamDone time.Time
			var firstTokenLatencyMs int64
			var streamDurationMs int64

			completion, err := s.openAIProxy.ForwardChatCompletionStream(ctx, ep, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
				if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
					firstTokenTime = time.Now()
					firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				chunk.Model = req.Body.Model
				chunkData, marshalErr := sonic.Marshal(chunk)
				if marshalErr != nil {
					log.Error("[OpenAIService] Failed to marshal chunk", zap.Error(marshalErr))
					return marshalErr
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
				return w.Flush()
			})
			streamDone = time.Now()
			if !firstTokenTime.IsZero() {
				streamDurationMs = streamDone.Sub(firstTokenTime).Milliseconds()
			}
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
				_ = w.Flush()
			}

			s.storeFromCompletion(ctx, log, req, completion, err, ep.Model)
			var usage *dto.OpenAICompletionUsage
			if completion != nil {
				usage = completion.Usage
			}
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             endpoint.ID,
				Model:               req.Body.Model,
				UpstreamProvider:    endpoint.Provider,
				APIProvider:         enum.ProviderOpenAI,
				FirstTokenLatencyMs: firstTokenLatencyMs,
				StreamDurationMs:    streamDurationMs,
			}
			task.SetTokensFromOpenAIUsage(usage)
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		completion, err := s.openAIProxy.ForwardChatCompletion(ctx, ep, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             endpoint.ID,
				Model:               req.Body.Model,
				UpstreamProvider:    endpoint.Provider,
				APIProvider:         enum.ProviderOpenAI,
				FirstTokenLatencyMs: totalMs,
			}
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		s.storeFromCompletion(ctx, log, req, completion, nil, ep.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromOpenAIUsage(completion.Usage)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	}), nil
}

// forwardViaAnthropic 通过 Anthropic 协议上游转发 OpenAI 请求
func (s *openAIService) forwardViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, stream bool) (*huma.StreamResponse, error) {
	ep := toUpstream(endpoint)
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromOpenAIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIService] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}
	anthropicReq.Model = ep.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			startTime := time.Now()
			var firstTokenTime time.Time
			var streamDone time.Time
			var firstTokenLatencyMs int64
			var streamDurationMs int64

			chunkID := converter.GenerateOpenAIChunkID()
			anthropicMsg, err := s.anthropicProxy.ForwardCreateMessageStream(ctx, ep, body, func(event dto.AnthropicSSEEvent) error {
				chunks, err := conv.ToOpenAISSEResponse(event, req.Body.Model, chunkID)
				if err != nil {
					return err
				}
				for _, chunk := range chunks {
					if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
						firstTokenTime = time.Now()
						firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
					}
					chunkData, marshalErr := sonic.Marshal(chunk)
					if marshalErr != nil {
						log.Error("[OpenAIService] Failed to marshal chunk", zap.Error(marshalErr))
						return marshalErr
					}
					_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
					if err := w.Flush(); err != nil {
						return err
					}
				}
				return nil
			})
			streamDone = time.Now()
			if !firstTokenTime.IsZero() {
				streamDurationMs = streamDone.Sub(firstTokenTime).Milliseconds()
			}
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
				_ = w.Flush()
			}

			s.storeFromAnthropicMsg(ctx, log, req, anthropicMsg, err, ep.Model)
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             endpoint.ID,
				Model:               req.Body.Model,
				UpstreamProvider:    endpoint.Provider,
				APIProvider:         enum.ProviderOpenAI,
				FirstTokenLatencyMs: firstTokenLatencyMs,
				StreamDurationMs:    streamDurationMs,
			}
			task.SetTokensFromAnthropicUsage(anthropicMsg)
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		anthropicMsg, err := s.anthropicProxy.ForwardCreateMessage(ctx, ep, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             endpoint.ID,
				Model:               req.Body.Model,
				UpstreamProvider:    endpoint.Provider,
				APIProvider:         enum.ProviderOpenAI,
				FirstTokenLatencyMs: totalMs,
			}
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			log.Error("[OpenAIService] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(fiber.StatusInternalServerError, openAIInternalErrorBody)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		s.storeFromCompletion(ctx, log, req, completion, nil, ep.Model)
		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		task.SetTokensFromAnthropicUsage(anthropicMsg)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	}), nil
}

// CreateResponse 创建 Response API 响应
//
//	@receiver s *openAIService
//	@param ctx context.Context
//	@param req *dto.OpenAICreateResponseRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-17 10:00:00
func (s *openAIService) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	endpoint, err := findEndpoint(ctx, s.modelEndpointDAO, req.Body.Model, enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIService] Response API model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	// Response API 仅支持 OpenAI 上游
	if endpoint.Provider != enum.ProviderOpenAI {
		log.Error("[OpenAIService] Response API does not support non-OpenAI provider", zap.String("model", req.Body.Model), zap.String("provider", endpoint.Provider))
		return util.SendOpenAIInternalError(), nil
	}

	ep := toUpstream(endpoint)
	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)
	stream := req.Body.Stream != nil && *req.Body.Stream

	if stream {
		return s.forwardResponseStream(ctx, log, req, endpoint, ep, body)
	}
	return s.forwardResponseNonStream(ctx, log, req, endpoint, ep, body)
}

// forwardResponseStream Response API 流式转发
func (s *openAIService) forwardResponseStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) (*huma.StreamResponse, error) {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs int64
		var streamDurationMs int64

		proxyErr := s.openAIProxy.ForwardCreateResponseStream(ctx, ep, body, func(event string, data []byte) error {
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			replaced := proxy.ReplaceModelInSSEData(data, req.Body.Model)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, replaced)
			return w.Flush()
		})

		streamDone := time.Now()
		if !firstTokenTime.IsZero() {
			streamDurationMs = streamDone.Sub(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIService] Response API stream error", zap.Error(proxyErr))
		}

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(proxyErr)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	}), nil
}

// forwardResponseNonStream Response API 非流式转发
func (s *openAIService) forwardResponseNonStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) (*huma.StreamResponse, error) {
	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		startTime := time.Now()
		respBody, err := s.openAIProxy.ForwardCreateResponse(ctx, ep, body)
		totalMs := time.Since(startTime).Milliseconds()
		if err != nil {
			util.WriteUpstreamError(log, writer, err, openAIInternalErrorBody)
			task := &dto.ModelCallAuditTask{
				Ctx:                 util.CopyContextValues(ctx),
				ModelID:             endpoint.ID,
				Model:               req.Body.Model,
				UpstreamProvider:    endpoint.Provider,
				APIProvider:         enum.ProviderOpenAI,
				FirstTokenLatencyMs: totalMs,
			}
			task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
			_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
			return
		}

		replaced := proxy.ReplaceModelInBody(respBody, req.Body.Model)
		writer.HumaCtx.SetStatus(fiber.StatusOK)
		writer.HumaCtx.SetHeader("Content-Type", "application/json")
		_, _ = writer.HumaCtx.BodyWriter().Write(replaced)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	}), nil
}

// ==================== Store Messages ====================

func (s *openAIService) storeFromCompletion(ctx context.Context, logger *zap.Logger, req *dto.OpenAIChatCompletionRequest, completion *dto.OpenAIChatCompletion, proxyErr error, upstreamModel string) {
	if proxyErr != nil || completion == nil || len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	s.storeOpenAIMessages(ctx, logger, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

func (s *openAIService) storeFromAnthropicMsg(ctx context.Context, logger *zap.Logger, req *dto.OpenAIChatCompletionRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	conv := converter.AnthropicProtocolConverter{}
	completion, err := conv.ToOpenAIResponse(msg)
	if err != nil {
		logger.Error("[OpenAIService] Failed to convert for storage", zap.Error(err))
		return
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	s.storeOpenAIMessages(ctx, logger, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

func (s *openAIService) storeOpenAIMessages(ctx context.Context, logger *zap.Logger, req *dto.OpenAIChatCompletionRequest, assistantMsg *dto.OpenAIChatCompletionMessageParam, upstreamModel string, usage *dto.OpenAICompletionUsage) {
	var unifiedMessages []*dto.UnifiedMessage
	for _, msg := range req.Body.Messages {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			logger.Error("[OpenAIService] Failed to convert openai message", zap.Error(err))
			return
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromOpenAIMessage(assistantMsg)
	if err != nil {
		logger.Error("[OpenAIService] Failed to convert ai response message", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := lo.Map(req.Body.Tools, func(tool dto.OpenAIChatCompletionTool, _ int) *dto.UnifiedTool {
		return dto.FromOpenAITool(&tool)
	})

	var inputTokens, outputTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
	}

	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		logger.Error("[OpenAIService] Failed to submit message store task", zap.Error(err))
	}
}
