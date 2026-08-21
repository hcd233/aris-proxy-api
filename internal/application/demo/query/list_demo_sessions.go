package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

type listDemoSessionsHandler struct {
	demoRepo port.DemoSessionRepository
	readRepo session.SessionReadRepository
}

func NewListDemoSessionsHandler(demoRepo port.DemoSessionRepository, readRepo session.SessionReadRepository) port.ListDemoSessionsHandler {
	return &listDemoSessionsHandler{demoRepo: demoRepo, readRepo: readRepo}
}

func (h *listDemoSessionsHandler) Handle(ctx context.Context, q port.ListDemoSessionsQuery) ([]*port.DemoSessionView, *model.PageInfo, error) {
	ids, err := h.demoRepo.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return []*port.DemoSessionView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
	}
	param := model.CommonParam{
		PageParam: model.PageParam{Page: max(q.Page, 1), PageSize: q.PageSize},
	}
	projections, pageInfo, err := h.readRepo.ListSessionsByIDs(ctx, ids, param)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list demo sessions by ids")
	}
	views := lo.Map(projections, func(p *session.SessionSummaryProjection, _ int) *port.DemoSessionView {
		return &port.DemoSessionView{
			ID:           p.ID,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
			CreatedAt:    p.CreatedAt,
		}
	})
	return views, pageInfo, nil
}
