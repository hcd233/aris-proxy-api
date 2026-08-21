package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
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
		SortParam: model.SortParam{Sort: enum.SortDesc, SortField: constant.FieldCreatedAt},
	}
	projections, pageInfo, err := h.readRepo.ListSessionsByIDs(ctx, ids, param)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list demo sessions by ids")
	}

	log := logger.WithCtx(ctx)
	firstQuestionIDs := lo.FilterMap(projections, func(p *session.SessionSummaryProjection, _ int) (uint, bool) {
		if len(p.Questions) > 0 {
			return p.Questions[0], true
		}
		return 0, false
	})
	var msgByID map[uint]*session.MessageDetailProjection
	if len(firstQuestionIDs) > 0 {
		msgs, msgErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(firstQuestionIDs))
		if msgErr != nil {
			log.Warn("[DemoQuery] Failed to load questions[first] messages for summary", zap.Error(msgErr))
		} else {
			msgByID = lo.SliceToMap(msgs, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
				return m.ID, m
			})
		}
	}

	views := lo.Map(projections, func(p *session.SessionSummaryProjection, _ int) *port.DemoSessionView {
		summary := ""
		if len(p.Questions) > 0 {
			if m, ok := msgByID[p.Questions[0]]; ok && m.Message != nil {
				summary = util.ExtractMessageText(m.Message.Content)
			}
		}
		return &port.DemoSessionView{
			ID:           p.ID,
			Summary:      summary,
			Score:        p.Score,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
			CreatedAt:    p.CreatedAt,
			ModelIDs:     p.ModelIDs,
		}
	})
	return views, pageInfo, nil
}
