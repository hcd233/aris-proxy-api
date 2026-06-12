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

type TokenRateQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenRateByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenRateHandler interface {
	Handle(ctx context.Context, q TokenRateQuery) ([]*dto.TokenRateItem, error)
}

type TokenRateByUserHandler interface {
	Handle(ctx context.Context, q TokenRateByUserQuery) ([]*dto.TokenRateItem, error)
}

type tokenRateHandler struct {
	repo modelcall.AuditRepository
}

type tokenRateByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenRateHandler(repo modelcall.AuditRepository) TokenRateHandler {
	return &tokenRateHandler{repo: repo}
}

func NewTokenRateByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenRateByUserHandler {
	return &tokenRateByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenRateHandler) Handle(ctx context.Context, q TokenRateQuery) ([]*dto.TokenRateItem, error) {
	points, err := h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillTokenRateSeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func (h *tokenRateByUserHandler) Handle(ctx context.Context, q TokenRateByUserQuery) ([]*dto.TokenRateItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillTokenRateSeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func fillTokenRateSeries(points []*modelcall.TokenThroughputPoint, start, end time.Time, granularity enum.Granularity) []*dto.TokenRateItem {
	type rateSlot struct{ outputTokensPerSec float64 }
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.TokenThroughputPoint) string { return p.Model },
		func(p *modelcall.TokenThroughputPoint) time.Time { return p.Time.UTC() },
		func(p *modelcall.TokenThroughputPoint) rateSlot {
			return rateSlot{outputTokensPerSec: p.OutputTokensPerSecond}
		},
	)
	buckets := buildBuckets(start.UTC(), end.UTC(), granularity, timeSet)
	items := lo.Map(modelOrder, func(m string, _ int) *dto.TokenRateItem {
		pts := lo.Map(buckets, func(t time.Time, _ int) *dto.TokenRatePoint {
			s := byModel[m][t]
			return &dto.TokenRatePoint{Time: t, OutputTokensPerSecond: s.outputTokensPerSec}
		})
		return &dto.TokenRateItem{Model: m, Points: pts}
	})
	return items
}
