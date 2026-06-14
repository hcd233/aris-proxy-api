package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type deleteBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
}

func NewDeleteBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func(ctx context.Context)) port.DeleteBlockedHandler {
	return &deleteBlockedHandler{repo: repo, rebuildNotify: rebuildNotify}
}

func (h *deleteBlockedHandler) Handle(ctx context.Context, cmd port.DeleteBlockedCommand) error {
	err := h.repo.Delete(ctx, cmd.BlockedID)
	if err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	return nil
}
