package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type getTraceHandler struct {
	repo       trace.TraceRepository
	authorizer *traceAuthorizer
}

// NewGetTraceHandler 构造详情 handler
func NewGetTraceHandler(
	repo trace.TraceRepository,
	apiKeyRepo apikeydomain.APIKeyRepository,
) port.GetTraceHandler {
	return &getTraceHandler{repo: repo, authorizer: newTraceAuthorizer(repo, apiKeyRepo)}
}

func (h *getTraceHandler) Handle(
	ctx context.Context,
	q port.GetTraceQuery,
) (*port.TraceDetailView, error) {
	item, err := h.authorizer.Find(ctx, q.UserID, q.IsAdmin, q.TraceID)
	if err != nil {
		return nil, err
	}
	count, err := h.repo.CountEvents(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return &port.TraceDetailView{
		ID:         item.ID,
		SessionID:  item.SessionID,
		Agent:      item.Agent,
		APIKeyName: item.APIKeyName,
		Model:      item.Model,
		CWD:        item.CWD,
		Source:     item.Source,
		Status:     item.Status,
		Metadata:   item.Metadata,
		EventCount: count,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}, nil
}
