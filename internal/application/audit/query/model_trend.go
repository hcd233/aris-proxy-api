package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

type ModelTrendQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

type ModelTrendByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

type ModelTrendHandler interface {
	Handle(ctx context.Context, q ModelTrendQuery) ([]*modelcall.ModelTrendPoint, error)
}

type ModelTrendByUserHandler interface {
	Handle(ctx context.Context, q ModelTrendByUserQuery) ([]*modelcall.ModelTrendPoint, error)
}

type modelTrendHandler struct {
	repo modelcall.AuditRepository
}

type modelTrendByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs apiKeyIDLookup
}

func NewModelTrendHandler(repo modelcall.AuditRepository) ModelTrendHandler {
	return &modelTrendHandler{repo: repo}
}

func NewModelTrendByUserHandler(repo modelcall.AuditRepository, apiKeyIDs apiKeyIDLookup) ModelTrendByUserHandler {
	return &modelTrendByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *modelTrendHandler) Handle(ctx context.Context, q ModelTrendQuery) ([]*modelcall.ModelTrendPoint, error) {
	return h.repo.QueryModelTrend(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *modelTrendByUserHandler) Handle(ctx context.Context, q ModelTrendByUserQuery) ([]*modelcall.ModelTrendPoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryModelTrend(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
