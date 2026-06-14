package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateBlockedCommand struct {
	Word string
}

type CreateBlockedResult struct {
	BlockedID uint
}

type CreateBlockedHandler interface {
	Handle(ctx context.Context, cmd CreateBlockedCommand) (*CreateBlockedResult, error)
}

type DeleteBlockedCommand struct {
	BlockedID uint
}

type DeleteBlockedHandler interface {
	Handle(ctx context.Context, cmd DeleteBlockedCommand) error
}

type BlockedView struct {
	ID        uint
	Word      string
	HitCount  uint
	CreatedAt time.Time
}

type ListBlockedQuery struct {
	model.CommonParam
}

type ListBlockedHandler interface {
	Handle(ctx context.Context, q ListBlockedQuery) ([]*BlockedView, *model.PageInfo, error)
}
