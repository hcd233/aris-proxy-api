package trigger

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type TriggerRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error)
	Create(ctx context.Context, word *aggregate.Trigger) (uint, error)
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
	UpdateAction(ctx context.Context, id uint, action string) error
	Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error)
	ListAll(ctx context.Context) ([]*aggregate.Trigger, error)
	BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error
}
