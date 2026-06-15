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

type ModelUsageQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type ModelUsageByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type ModelUsageHandler interface {
	Handle(ctx context.Context, q ModelUsageQuery) ([]*dto.ModelUsageItem, error)
}

type ModelUsageByUserHandler interface {
	Handle(ctx context.Context, q ModelUsageByUserQuery) ([]*dto.ModelUsageItem, error)
}

type modelUsageHandler struct {
	repo modelcall.AuditRepository
}

type modelUsageByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewModelUsageHandler(repo modelcall.AuditRepository) ModelUsageHandler {
	return &modelUsageHandler{repo: repo}
}

func NewModelUsageByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) ModelUsageByUserHandler {
	return &modelUsageByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *modelUsageHandler) Handle(ctx context.Context, q ModelUsageQuery) ([]*dto.ModelUsageItem, error) {
	points, err := h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateModelUsage(points), nil
}

func (h *modelUsageByUserHandler) Handle(ctx context.Context, q ModelUsageByUserQuery) ([]*dto.ModelUsageItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateModelUsage(points), nil
}

func aggregateModelUsage(points []*modelcall.TokenThroughputPoint) []*dto.ModelUsageItem {
	groups := lo.GroupBy(points, func(p *modelcall.TokenThroughputPoint) string {
		return p.Model
	})
	return lo.Map(lo.Keys(groups), func(model string, _ int) *dto.ModelUsageItem {
		item := &dto.ModelUsageItem{Model: model}
		for _, p := range groups[model] {
			item.InputTokens += p.InputTokens
			item.OutputTokens += p.OutputTokens
			item.CacheReadTokens += p.CacheReadTokens
			item.CacheCreationTokens += p.CacheCreationTokens
		}
		return item
	})
}
