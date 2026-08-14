package trigger_command_test

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type deleteFakeRepo struct {
	deletedIDs []uint
}

func (f *deleteFakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}

func (f *deleteFakeRepo) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
	return 0, nil
}

func (f *deleteFakeRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *deleteFakeRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	f.deletedIDs = ids
	return nil
}

func (f *deleteFakeRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *deleteFakeRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *deleteFakeRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	return nil, nil
}

func (f *deleteFakeRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ triggerdomain.TriggerRepository = (*deleteFakeRepo)(nil)

func TestDeleteTriggerHandler_Batch(t *testing.T) {
	t.Parallel()
	repo := &deleteFakeRepo{}
	rebuildCalled := false
	h := command.NewDeleteTriggerHandler(repo, func(ctx context.Context) { rebuildCalled = true }, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.DeleteTriggerCommand{TriggerIDs: []uint{1, 2, 3}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deletedIDs) != 3 || repo.deletedIDs[0] != 1 || repo.deletedIDs[2] != 3 {
		t.Fatalf("expected DeleteBatch with [1 2 3], got %v", repo.deletedIDs)
	}
	if !rebuildCalled {
		t.Fatal("expected rebuildNotify to be called")
	}
}

func TestDeleteTriggerHandler_EmptyIDs(t *testing.T) {
	t.Parallel()
	repo := &deleteFakeRepo{}
	h := command.NewDeleteTriggerHandler(repo, func(ctx context.Context) {}, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.DeleteTriggerCommand{TriggerIDs: []uint{}})
	if err != nil {
		t.Fatalf("empty ids should be a no-op success, got error: %v", err)
	}
	if repo.deletedIDs != nil {
		t.Fatalf("expected no DeleteBatch call, got %v", repo.deletedIDs)
	}
}
