// Package usecase LLMProxy 域查询（Query）侧
//
// ListModels / CountTokens 是纯读路径，通过 EndpointReadRepository 接口查询，
// 不直接依赖 DAO 或 dbmodel。
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
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ============================================================
// ListOpenAIModels OpenAI 模型列表查询
// ============================================================

// ListOpenAIModels OpenAI 模型列表查询
type ListOpenAIModels interface {
	Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
}

type listOpenAIModels struct {
	readRepo llmproxy.EndpointReadRepository
}

// NewListOpenAIModels 构造 OpenAI ListModels 查询
//
//	@param readRepo llmproxy.EndpointReadRepository
//	@return ListOpenAIModels
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewListOpenAIModels(readRepo llmproxy.EndpointReadRepository) ListOpenAIModels {
	return &listOpenAIModels{readRepo: readRepo}
}

// Handle 查询 provider=openai 的所有 alias
func (q *listOpenAIModels) Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	projections, err := q.readRepo.ListAliasesByProvider(ctx, enum.ProviderOpenAI)
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIQuery] Failed to query model endpoints", zap.Error(err))
		return &dto.OpenAIListModelsRsp{Object: constant.OpenAIListObject, Data: []*dto.OpenAIModel{}}, nil
	}
	return &dto.OpenAIListModelsRsp{
		Object: constant.OpenAIListObject,
		Data: lo.Map(projections, func(p *llmproxy.EndpointAliasProjection, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      p.Alias,
				Created: time.Now().Unix(),
				Object:  constant.OpenAIModelObject,
				OwnedBy: constant.OpenAIModelOwnedBy,
			}
		}),
	}, nil
}

// ============================================================
// ListAnthropicModels Anthropic 模型列表查询
// ============================================================

// ListAnthropicModels Anthropic 模型列表查询
type ListAnthropicModels interface {
	Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
}

type listAnthropicModels struct {
	readRepo llmproxy.EndpointReadRepository
}

// NewListAnthropicModels 构造 Anthropic ListModels 查询
//
//	@param readRepo llmproxy.EndpointReadRepository
//	@return ListAnthropicModels
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewListAnthropicModels(readRepo llmproxy.EndpointReadRepository) ListAnthropicModels {
	return &listAnthropicModels{readRepo: readRepo}
}

// Handle 查询 provider=anthropic 的所有 alias
func (q *listAnthropicModels) Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	projections, err := q.readRepo.ListAliasesByProvider(ctx, enum.ProviderAnthropic)
	if err != nil {
		logger.WithCtx(ctx).Error("[AnthropicQuery] Failed to query model endpoints", zap.Error(err))
		return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{}}, nil
	}

	models := lo.Map(projections, func(p *llmproxy.EndpointAliasProjection, _ int) *dto.AnthropicModelInfo {
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

// ============================================================
// CountTokens Anthropic /v1/messages/count_tokens
// ============================================================

// CountTokens Anthropic Token 计数查询
type CountTokens interface {
	Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type countTokens struct {
	readRepo llmproxy.EndpointReadRepository
	proxy    transport.AnthropicProxy
}

// NewCountTokens 构造 Anthropic CountTokens 查询
//
//	@param readRepo llmproxy.EndpointReadRepository
//	@param proxy transport.AnthropicProxy
//	@return CountTokens
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewCountTokens(readRepo llmproxy.EndpointReadRepository, proxy transport.AnthropicProxy) CountTokens {
	return &countTokens{readRepo: readRepo, proxy: proxy}
}

// Handle 调用上游 count_tokens；任何错误（包括 model not found）返回空结果（与旧行为一致）
func (q *countTokens) Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)

	creds, err := q.readRepo.FindCredentialByAliasAndProvider(ctx, req.Body.Model, enum.ProviderAnthropic)
	if err != nil {
		log.Warn("[AnthropicQuery] Model not found, returning 0", zap.String("model", req.Body.Model), zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	if creds == nil {
		log.Warn("[AnthropicQuery] Model not found, returning 0", zap.String("model", req.Body.Model))
		return &dto.AnthropicTokensCount{}, nil
	}

	upstream := vo.UpstreamEndpoint{
		Model:   creds.Model,
		APIKey:  creds.APIKey,
		BaseURL: creds.BaseURL,
	}
	body := util.MarshalAnthropicCountTokensBodyForModel(req.Body, upstream.Model)

	rsp, err := q.proxy.ForwardCountTokens(ctx, upstream, body)
	if err != nil {
		log.Warn("[AnthropicQuery] Count tokens error, returning 0", zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	return rsp, nil
}
