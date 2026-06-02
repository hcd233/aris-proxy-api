package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

type TokenThroughputQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenThroughputByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenThroughputHandler interface {
	Handle(ctx context.Context, q TokenThroughputQuery) ([]*modelcall.TokenThroughputPoint, error)
}

type TokenThroughputByUserHandler interface {
	Handle(ctx context.Context, q TokenThroughputByUserQuery) ([]*modelcall.TokenThroughputPoint, error)
}

type tokenThroughputHandler struct {
	repo modelcall.AuditRepository
}

type tokenThroughputByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenThroughputHandler(repo modelcall.AuditRepository) TokenThroughputHandler {
	return &tokenThroughputHandler{repo: repo}
}

func NewTokenThroughputByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenThroughputByUserHandler {
	return &tokenThroughputByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenThroughputHandler) Handle(ctx context.Context, q TokenThroughputQuery) ([]*modelcall.TokenThroughputPoint, error) {
	return h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *tokenThroughputByUserHandler) Handle(ctx context.Context, q TokenThroughputByUserQuery) ([]*modelcall.TokenThroughputPoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
