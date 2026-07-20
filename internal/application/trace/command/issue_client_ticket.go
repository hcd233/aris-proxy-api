package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

type issueTraceClientTicketHandler struct {
	store port.TraceClientTicketStore
}

func NewIssueTraceClientTicketHandler(
	store port.TraceClientTicketStore,
) port.IssueTraceClientTicketHandler {
	return &issueTraceClientTicketHandler{store: store}
}

func (h *issueTraceClientTicketHandler) Handle(
	ctx context.Context,
	cmd port.IssueTraceClientTicketCommand,
) (*port.TraceClientTicketView, error) {
	ticket, expiresAt, err := h.store.Issue(ctx, cmd.UserID, constant.TraceClientTicketTTL)
	if err != nil {
		return nil, err
	}
	return &port.TraceClientTicketView{Ticket: ticket, ExpiresAt: expiresAt}, nil
}
