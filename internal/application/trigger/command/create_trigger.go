package command

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type createTriggerHandler struct {
	repo          trigger.TriggerRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewCreateTriggerHandler(repo trigger.TriggerRepository, rebuildNotify, notifyChanged func(ctx context.Context)) port.CreateTriggerHandler {
	return &createTriggerHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *createTriggerHandler) Handle(ctx context.Context, cmd port.CreateTriggerCommand) (*port.CreateTriggerResult, error) {
	b, err := aggregate.CreateTrigger(0, cmd.Word, cmd.Action)
	if err != nil {
		return nil, err
	}
	id, err := h.repo.Create(ctx, b)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 唯一索引 (word, deleted_at) 冲突：同词已存在（含软删除记录占用索引的场景）。
			return nil, ierr.Wrap(ierr.ErrDataExists, err, "trigger word already exists")
		}
		return nil, err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return &port.CreateTriggerResult{TriggerID: id}, nil
}
