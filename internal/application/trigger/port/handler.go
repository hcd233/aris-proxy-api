package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateTriggerCommand struct {
	Word   string
	Action string
}

type CreateTriggerResult struct {
	TriggerID uint
}

type CreateTriggerHandler interface {
	Handle(ctx context.Context, cmd CreateTriggerCommand) (*CreateTriggerResult, error)
}

type DeleteTriggerCommand struct {
	TriggerIDs []uint
}

type DeleteTriggerHandler interface {
	Handle(ctx context.Context, cmd DeleteTriggerCommand) error
}

type UpdateTriggerCommand struct {
	TriggerID uint
	Action    string
}

type UpdateTriggerHandler interface {
	Handle(ctx context.Context, cmd UpdateTriggerCommand) error
}

type TriggerView struct {
	ID        uint
	Word      string
	Action    string
	HitCount  uint
	CreatedAt time.Time
}

type ListTriggerQuery struct {
	model.CommonParam
}

type ListTriggerHandler interface {
	Handle(ctx context.Context, q ListTriggerQuery) ([]*TriggerView, *model.PageInfo, error)
}
