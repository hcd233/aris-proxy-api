package usecase

import (
	"context"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ListOpenAIModels 列出 OpenAI 兼容模型
type ListOpenAIModels interface {
	Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
}

type listOpenAIModels struct {
	readRepo llmproxy.EndpointReadRepository
}

func NewListOpenAIModels(readRepo llmproxy.EndpointReadRepository) ListOpenAIModels {
	return &listOpenAIModels{readRepo: readRepo}
}

func (q *listOpenAIModels) Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	projections, err := q.readRepo.ListAliases(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIQuery] Failed to query models", zap.Error(err))
		return &dto.OpenAIListModelsRsp{Object: constant.OpenAIListObject, Data: []*dto.OpenAIModel{}}, nil
	}
	return &dto.OpenAIListModelsRsp{
		Object: constant.OpenAIListObject,
		Data: lo.Map(projections, func(p *llmproxy.ModelAliasProjection, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      p.Alias,
				Created: time.Now().Unix(),
				Object:  constant.OpenAIModelObject,
				OwnedBy: constant.OpenAIModelOwnedBy,
			}
		}),
	}, nil
}

// ListAnthropicModels 列出 Anthropic 兼容模型
type ListAnthropicModels interface {
	Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
}

type listAnthropicModels struct {
	readRepo llmproxy.EndpointReadRepository
}

func NewListAnthropicModels(readRepo llmproxy.EndpointReadRepository) ListAnthropicModels {
	return &listAnthropicModels{readRepo: readRepo}
}

func (q *listAnthropicModels) Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	projections, err := q.readRepo.ListAliases(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[AnthropicQuery] Failed to query models", zap.Error(err))
		return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{}}, nil
	}
	models := lo.Map(projections, func(p *llmproxy.ModelAliasProjection, _ int) *dto.AnthropicModelInfo {
		return &dto.AnthropicModelInfo{
			ID:          p.Alias,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			DisplayName: p.Alias,
			Type:        constant.AnthropicModelType,
		}
	})
	rsp := &dto.AnthropicListModelsRsp{Data: models, HasMore: false}
	if len(models) > 0 {
		rsp.FirstID = models[0].ID
		rsp.LastID = models[len(models)-1].ID
	}
	return rsp, nil
}

// CountTokens Anthropic /v1/messages/count_tokens
type CountTokens interface {
	Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type countTokens struct {
	readRepo llmproxy.EndpointReadRepository
	proxy    transport.AnthropicProxy
}

func NewCountTokens(readRepo llmproxy.EndpointReadRepository, proxy transport.AnthropicProxy) CountTokens {
	return &countTokens{readRepo: readRepo, proxy: proxy}
}

func (q *countTokens) Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)

	epProj, modelNameProj, err := q.readRepo.FindEndpointByAlias(ctx, req.Body.Model, func(ep *llmproxy.EndpointProjection) bool {
		return ep.SupportAnthropicMessage
	})
	if err != nil {
		log.Warn("[AnthropicQuery] Model lookup error, returning 0", zap.String("model", req.Body.Model), zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	if epProj == nil {
		log.Warn("[AnthropicQuery] Model not found, returning 0", zap.String("model", req.Body.Model))
		return &dto.AnthropicTokensCount{}, nil
	}
	if !epProj.SupportAnthropicMessage {
		log.Warn("[AnthropicQuery] Endpoint does not support messages API", zap.String("model", req.Body.Model))
		return &dto.AnthropicTokensCount{}, nil
	}

	upstream := vo.UpstreamEndpoint{
		Model:   modelNameProj.Alias,
		APIKey:  epProj.APIKey,
		BaseURL: epProj.AnthropicBaseURL,
	}
	body := util.MarshalAnthropicCountTokensBodyForModel(req.Body, upstream.Model)

	rsp, err := q.proxy.ForwardCountTokens(ctx, upstream, body)
	if err != nil {
		log.Warn("[AnthropicQuery] Count tokens error, returning 0", zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	return rsp, nil
}
