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
	totals := make(map[string]*dto.ModelUsageItem)
	order := make([]string, 0)
	for _, p := range points {
		if _, ok := totals[p.Model]; !ok {
			order = append(order, p.Model)
			totals[p.Model] = &dto.ModelUsageItem{Model: p.Model}
		}
		t := totals[p.Model]
		t.InputTokens += p.InputTokens
		t.OutputTokens += p.OutputTokens
		t.CacheReadTokens += p.CacheReadTokens
		t.CacheCreationTokens += p.CacheCreationTokens
	}
	items := lo.Map(order, func(m string, _ int) *dto.ModelUsageItem { return totals[m] })
	return items
}
