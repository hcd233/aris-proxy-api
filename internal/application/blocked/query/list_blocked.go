package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type listBlockedHandler struct {
	repo blocked.BlockedRepository
}

func NewListBlockedHandler(repo blocked.BlockedRepository) port.ListBlockedHandler {
	return &listBlockedHandler{repo: repo}
}

func (h *listBlockedHandler) Handle(ctx context.Context, q port.ListBlockedQuery) ([]*port.BlockedView, *model.PageInfo, error) {
	items, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		return nil, nil, err
	}
	views := lo.Map(items, func(b *aggregate.Blocked, _ int) *port.BlockedView {
		return &port.BlockedView{
			ID:        b.AggregateID(),
			Word:      b.Word(),
			HitCount:  b.HitCount(),
			CreatedAt: b.CreatedAt(),
		}
	})
	return views, pageInfo, nil
}
