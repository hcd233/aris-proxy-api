package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTraceEventsHandler struct {
	repo trace.TraceRepository
}

// NewListTraceEventsHandler 构造事件时间线 handler
func NewListTraceEventsHandler(repo trace.TraceRepository) port.ListTraceEventsHandler {
	return &listTraceEventsHandler{repo: repo}
}

func (h *listTraceEventsHandler) Handle(ctx context.Context, q port.ListTraceEventsQuery) ([]*port.TraceEventView, *model.PageInfo, error) {
	events, pageInfo, err := h.repo.ListEvents(ctx, q.TraceID, model.CommonParam{PageParam: model.PageParam{Page: q.Page, PageSize: q.PageSize}})
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
