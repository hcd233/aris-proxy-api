package blocked_command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/command"
	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type updateFakeRepo struct {
	updatedID     uint
	updatedAction string
}

func (f *updateFakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) {
	return nil, nil
}

func (f *updateFakeRepo) Create(ctx context.Context, word *aggregate.Blocked) (uint, error) {
	return 0, nil
}

func (f *updateFakeRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *updateFakeRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	return nil
}

func (f *updateFakeRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	f.updatedID = id
	f.updatedAction = action
	return nil
}

func (f *updateFakeRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *updateFakeRepo) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	return nil, nil
}

func (f *updateFakeRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ blockeddomain.BlockedRepository = (*updateFakeRepo)(nil)

func TestUpdateBlockedHandler_Success(t *testing.T) {
	t.Parallel()
	repo := &updateFakeRepo{}
	rebuildCalled := false
	h := command.NewUpdateBlockedHandler(repo, func(ctx context.Context) { rebuildCalled = true }, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.UpdateBlockedCommand{BlockedID: 7, Action: enum.BlockedActionOmit})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updatedID != 7 || repo.updatedAction != enum.BlockedActionOmit {
		t.Fatalf("expected update (7, omit), got (%d, %q)", repo.updatedID, repo.updatedAction)
	}
	if !rebuildCalled {
		t.Fatal("expected rebuildNotify to be called")
	}
}

func TestUpdateBlockedHandler_InvalidAction(t *testing.T) {
	t.Parallel()
	repo := &updateFakeRepo{}
	h := command.NewUpdateBlockedHandler(repo, func(ctx context.Context) {}, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.UpdateBlockedCommand{BlockedID: 7, Action: "ban"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
