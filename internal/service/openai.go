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
		return s.forwardNativeStream(ctx, log, req, endpoint, ep, body), nil
	}
	return s.forwardNativeNonStream(ctx, log, req, endpoint, ep, body), nil
}

// forwardNativeStream OpenAI 原生协议流式转发
func (s *openAIService) forwardNativeStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
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
		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
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
	})
}

// forwardNativeNonStream OpenAI 原生协议非流式转发
func (s *openAIService) forwardNativeNonStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) *huma.StreamResponse {
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
	})
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
		return s.forwardViaAnthropicStream(ctx, log, req, endpoint, ep, body, &conv), nil
	}
	return s.forwardViaAnthropicNonStream(ctx, log, req, endpoint, ep, body, &conv), nil
}

// forwardViaAnthropicStream 通过 Anthropic 上游转发并以 OpenAI SSE 协议返回
func (s *openAIService) forwardViaAnthropicStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs int64
		var streamDurationMs int64

		chunkID := converter.GenerateOpenAIChunkID()
		anthropicMsg, err := s.anthropicProxy.ForwardCreateMessageStream(ctx, ep, body, func(event dto.AnthropicSSEEvent) error {
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
					log.Error("[OpenAIService] Failed to marshal chunk", zap.Error(marshalErr))
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
	})
}

// forwardViaAnthropicNonStream 通过 Anthropic 上游转发并以 OpenAI JSON 协议返回
func (s *openAIService) forwardViaAnthropicNonStream(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
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
	})
}

// ==================== Store Messages ====================

// storeFromCompletion 从原生 OpenAI 响应抽取并存储会话
func (s *openAIService) storeFromCompletion(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, completion *dto.OpenAIChatCompletion, proxyErr error, upstreamModel string) {
	if proxyErr != nil || completion == nil || len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	s.storeOpenAIMessages(ctx, log, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeFromAnthropicMsg 将 Anthropic 响应先转为 OpenAI 再存储（保持 /chat/completions 路径的存储格式一致）
func (s *openAIService) storeFromAnthropicMsg(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	conv := converter.AnthropicProtocolConverter{}
	completion, err := conv.ToOpenAIResponse(msg)
	if err != nil {
		log.Error("[OpenAIService] Failed to convert for storage", zap.Error(err))
		return
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	s.storeOpenAIMessages(ctx, log, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeOpenAIMessages 把 /chat/completions 形式的会话投递到消息存储协程池
func (s *openAIService) storeOpenAIMessages(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, assistantMsg *dto.OpenAIChatCompletionMessageParam, upstreamModel string, usage *dto.OpenAICompletionUsage) {
	var unifiedMessages []*dto.UnifiedMessage
	for _, msg := range req.Body.Messages {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			log.Error("[OpenAIService] Failed to convert openai message", zap.Error(err))
			return
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromOpenAIMessage(assistantMsg)
	if err != nil {
		log.Error("[OpenAIService] Failed to convert ai response message", zap.Error(err))
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
		log.Error("[OpenAIService] Failed to submit message store task", zap.Error(err))
	}
}

// ==================== Response API ====================

// CreateResponse 创建 Response API 响应
//
//	@receiver s *openAIService
//	@param ctx context.Context
//	@param req *dto.OpenAICreateResponseRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-18 18:00:00
func (s *openAIService) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	endpoint, err := findEndpoint(ctx, s.modelEndpointDAO, req.Body.Model, enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		log.Error("[OpenAIService] Response API model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	ep := toUpstream(endpoint)
	stream := req.Body.Stream != nil && *req.Body.Stream

	if endpoint.Provider == enum.ProviderAnthropic {
		return s.forwardResponseViaAnthropic(ctx, log, req, endpoint, ep, stream)
	}
	return s.forwardResponseNative(ctx, log, req, endpoint, ep, stream)
}

// forwardResponseNative 原生 OpenAI 协议转发 Response API
func (s *openAIService) forwardResponseNative(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, stream bool) (*huma.StreamResponse, error) {
	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)

	if stream {
		return s.forwardResponseStream(ctx, log, req, endpoint, ep, body), nil
	}
	return s.forwardResponseNonStream(ctx, log, req, endpoint, ep, body), nil
}

// forwardResponseViaAnthropic 通过 Anthropic 协议上游转发 Response API 请求
func (s *openAIService) forwardResponseViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, stream bool) (*huma.StreamResponse, error) {
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromResponseAPIRequest(req.Body)
	if err != nil {
		log.Error("[OpenAIService] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}
	anthropicReq.Model = ep.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return s.forwardResponseAnthropicStream(ctx, log, req, endpoint, ep, body, &conv), nil
	}
	return s.forwardResponseAnthropicNonStream(ctx, log, req, endpoint, ep, body, &conv), nil
}

// forwardResponseStream 原生 OpenAI 协议流式转发
func (s *openAIService) forwardResponseStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs int64
		var streamDurationMs int64
		var finalResponse *dto.OpenAICreateResponseRsp

		proxyErr := s.openAIProxy.ForwardCreateResponseStream(ctx, ep, body, func(event string, data []byte) error {
			// 只在真正携带生成 token 的事件上计算 time-to-first-token（*.delta），
			// 避免 response.created / response.in_progress / *.added / *.done
			// 等事件污染基线，与 /chat/completions 的度量口径保持一致。
			if firstTokenTime.IsZero() && util.IsResponseAPIDeltaEvent(event) {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			// 拦截终态事件（completed / failed / incomplete）——每一个都携带
			// Response 对象（含 usage），审计路径需要它；存储路径会按 status 决定
			// 是否持久化会话。
			if finalResponse == nil && util.IsResponseAPITerminalEvent(event) {
				var ev dto.ResponseStreamTerminalEvent
				if err := sonic.Unmarshal(data, &ev); err != nil {
					log.Warn("[OpenAIService] Failed to parse response terminal event",
						zap.String("event", event), zap.Error(err))
				} else {
					finalResponse = ev.Response
				}
			}
			replaced := proxy.ReplaceModelInSSEData(data, req.Body.Model)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, replaced)
			return w.Flush()
		})

		if !firstTokenTime.IsZero() {
			streamDurationMs = time.Since(firstTokenTime).Milliseconds()
		}
		if proxyErr != nil {
			log.Error("[OpenAIService] Response API stream error", zap.Error(proxyErr))
		}

		s.storeFromResponseRsp(ctx, log, req, finalResponse, proxyErr, ep.Model)

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: firstTokenLatencyMs,
			StreamDurationMs:    streamDurationMs,
		}
		task.SetTokensFromResponseUsage(finalResponse)
		task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(proxyErr)
		// 上游 HTTP 200 但 Response 对象 status=failed/incomplete 时，
		// util.ExtractUpstreamStatusAndError 只能看到 HTTP 层（返回 200）；
		// 这里从 Response 对象补充 in-band 失败/未完成的原因。
		task.SetErrorFromResponseStatus(finalResponse)
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardResponseNonStream 原生 OpenAI 协议非流式转发
func (s *openAIService) forwardResponseNonStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte) *huma.StreamResponse {
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

		var rsp dto.OpenAICreateResponseRsp
		parseErr := sonic.Unmarshal(respBody, &rsp)
		if parseErr != nil {
			log.Warn("[OpenAIService] Failed to parse Response API non-stream body", zap.Error(parseErr))
		} else {
			s.storeFromResponseRsp(ctx, log, req, &rsp, nil, ep.Model)
		}

		task := &dto.ModelCallAuditTask{
			Ctx:                 util.CopyContextValues(ctx),
			ModelID:             endpoint.ID,
			Model:               req.Body.Model,
			UpstreamProvider:    endpoint.Provider,
			APIProvider:         enum.ProviderOpenAI,
			FirstTokenLatencyMs: totalMs,
			UpstreamStatusCode:  fiber.StatusOK,
		}
		if parseErr == nil {
			task.SetTokensFromResponseUsage(&rsp)
			task.SetErrorFromResponseStatus(&rsp)
		}
		_ = pool.GetPoolManager().SubmitModelCallAuditTask(task)
	})
}

// forwardResponseAnthropicStream Anthropic 上游流式转发，以 OpenAI chat chunk 形式回写
func (s *openAIService) forwardResponseAnthropicStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var firstTokenLatencyMs int64
		var streamDurationMs int64

		chunkID := converter.GenerateOpenAIChunkID()
		anthropicMsg, err := s.anthropicProxy.ForwardCreateMessageStream(ctx, ep, body, func(event dto.AnthropicSSEEvent) error {
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
					log.Error("[OpenAIService] Failed to marshal chunk", zap.Error(marshalErr))
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
		}

		s.storeFromAnthropicMsgForResponse(ctx, log, req, anthropicMsg, err, ep.Model)

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
	})
}

// forwardResponseAnthropicNonStream Anthropic 上游非流式转发，以 OpenAI chat JSON 形式回写
func (s *openAIService) forwardResponseAnthropicNonStream(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, endpoint *dbmodel.ModelEndpoint, ep proxy.UpstreamEndpoint, body []byte, conv *converter.AnthropicProtocolConverter) *huma.StreamResponse {
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

		s.storeFromAnthropicMsgForResponse(ctx, log, req, anthropicMsg, nil, ep.Model)

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
	})
}

// ==================== Response API Store Helpers ====================

// buildResponseRequestUnifiedMessages 将 Response API 请求中的 instructions + input
// 转换为 UnifiedMessage 列表，作为存储时的前置会话上下文。
//
// 返回 (messages, ok)：
//
//   - ok=false 表示 input.Items 转换失败，调用方应放弃落盘（错误日志已由本函数打印）。
//
//   - ok=true 时 messages 可能为空（表示既无 instructions 也无 input）。
//
//     @param log *zap.Logger
//     @param req *dto.OpenAICreateResponseRequest
//     @return []*dto.UnifiedMessage
//     @return bool
//     @author centonhuang
//     @update 2026-04-20 11:00:00
func buildResponseRequestUnifiedMessages(log *zap.Logger, req *dto.OpenAICreateResponseRequest) ([]*dto.UnifiedMessage, bool) {
	var messages []*dto.UnifiedMessage

	if req.Body.Instructions != nil && *req.Body.Instructions != "" {
		messages = append(messages, &dto.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &dto.UnifiedContent{Text: *req.Body.Instructions},
		})
	}

	if req.Body.Input != nil {
		if len(req.Body.Input.Items) > 0 {
			inputMsgs, err := dto.FromResponseAPIInputItems(req.Body.Input.Items)
			if err != nil {
				log.Error("[OpenAIService] Failed to convert response input items", zap.Error(err))
				return nil, false
			}
			messages = append(messages, inputMsgs...)
		} else if req.Body.Input.Text != "" {
			messages = append(messages, &dto.UnifiedMessage{
				Role:    enum.RoleUser,
				Content: &dto.UnifiedContent{Text: req.Body.Input.Text},
			})
		}
	}

	return messages, true
}

// buildResponseUnifiedTools 将 Response API 请求的 tools 转换为 UnifiedTool
func buildResponseUnifiedTools(tools []*dto.ResponseTool) []*dto.UnifiedTool {
	result := make([]*dto.UnifiedTool, 0, len(tools))
	for _, tool := range tools {
		if ut := dto.FromResponseAPITool(tool); ut != nil {
			result = append(result, ut)
		}
	}
	return result
}

// anthropicResponseContentToUnified 将 Anthropic Message.Content 中的块转换为 UnifiedMessage。
//
// 失败策略：任何单个 tool_use 块 marshal 失败即视为整条响应不可信，返回 false 让调用方
// 放弃落盘（而不是把残缺消息写入存储）。
//
//	@param log *zap.Logger
//	@param blocks []*dto.AnthropicContentBlock
//	@return []*dto.UnifiedMessage
//	@return bool
//	@author centonhuang
//	@update 2026-04-20 11:00:00
func anthropicResponseContentToUnified(log *zap.Logger, blocks []*dto.AnthropicContentBlock) ([]*dto.UnifiedMessage, bool) {
	var messages []*dto.UnifiedMessage
	for _, block := range blocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			messages = append(messages, &dto.UnifiedMessage{
				Role:    enum.RoleAssistant,
				Content: &dto.UnifiedContent{Text: block.Text},
			})
		case enum.AnthropicContentBlockTypeThinking:
			messages = append(messages, &dto.UnifiedMessage{
				Role:             enum.RoleAssistant,
				ReasoningContent: lo.FromPtr(block.Thinking),
			})
		case enum.AnthropicContentBlockTypeToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				log.Error("[OpenAIService] Failed to marshal tool_use input, abort storage to avoid partial conversation",
					zap.String("toolID", block.ID), zap.String("toolName", block.Name), zap.Error(err))
				return nil, false
			}
			messages = append(messages, &dto.UnifiedMessage{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.UnifiedToolCall{{
					ID:   block.ID,
					Name: block.Name,
				}},
				Content: &dto.UnifiedContent{Text: args},
			})
		}
	}
	return messages, true
}

// storeFromAnthropicMsgForResponse 将 Anthropic Message 响应转换为 UnifiedMessage 并投递存储任务（Response API 路径）
func (s *openAIService) storeFromAnthropicMsgForResponse(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(log, req)
	if !ok {
		return
	}

	outputMsgs, ok := anthropicResponseContentToUnified(log, msg.Content)
	if !ok {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if msg.Usage != nil {
		inputTokens = msg.Usage.InputTokens
		outputTokens = msg.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, log, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// storeFromResponseRsp 将 Response API 请求+响应转换为 UnifiedMessage 并投递存储任务
//
// 当 proxyErr 非 nil 或响应为空时直接返回，不落盘。input 为字符串形态时
// 作为单条 user 消息处理；instructions 若非空则以 system 消息前置插入。
//
// 对响应状态的处理与 /chat/completions 对齐：只要上游产出了可落盘的内容
// （Output 非空），就持久化，不论是 completed 还是 incomplete（例如触发
// max_output_tokens 但已经生成了部分文本）。只有明确的 failed / cancelled
// / queued / in_progress 等没有有效 output 的终态/中间态才跳过。
func (s *openAIService) storeFromResponseRsp(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, rsp *dto.OpenAICreateResponseRsp, proxyErr error, upstreamModel string) {
	if proxyErr != nil || rsp == nil {
		return
	}
	switch rsp.Status {
	case "",
		enum.ResponseStatusCompleted,
		enum.ResponseStatusIncomplete:
		// persistable
	default:
		// failed / cancelled / queued / in_progress: no reliable content
		return
	}
	if len(rsp.Output) == 0 {
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(log, req)
	if !ok {
		return
	}

	outputMsgs, err := dto.FromResponseAPIOutputItems(rsp.Output)
	if err != nil {
		log.Error("[OpenAIService] Failed to convert response output items", zap.Error(err))
		return
	}
	if len(outputMsgs) == 0 {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if rsp.Usage != nil {
		inputTokens = rsp.Usage.InputTokens
		outputTokens = rsp.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, log, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// submitResponseMessageStoreTask 将 Response API 路径下的会话投递到消息存储协程池，
// 统一填充 tools / metadata / API key name，避免在两个 store 函数里重复这段 boilerplate。
func submitResponseMessageStoreTask(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, upstreamModel string, messages []*dto.UnifiedMessage, inputTokens, outputTokens int) {
	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     messages,
		Tools:        buildResponseUnifiedTools(req.Body.Tools),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		log.Error("[OpenAIService] Failed to submit response message store task", zap.Error(err))
	}
}
