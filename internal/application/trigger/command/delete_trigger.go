package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
)

type deleteTriggerHandler struct {
	repo          trigger.TriggerRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewDeleteTriggerHandler(repo trigger.TriggerRepository, rebuildNotify, notifyChanged func(ctx context.Context)) port.DeleteTriggerHandler {
	return &deleteTriggerHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *deleteTriggerHandler) Handle(ctx context.Context, cmd port.DeleteTriggerCommand) error {
	// 空列表视为无操作（防御，调用侧已校验）
	if len(cmd.TriggerIDs) == 0 {
		return nil
	}
	err := h.repo.DeleteBatch(ctx, cmd.TriggerIDs)
	if err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return nil
}
