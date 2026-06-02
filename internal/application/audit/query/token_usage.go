package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type TokenUsageQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenUsageByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenUsageHandler interface {
	Handle(ctx context.Context, q TokenUsageQuery) ([]*dto.TokenUsageItem, error)
}

type TokenUsageByUserHandler interface {
	Handle(ctx context.Context, q TokenUsageByUserQuery) ([]*dto.TokenUsageItem, error)
}

type tokenUsageHandler struct {
	repo modelcall.AuditRepository
}

type tokenUsageByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenUsageHandler(repo modelcall.AuditRepository) TokenUsageHandler {
	return &tokenUsageHandler{repo: repo}
}

func NewTokenUsageByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenUsageByUserHandler {
	return &tokenUsageByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenUsageHandler) Handle(ctx context.Context, q TokenUsageQuery) ([]*dto.TokenUsageItem, error) {
	points, err := h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateTokenUsage(points), nil
}

func (h *tokenUsageByUserHandler) Handle(ctx context.Context, q TokenUsageByUserQuery) ([]*dto.TokenUsageItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateTokenUsage(points), nil
}

func aggregateTokenUsage(points []*modelcall.TokenThroughputPoint) []*dto.TokenUsageItem {
	totals := make(map[string]*dto.TokenUsageItem)
	order := make([]string, 0)
	for _, p := range points {
		if _, ok := totals[p.Model]; !ok {
			order = append(order, p.Model)
			totals[p.Model] = &dto.TokenUsageItem{Model: p.Model}
		}
		t := totals[p.Model]
		t.InputTokens += p.InputTokens
		t.OutputTokens += p.OutputTokens
		t.CacheReadTokens += p.CacheReadTokens
		t.CacheCreationTokens += p.CacheCreationTokens
	}
	items := make([]*dto.TokenUsageItem, 0, len(order))
	for _, m := range order {
		items = append(items, totals[m])
	}
	return items
}
