package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type getTraceHandler struct {
	repo trace.TraceRepository
}

// NewGetTraceHandler 构造详情 handler
func NewGetTraceHandler(repo trace.TraceRepository) port.GetTraceHandler {
	return &getTraceHandler{repo: repo}
}

func (h *getTraceHandler) Handle(ctx context.Context, q port.GetTraceQuery) (*port.TraceDetailView, error) {
	t, err := h.repo.FindByID(ctx, q.TraceID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "trace not found")
	}
	count, err := h.repo.CountEvents(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	return &port.TraceDetailView{
		ID: t.ID, SessionID: t.SessionID, Agent: t.Agent, APIKeyName: t.APIKeyName,
		Model: t.Model, CWD: t.CWD, Source: t.Source, Status: t.Status,
		Metadata: t.Metadata, EventCount: count, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}, nil
}
