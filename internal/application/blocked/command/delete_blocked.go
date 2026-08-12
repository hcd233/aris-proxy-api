package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type deleteBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewDeleteBlockedHandler(repo blocked.BlockedRepository, rebuildNotify, notifyChanged func(ctx context.Context)) port.DeleteBlockedHandler {
	return &deleteBlockedHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *deleteBlockedHandler) Handle(ctx context.Context, cmd port.DeleteBlockedCommand) error {
	// 空列表视为无操作（防御，调用侧已校验）
	if len(cmd.BlockedIDs) == 0 {
		return nil
	}
	err := h.repo.DeleteBatch(ctx, cmd.BlockedIDs)
	if err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return nil
}
