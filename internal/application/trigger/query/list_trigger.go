package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type listTriggerHandler struct {
	repo trigger.TriggerRepository
}

func NewListTriggerHandler(repo trigger.TriggerRepository) port.ListTriggerHandler {
	return &listTriggerHandler{repo: repo}
}

func (h *listTriggerHandler) Handle(ctx context.Context, q port.ListTriggerQuery) ([]*port.TriggerView, *model.PageInfo, error) {
	items, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		return nil, nil, err
	}
	views := lo.Map(items, func(b *aggregate.Trigger, _ int) *port.TriggerView {
		return &port.TriggerView{
			ID:        b.AggregateID(),
			Word:      b.Word(),
			Action:    b.Action(),
			HitCount:  b.HitCount(),
			CreatedAt: b.CreatedAt(),
		}
	})
	return views, pageInfo, nil
}
