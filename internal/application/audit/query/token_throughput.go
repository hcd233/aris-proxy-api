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
	// 名下无 API Key：直接返回空，防止空列表在仓储层退化为全量查询（越权）
	if len(keyIDs) == 0 {
		return []*modelcall.TokenThroughputPoint{}, nil
	}
	return h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
