package trigger_command_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type createFakeRepo struct {
	createErr error
	created   *aggregate.Trigger
}

func (f *createFakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}

func (f *createFakeRepo) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
	f.created = word
	return 1, f.createErr
}

func (f *createFakeRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *createFakeRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	return nil
}

func (f *createFakeRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *createFakeRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *createFakeRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	return nil, nil
}

func (f *createFakeRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ triggerdomain.TriggerRepository = (*createFakeRepo)(nil)

func TestCreateTriggerHandler_Success(t *testing.T) {
	t.Parallel()
	repo := &createFakeRepo{}
	rebuildCalled := false
	h := command.NewCreateTriggerHandler(repo, func(ctx context.Context) { rebuildCalled = true }, func(ctx context.Context) {})

	result, err := h.Handle(context.Background(), port.CreateTriggerCommand{Word: "你好", Action: enum.TriggerActionDeny})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TriggerID != 1 {
		t.Fatalf("expected TriggerID=1, got %d", result.TriggerID)
	}
	if repo.created == nil || repo.created.Word() != "你好" {
		t.Fatalf("expected created word 你好, got %+v", repo.created)
	}
	if !rebuildCalled {
		t.Fatal("expected rebuildNotify to be called")
	}
}

// TestCreateTriggerHandler_DuplicatedKey 回归：唯一键冲突（含软删除记录占用 word 单列唯一索引）
// 应映射为 ErrDataExists（HTTP 409），而不是透传底层错误导致 500。
func TestCreateTriggerHandler_DuplicatedKey(t *testing.T) {
	t.Parallel()
	repo := &createFakeRepo{createErr: gorm.ErrDuplicatedKey}
	rebuildCalled := false
	h := command.NewCreateTriggerHandler(repo, func(ctx context.Context) { rebuildCalled = true }, func(ctx context.Context) {})

	_, err := h.Handle(context.Background(), port.CreateTriggerCommand{Word: "你好", Action: enum.TriggerActionDeny})
	if !errors.Is(err, ierr.ErrDataExists) {
		t.Fatalf("expected ErrDataExists, got %v", err)
	}
	if rebuildCalled {
		t.Fatal("expected rebuildNotify NOT to be called on failure")
	}
}

func TestCreateTriggerHandler_OtherError_Passthrough(t *testing.T) {
	t.Parallel()
	otherErr := ierr.New(ierr.ErrInternal, "db down")
	repo := &createFakeRepo{createErr: otherErr}
	h := command.NewCreateTriggerHandler(repo, func(ctx context.Context) { /* rebuildNotify */ }, func(ctx context.Context) { /* notifyChanged */ })

	_, err := h.Handle(context.Background(), port.CreateTriggerCommand{Word: "你好", Action: enum.TriggerActionDeny})
	if !errors.Is(err, otherErr) {
		t.Fatalf("expected original error to pass through, got %v", err)
	}
	if errors.Is(err, ierr.ErrDataExists) {
		t.Fatalf("expected not ErrDataExists, got %v", err)
	}
}
