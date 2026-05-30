package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

type RequestRateQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

type RequestRateByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

type RequestRateHandler interface {
	Handle(ctx context.Context, q RequestRateQuery) ([]*modelcall.RequestRatePoint, error)
}

type RequestRateByUserHandler interface {
	Handle(ctx context.Context, q RequestRateByUserQuery) ([]*modelcall.RequestRatePoint, error)
}

type requestRateHandler struct {
	repo modelcall.AuditRepository
}

type requestRateByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs apiKeyIDLookup
}

func NewRequestRateHandler(repo modelcall.AuditRepository) RequestRateHandler {
	return &requestRateHandler{repo: repo}
}

func NewRequestRateByUserHandler(repo modelcall.AuditRepository, apiKeyIDs apiKeyIDLookup) RequestRateByUserHandler {
	return &requestRateByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *requestRateHandler) Handle(ctx context.Context, q RequestRateQuery) ([]*modelcall.RequestRatePoint, error) {
	return h.repo.QueryRequestRate(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *requestRateByUserHandler) Handle(ctx context.Context, q RequestRateByUserQuery) ([]*modelcall.RequestRatePoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryRequestRate(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
