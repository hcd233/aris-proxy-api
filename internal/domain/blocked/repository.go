package blocked

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type BlockedRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error)
	Create(ctx context.Context, word *aggregate.Blocked) (uint, error)
	Delete(ctx context.Context, id uint) error
	Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error)
	ListAll(ctx context.Context) ([]*aggregate.Blocked, error)
	BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error
}
