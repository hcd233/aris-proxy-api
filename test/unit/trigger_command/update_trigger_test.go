package trigger_command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type updateFakeRepo struct {
	updatedID     uint
	updatedAction string
}

func (f *updateFakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}

func (f *updateFakeRepo) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
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

func (f *updateFakeRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *updateFakeRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	return nil, nil
}

func (f *updateFakeRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ triggerdomain.TriggerRepository = (*updateFakeRepo)(nil)

func TestUpdateTriggerHandler_Success(t *testing.T) {
	t.Parallel()
	repo := &updateFakeRepo{}
	rebuildCalled := false
	h := command.NewUpdateTriggerHandler(repo, func(ctx context.Context) { rebuildCalled = true }, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.UpdateTriggerCommand{TriggerID: 7, Action: enum.TriggerActionOmit})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updatedID != 7 || repo.updatedAction != enum.TriggerActionOmit {
		t.Fatalf("expected update (7, omit), got (%d, %q)", repo.updatedID, repo.updatedAction)
	}
	if !rebuildCalled {
		t.Fatal("expected rebuildNotify to be called")
	}
}

func TestUpdateTriggerHandler_CaptureAction(t *testing.T) {
	t.Parallel()
	repo := &updateFakeRepo{}
	h := command.NewUpdateTriggerHandler(repo, func(ctx context.Context) {}, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.UpdateTriggerCommand{TriggerID: 7, Action: enum.TriggerActionCapture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updatedID != 7 || repo.updatedAction != enum.TriggerActionCapture {
		t.Fatalf("expected update (7, capture), got (%d, %q)", repo.updatedID, repo.updatedAction)
	}
}

func TestUpdateTriggerHandler_InvalidAction(t *testing.T) {
	t.Parallel()
	repo := &updateFakeRepo{}
	h := command.NewUpdateTriggerHandler(repo, func(ctx context.Context) {}, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.UpdateTriggerCommand{TriggerID: 7, Action: "ban"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
