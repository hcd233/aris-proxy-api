package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
)

type updateTriggerHandler struct {
	repo          trigger.TriggerRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewUpdateTriggerHandler(repo trigger.TriggerRepository, rebuildNotify, notifyChanged func(ctx context.Context)) port.UpdateTriggerHandler {
	return &updateTriggerHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *updateTriggerHandler) Handle(ctx context.Context, cmd port.UpdateTriggerCommand) error {
	if cmd.Action != enum.TriggerActionDeny && cmd.Action != enum.TriggerActionOmit && cmd.Action != enum.TriggerActionCapture {
		return ierr.New(ierr.ErrValidation, "invalid trigger action, must be deny, omit or capture")
	}
	if err := h.repo.UpdateAction(ctx, cmd.TriggerID, cmd.Action); err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return nil
}
