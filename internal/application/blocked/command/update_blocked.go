package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type updateBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewUpdateBlockedHandler(repo blocked.BlockedRepository, rebuildNotify, notifyChanged func(ctx context.Context)) port.UpdateBlockedHandler {
	return &updateBlockedHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *updateBlockedHandler) Handle(ctx context.Context, cmd port.UpdateBlockedCommand) error {
	if cmd.Action != enum.BlockedActionDeny && cmd.Action != enum.BlockedActionOmit {
		return ierr.New(ierr.ErrValidation, "invalid blocked action, must be deny or omit")
	}
	if err := h.repo.UpdateAction(ctx, cmd.BlockedID, cmd.Action); err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return nil
}
