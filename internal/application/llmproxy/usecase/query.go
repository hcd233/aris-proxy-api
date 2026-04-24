// Package usecase LLMProxy 域查询（Query）侧
//
// ListModels / CountTokens 是纯读路径，绕过聚合重建，直接走 DAO 投影，
// 语义与原 service 的 ListModels / CountTokens 完全一致。
package usecase

import (
	"context"
	"time"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ============================================================
// ListOpenAIModels OpenAI 模型列表查询
// ============================================================

// ListOpenAIModels OpenAI 模型列表查询
type ListOpenAIModels interface {
	Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
}

type listOpenAIModels struct {
	dao *dao.ModelEndpointDAO
}

// NewListOpenAIModels 构造 OpenAI ListModels 查询
//
//	@return ListOpenAIModels
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func NewListOpenAIModels() ListOpenAIModels {
	return &listOpenAIModels{dao: dao.GetModelEndpointDAO()}
}

// Handle 查询 provider=openai 的所有 alias
func (q *listOpenAIModels) Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	db := database.GetDBInstance(ctx)
	endpoints, err := q.dao.BatchGet(db, &dbmodel.ModelEndpoint{Provider: enum.ProviderOpenAI}, []string{"alias"})
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIQuery] Failed to query model endpoints", zap.Error(err))
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

// ============================================================
// ListAnthropicModels Anthropic 模型列表查询
// ============================================================

// ListAnthropicModels Anthropic 模型列表查询
type ListAnthropicModels interface {
	Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
}

type listAnthropicModels struct {
	dao *dao.ModelEndpointDAO
}

// NewListAnthropicModels 构造 Anthropic ListModels 查询
//
//	@return ListAnthropicModels
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func NewListAnthropicModels() ListAnthropicModels {
	return &listAnthropicModels{dao: dao.GetModelEndpointDAO()}
}

// Handle 查询 provider=anthropic 的所有 alias
func (q *listAnthropicModels) Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	db := database.GetDBInstance(ctx)
	endpoints, err := q.dao.BatchGet(db, &dbmodel.ModelEndpoint{Provider: enum.ProviderAnthropic}, []string{"alias"})
	if err != nil {
		logger.WithCtx(ctx).Error("[AnthropicQuery] Failed to query model endpoints", zap.Error(err))
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

// ============================================================
// CountTokens Anthropic /v1/messages/count_tokens
// ============================================================

// CountTokens Anthropic Token 计数查询
type CountTokens interface {
	Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type countTokens struct {
	dao    *dao.ModelEndpointDAO
	proxy  transport.AnthropicProxy
	fields []string
}

// NewCountTokens 构造 Anthropic CountTokens 查询
//
//	@param proxy transport.AnthropicProxy
//	@return CountTokens
//	@author centonhuang
//	@update 2026-04-22 20:45:00
func NewCountTokens(proxy transport.AnthropicProxy) CountTokens {
	return &countTokens{
		dao:    dao.GetModelEndpointDAO(),
		proxy:  proxy,
		fields: []string{"model", "api_key", "base_url"},
	}
}

// Handle 调用上游 count_tokens；任何错误（包括 model not found）返回空结果（与旧行为一致）
func (q *countTokens) Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	endpoint, err := q.dao.Get(db, &dbmodel.ModelEndpoint{
		Alias:    req.Body.Model,
		Provider: enum.ProviderAnthropic,
	}, q.fields)
	if err != nil {
		log.Warn("[AnthropicQuery] Model not found, returning 0", zap.String("model", req.Body.Model), zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}

	upstream := transport.UpstreamEndpoint{Model: endpoint.Model, APIKey: endpoint.APIKey, BaseURL: endpoint.BaseURL}
	body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)

	rsp, err := q.proxy.ForwardCountTokens(ctx, upstream, body)
	if err != nil {
		log.Warn("[AnthropicQuery] Count tokens error, returning 0", zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	return rsp, nil
}
