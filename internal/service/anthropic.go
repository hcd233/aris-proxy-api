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

// anthropicInternalErrorBody Anthropic 内部错误响应 body（预序列化）
var anthropicInternalErrorBody = lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
	Type:  "error",
	Error: &dto.AnthropicError{Type: "api_error", Message: "Internal server error"},
}))

// AnthropicService Anthropic服务
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type AnthropicService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type anthropicService struct {
	modelEndpointDAO *dao.ModelEndpointDAO
	anthropicProxy   proxy.AnthropicProxy
	openAIProxy      proxy.OpenAIProxy
}

// NewAnthropicService 创建Anthropic服务
//
//	@return AnthropicService
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func NewAnthropicService() AnthropicService {
	return &anthropicService{
		modelEndpointDAO: dao.GetModelEndpointDAO(),
		anthropicProxy:   proxy.NewAnthropicProxy(),
		openAIProxy:      proxy.NewOpenAIProxy(),
	}
}

// ListModels 获取Anthropic模型列表
//
//	@receiver s *anthropicService
//	@param ctx context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.AnthropicListModelsRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func (s *anthropicService) ListModels(ctx context.Context, _ *dto.EmptyReq) (*dto.AnthropicListModelsRsp, error) {
	db := database.GetDBInstance(ctx)

	endpoints, err := s.modelEndpointDAO.BatchGet(db, &dbmodel.ModelEndpoint{Provider: enum.ProviderAnthropic}, []string{"alias"})
	if err != nil {
		logger.WithCtx(ctx).Error("[AnthropicService] Failed to query model endpoints", zap.Error(err))
		return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{}}, nil
	}

	models := lo.Map(endpoints, func(ep *dbmodel.ModelEndpoint, _ int) *dto.AnthropicModelInfo {
		return &dto.AnthropicModelInfo{
			ID:          ep.Alias,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			DisplayName: ep.Alias,
			Type:        "model",
		}
	})

	rsp := &dto.AnthropicListModelsRsp{Data: models, HasMore: false}
	if len(models) > 0 {
		rsp.FirstID = models[0].ID
		rsp.LastID = models[len(models)-1].ID
	}
	return rsp, nil
}

// CreateMessage 创建Anthropic消息
//
//	@receiver s *anthropicService
//	@param ctx context.Context
//	@param req *dto.AnthropicCreateMessageRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (s *anthropicService) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	logger := logger.WithCtx(ctx)

	endpoint, err := findEndpoint(ctx, s.modelEndpointDAO, req.Body.Model, enum.ProviderAnthropic, enum.ProviderOpenAI)
	if err != nil {
		logger.Error("[AnthropicService] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	ep := toUpstream(endpoint)
	stream := req.Body.Stream != nil && *req.Body.Stream
	exposedModel := req.Body.Model

	if endpoint.Provider == enum.ProviderOpenAI {
		return s.forwardViaOpenAI(ctx, logger, req, ep, exposedModel, stream)
	}
	return s.forwardNative(ctx, logger, req, ep, exposedModel, stream)
}

// forwardNative 原生 Anthropic 协议转发
func (s *anthropicService) forwardNative(ctx context.Context, logger *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep proxy.UpstreamEndpoint, exposedModel string, stream bool) (*huma.StreamResponse, error) {
	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			anthropicMsg, err := s.anthropicProxy.ForwardCreateMessageStream(ctx, ep, body, func(event dto.AnthropicSSEEvent) error {
				modifiedData := proxy.ReplaceModelInSSEData(event.Data, exposedModel)
				fmt.Fprintf(w, "event: %s\n", event.Event)
				fmt.Fprintf(w, "data: %s\n\n", modifiedData)
				return w.Flush()
			})
			if err == nil {
				fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
				w.Flush()
			}

			s.storeFromAnthropicMsg(ctx, logger, req, anthropicMsg, err, ep.Model)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		anthropicMsg, err := s.anthropicProxy.ForwardCreateMessage(ctx, ep, body)
		if err != nil {
			util.WriteUpstreamError(logger, writer, err, anthropicInternalErrorBody)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		s.storeFromAnthropicMsg(ctx, logger, req, anthropicMsg, nil, ep.Model)
	}), nil
}

// forwardViaOpenAI 通过 OpenAI 协议上游转发 Anthropic 请求
func (s *anthropicService) forwardViaOpenAI(ctx context.Context, logger *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep proxy.UpstreamEndpoint, exposedModel string, stream bool) (*huma.StreamResponse, error) {
	conv := converter.OpenAIProtocolConverter{}
	openAIReq, err := conv.FromAnthropicRequest(req.Body)
	if err != nil {
		logger.Error("[AnthropicService] Failed to convert request to OpenAI format", zap.Error(err))
		return util.SendAnthropicInternalError(), nil
	}
	openAIReq.Model = ep.Model
	body := lo.Must1(sonic.Marshal(openAIReq))

	if stream {
		return util.WrapStreamResponse(func(w *bufio.Writer) {
			isFirst := true
			tracker := converter.NewSSEContentBlockTracker()
			completion, proxyErr := s.openAIProxy.ForwardChatCompletionStream(ctx, ep, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
				events, err := conv.ToAnthropicSSEResponse(chunk, isFirst, exposedModel, tracker)
				if err != nil {
					return err
				}
				isFirst = false
				for _, event := range events {
					fmt.Fprintf(w, "event: %s\n", event.Event)
					fmt.Fprintf(w, "data: %s\n\n", string(event.Data))
					if err := w.Flush(); err != nil {
						return err
					}
				}
				return nil
			})
			if proxyErr == nil {
				fmt.Fprintf(w, "event: message_stop\ndata: {}\n\n")
				w.Flush()
			}

			if proxyErr != nil || completion == nil {
				return
			}
			anthropicMsg, err := conv.ToAnthropicResponse(completion)
			if err != nil {
				logger.Error("[AnthropicService] Failed to convert for storage", zap.Error(err))
				return
			}
			s.storeFromAnthropicMsg(ctx, logger, req, anthropicMsg, nil, ep.Model)
		}), nil
	}

	return util.WrapJSONResponse(func(writer util.JSONResponseWriter) {
		completion, err := s.openAIProxy.ForwardChatCompletion(ctx, ep, body)
		if err != nil {
			util.WriteUpstreamError(logger, writer, err, anthropicInternalErrorBody)
			return
		}
		anthropicMsg, err := conv.ToAnthropicResponse(completion)
		if err != nil {
			logger.Error("[AnthropicService] Failed to convert OpenAI response", zap.Error(err))
			writer.WriteError(500, anthropicInternalErrorBody)
			return
		}
		anthropicMsg.Model = exposedModel
		writer.WriteJSON(anthropicMsg)

		s.storeFromAnthropicMsg(ctx, logger, req, anthropicMsg, nil, ep.Model)
	}), nil
}

// ==================== Store Messages ====================

func (s *anthropicService) storeFromAnthropicMsg(ctx context.Context, logger *zap.Logger, req *dto.AnthropicCreateMessageRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	s.storeAnthropicMessages(ctx, logger, req, msg, upstreamModel)
}

func (s *anthropicService) storeAnthropicMessages(ctx context.Context, logger *zap.Logger, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage, upstreamModel string) {
	var unifiedMessages []*dto.UnifiedMessage
	for _, msg := range req.Body.Messages {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			logger.Error("[AnthropicService] Failed to convert anthropic message", zap.Error(err))
			return
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromAnthropicResponse(assistantMsg)
	if err != nil {
		logger.Error("[AnthropicService] Failed to convert anthropic response", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := make([]*dto.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		unifiedTools = append(unifiedTools, dto.FromAnthropicTool(tool))
	}

	var inputTokens, outputTokens int
	if assistantMsg.Usage != nil {
		inputTokens = assistantMsg.Usage.InputTokens
		outputTokens = assistantMsg.Usage.OutputTokens
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
		Metadata:     util.ExtractAnthropicMetadata(req.Body.Metadata),
	}); err != nil {
		logger.Error("[AnthropicService] Failed to submit message store task", zap.Error(err))
	}
}

// CountTokens 计算Token数量
//
//	@receiver s *anthropicService
//	@param ctx context.Context
//	@param req *dto.AnthropicCountTokensRequest
//	@return *dto.AnthropicTokensCount
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (s *anthropicService) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	logger := logger.WithCtx(ctx)

	db := database.GetDBInstance(ctx)
	endpoint, err := s.modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{
		Alias:    req.Body.Model,
		Provider: enum.ProviderAnthropic,
	}, []string{"model", "api_key", "base_url"})
	if err != nil {
		logger.Warn("[AnthropicService] Model not found, returning 0", zap.String("model", req.Body.Model), zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}

	ep := toUpstream(endpoint)
	body := proxy.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), ep.Model)

	rsp, err := s.anthropicProxy.ForwardCountTokens(ctx, ep, body)
	if err != nil {
		logger.Warn("[AnthropicService] Count tokens error, returning 0", zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}

	return rsp, nil
}
