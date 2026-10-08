package usecase

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ListClientModels 客户端模型列表用例
type ListClientModels struct {
	readRepo llmproxy.EndpointReadRepository
}

// NewListClientModels 构造客户端模型列表用例
func NewListClientModels(readRepo llmproxy.EndpointReadRepository) *ListClientModels {
	return &ListClientModels{readRepo: readRepo}
}

// Handle 返回启用中的模型列表（含能力与长度限制，仅当前 API Key 所属用户的模型）
func (q *ListClientModels) Handle(ctx context.Context) (*dto.ClientModelsRsp, error) {
	projections, err := q.readRepo.ListEnabledModelDetails(ctx, util.CtxValueUint(ctx, constant.CtxKeyUserID))
	if err != nil {
		logger.WithCtx(ctx).Error("[ClientQuery] Failed to list model details", zap.Error(err))
		return nil, err
	}
	return &dto.ClientModelsRsp{
		Models: lo.Map(projections, func(p *llmproxy.ModelDetailProjection, _ int) *dto.ClientModelItem {
			return &dto.ClientModelItem{
				Alias:           p.Alias,
				UpstreamModel:   p.UpstreamModel,
				ContextLength:   p.ContextLength,
				MaxOutputTokens: p.MaxOutputTokens,
				Capabilities:    lo.Map(p.Capabilities, func(c string, _ int) enum.InputModality { return c }),
				Cost:            toClientModelCost(p.Pricing),
			}
		}),
	}, nil
}

// toClientModelCost 定价 → 客户端导出的基础档单价（展示单位）；未计价返回 nil
func toClientModelCost(p vo.Pricing) *dto.ClientModelCost {
	rule, ok := p.BaseRule()
	if !ok {
		return nil
	}
	return &dto.ClientModelCost{
		Input:      dto.PriceDisplayFromMicro(rule.InputMicro),
		Output:     dto.PriceDisplayFromMicro(rule.OutputMicro),
		CacheRead:  dto.PriceDisplayFromMicro(rule.CacheReadMicro),
		CacheWrite: dto.PriceDisplayFromMicro(rule.CacheCreateMicro),
	}
}
