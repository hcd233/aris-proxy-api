package usecase

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListClientModels 客户端模型列表用例
type ListClientModels struct {
	readRepo llmproxy.EndpointReadRepository
}

// NewListClientModels 构造客户端模型列表用例
func NewListClientModels(readRepo llmproxy.EndpointReadRepository) *ListClientModels {
	return &ListClientModels{readRepo: readRepo}
}

// Handle 返回启用中的模型列表（含能力与长度限制）
func (q *ListClientModels) Handle(ctx context.Context) (*dto.ClientModelsRsp, error) {
	projections, err := q.readRepo.ListEnabledModelDetails(ctx)
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
			}
		}),
	}, nil
}
