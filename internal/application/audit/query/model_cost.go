package query

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ModelCostQuery 模型成本排行查询（admin/demo 全量视角）
type ModelCostQuery struct {
	StartTime time.Time
	EndTime   time.Time
}

// ModelCostByUserQuery 模型成本排行查询（user 视角，限定名下 key）
type ModelCostByUserQuery struct {
	UserID    uint
	StartTime time.Time
	EndTime   time.Time
}

// ModelCostHandler 模型成本排行处理器（admin/demo 全量视角）
type ModelCostHandler interface {
	Handle(ctx context.Context, q ModelCostQuery) ([]*dto.ModelCostItem, error)
}

// ModelCostByUserHandler 模型成本排行处理器（user 视角）
type ModelCostByUserHandler interface {
	Handle(ctx context.Context, q ModelCostByUserQuery) ([]*dto.ModelCostItem, error)
}

// ModelCostRepository 模型成本查询的仓储依赖
type ModelCostRepository interface {
	QueryModelCost(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]*modelcall.ModelCostPoint, error)
}

type modelCostHandler struct {
	repo ModelCostRepository
}

type modelCostByUserHandler struct {
	repo      ModelCostRepository
	apiKeyIDs port.APIKeyIDLookup
}

// NewModelCostHandler 构造模型成本排行处理器（admin/demo 全量视角）
func NewModelCostHandler(repo ModelCostRepository) ModelCostHandler {
	return &modelCostHandler{repo: repo}
}

// NewModelCostByUserHandler 构造模型成本排行处理器（user 视角）
func NewModelCostByUserHandler(repo ModelCostRepository, apiKeyIDs port.APIKeyIDLookup) ModelCostByUserHandler {
	return &modelCostByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *modelCostHandler) Handle(ctx context.Context, q ModelCostQuery) ([]*dto.ModelCostItem, error) {
	points, err := h.repo.QueryModelCost(ctx, nil, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}
	return lo.Map(points, toModelCostItem), nil
}

func (h *modelCostByUserHandler) Handle(ctx context.Context, q ModelCostByUserQuery) ([]*dto.ModelCostItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	// 名下无 API Key：直接返回空，防止空列表在仓储层退化为全量查询（越权）
	if len(keyIDs) == 0 {
		return []*dto.ModelCostItem{}, nil
	}
	points, err := h.repo.QueryModelCost(ctx, keyIDs, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}
	return lo.Map(points, toModelCostItem), nil
}

// toModelCostItem 微单位 → 展示单位（总费用=四维合计）
func toModelCostItem(p *modelcall.ModelCostPoint, _ int) *dto.ModelCostItem {
	return &dto.ModelCostItem{
		ModelID:           p.ModelID,
		Currency:          enum.Currency(p.Currency),
		InputCost:         dto.PriceDisplayFromMicro(p.InputCostMicro),
		OutputCost:        dto.PriceDisplayFromMicro(p.OutputCostMicro),
		CacheCreationCost: dto.PriceDisplayFromMicro(p.CacheCreationCostMicro),
		CacheReadCost:     dto.PriceDisplayFromMicro(p.CacheReadCostMicro),
		TotalCost:         dto.PriceDisplayFromMicro(p.TotalCostMicro()),
	}
}
