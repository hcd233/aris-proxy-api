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

type FirstTokenLatencyQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type FirstTokenLatencyByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type FirstTokenLatencyHandler interface {
	Handle(ctx context.Context, q FirstTokenLatencyQuery) ([]*dto.FirstTokenLatencyItem, error)
}

type FirstTokenLatencyByUserHandler interface {
	Handle(ctx context.Context, q FirstTokenLatencyByUserQuery) ([]*dto.FirstTokenLatencyItem, error)
}

type firstTokenLatencyHandler struct {
	repo modelcall.AuditRepository
}

type firstTokenLatencyByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewFirstTokenLatencyHandler(repo modelcall.AuditRepository) FirstTokenLatencyHandler {
	return &firstTokenLatencyHandler{repo: repo}
}

func NewFirstTokenLatencyByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) FirstTokenLatencyByUserHandler {
	return &firstTokenLatencyByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *firstTokenLatencyHandler) Handle(ctx context.Context, q FirstTokenLatencyQuery) ([]*dto.FirstTokenLatencyItem, error) {
	points, err := h.repo.QueryFirstTokenLatency(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillFirstTokenLatencySeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func (h *firstTokenLatencyByUserHandler) Handle(ctx context.Context, q FirstTokenLatencyByUserQuery) ([]*dto.FirstTokenLatencyItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryFirstTokenLatency(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillFirstTokenLatencySeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func fillFirstTokenLatencySeries(points []*modelcall.FirstTokenLatencyPoint, start, end time.Time, granularity enum.Granularity) []*dto.FirstTokenLatencyItem {
	type latencySlot struct{ avgLatencyMs float64 }
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.FirstTokenLatencyPoint) string { return p.Model },
		func(p *modelcall.FirstTokenLatencyPoint) time.Time { return p.Time.UTC() },
		func(p *modelcall.FirstTokenLatencyPoint) latencySlot {
			return latencySlot{avgLatencyMs: p.AverageLatencyMs}
		},
	)
	buckets := buildBuckets(start.UTC(), end.UTC(), granularity, timeSet)
	items := lo.Map(modelOrder, func(m string, _ int) *dto.FirstTokenLatencyItem {
		pts := lo.Map(buckets, func(t time.Time, _ int) *dto.FirstTokenLatencyPoint {
			s := byModel[m][t]
			return &dto.FirstTokenLatencyPoint{Time: t, AverageLatencyMs: s.avgLatencyMs}
		})
		return &dto.FirstTokenLatencyItem{Model: m, Points: pts}
	})
	return items
}
