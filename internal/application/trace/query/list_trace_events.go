package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTraceEventsHandler struct {
	repo       trace.TraceRepository
	authorizer *traceAuthorizer
}

// NewListTraceEventsHandler 构造事件时间线 handler
func NewListTraceEventsHandler(
	repo trace.TraceRepository,
	apiKeyRepo apikeydomain.APIKeyRepository,
) port.ListTraceEventsHandler {
	return &listTraceEventsHandler{repo: repo, authorizer: newTraceAuthorizer(repo, apiKeyRepo)}
}

func (h *listTraceEventsHandler) Handle(
	ctx context.Context,
	q port.ListTraceEventsQuery,
) ([]*port.TraceEventView, *model.PageInfo, error) {
	item, err := h.authorizer.Find(ctx, q.UserID, q.IsAdmin, q.TraceID)
	if err != nil {
		return nil, nil, err
	}
	events, pageInfo, err := h.repo.ListEvents(ctx, item.ID, model.CommonParam{
		PageParam: model.PageParam{Page: q.Page, PageSize: q.PageSize},
	})
	if err != nil {
		return nil, nil, err
	}
	return lo.Map(events, func(item *trace.TraceEvent, _ int) *port.TraceEventView {
		return &port.TraceEventView{
			ID:             item.ID,
			Source:         item.Source,
			RecordType:     item.RecordType,
			Event:          item.Event,
			TurnID:         item.TurnID,
			CallID:         item.CallID,
			TranscriptLine: item.TranscriptLine,
			ClientSequence: item.ClientSequence,
			DedupKey:       item.DedupKey,
			Payload:        item.Payload,
			CreatedAt:      item.CreatedAt,
		}
	}), pageInfo, nil
}
