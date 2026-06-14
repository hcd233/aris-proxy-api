package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type createBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
}

func NewCreateBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func(ctx context.Context)) port.CreateBlockedHandler {
	return &createBlockedHandler{repo: repo, rebuildNotify: rebuildNotify}
}

func (h *createBlockedHandler) Handle(ctx context.Context, cmd port.CreateBlockedCommand) (*port.CreateBlockedResult, error) {
	b, err := aggregate.CreateBlocked(0, cmd.Word)
	if err != nil {
		return nil, err
	}
	id, err := h.repo.Create(ctx, b)
	if err != nil {
		return nil, err
	}
	h.rebuildNotify(ctx)
	return &port.CreateBlockedResult{BlockedID: id}, nil
}
