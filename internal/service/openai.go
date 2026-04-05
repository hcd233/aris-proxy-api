package service

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
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
var endpointFields = []string{"model", "api_key", "base_url", "provider"}

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
//	@update 2026-04-05 10:00:00
type OpenAIService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
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
	logger := logger.WithCtx(ctx)

	endpoint, err := findEndpoint(ctx, s.modelEndpointDAO, req.Body.Model, enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		logger.Error("[OpenAIService] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	ep := toUpstream(endpoint)
	stream := req.Body.Stream != nil && *req.Body.Stream

	if endpoint.Provider == enum.ProviderAnthropic {
		return s.forwardViaAnthropic(ctx, logger, req, ep, stream)
	}
	return s.forwardNative(ctx, logger, req, ep, stream)
}

// forwardNative 原生 OpenAI 协议转发
func (s *openAIService) forwardNative(ctx context.Context, logger *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep proxy.UpstreamEndpoint, stream bool) (*huma.StreamResponse, error) {
	if req.Body.MaxTokens != nil {
		req.Body.MaxCompletionTokens, req.Body.MaxTokens = lo.ToPtr(*req.Body.MaxTokens), nil
	}

	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			completion, err := s.openAIProxy.ForwardChatCompletionStream(ctx, ep, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
				chunk.Model = req.Body.Model
				fmt.Fprintf(w, "data: %s\n\n", lo.Must1(sonic.Marshal(chunk)))
				return w.Flush()
			})
			fmt.Fprintf(w, "data: [DONE]\n\n")
			w.Flush()

			s.storeFromCompletion(ctx, logger, req, completion, err, ep.Model)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		completion, err := s.openAIProxy.ForwardChatCompletion(ctx, ep, body)
		if err != nil {
			util.WriteUpstreamError(logger, writer, err, openAIInternalErrorBody)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		s.storeFromCompletion(ctx, logger, req, completion, nil, ep.Model)
	}), nil
}

// forwardViaAnthropic 通过 Anthropic 协议上游转发 OpenAI 请求
func (s *openAIService) forwardViaAnthropic(ctx context.Context, logger *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep proxy.UpstreamEndpoint, stream bool) (*huma.StreamResponse, error) {
	conv := converter.AnthropicProtocolConverter{}
	anthropicReq, err := conv.FromOpenAIRequest(req.Body)
	if err != nil {
		logger.Error("[OpenAIService] Failed to convert request to Anthropic format", zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}
	anthropicReq.Model = ep.Model
	body := lo.Must1(sonic.Marshal(anthropicReq))

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			chunkID := converter.GenerateOpenAIChunkID()
			anthropicMsg, err := s.anthropicProxy.ForwardCreateMessageStream(ctx, ep, body, func(event dto.AnthropicSSEEvent) error {
				chunks, err := conv.ToOpenAISSEResponse(event, req.Body.Model, chunkID)
				if err != nil {
					return err
				}
				for _, chunk := range chunks {
					fmt.Fprintf(w, "data: %s\n\n", lo.Must1(sonic.Marshal(chunk)))
					if err := w.Flush(); err != nil {
						return err
					}
				}
				return nil
			})
			fmt.Fprintf(w, "data: [DONE]\n\n")
			w.Flush()

			s.storeFromAnthropicMsg(ctx, logger, req, anthropicMsg, err, ep.Model)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		anthropicMsg, err := s.anthropicProxy.ForwardCreateMessage(ctx, ep, body)
		if err != nil {
			util.WriteUpstreamError(logger, writer, err, openAIInternalErrorBody)
			return
		}
		completion, err := conv.ToOpenAIResponse(anthropicMsg)
		if err != nil {
			logger.Error("[OpenAIService] Failed to convert Anthropic response", zap.Error(err))
			writer.WriteError(500, openAIInternalErrorBody)
			return
		}
		completion.Model = req.Body.Model
		writer.WriteJSON(completion)

		s.storeFromCompletion(ctx, logger, req, completion, nil, ep.Model)
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
		Client:       util.CtxValueString(ctx, constant.CtxKeyClient),
		Metadata:     req.Body.Metadata,
	}); err != nil {
		logger.Error("[OpenAIService] Failed to submit message store task", zap.Error(err))
	}
}
