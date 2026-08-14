package trigger_matcher_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type fakeTriggerRepo struct {
	items []*aggregate.Trigger
}

func (f *fakeTriggerRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}

func (f *fakeTriggerRepo) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
	return 0, nil
}

func (f *fakeTriggerRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *fakeTriggerRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	return nil
}

func (f *fakeTriggerRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *fakeTriggerRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *fakeTriggerRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	return f.items, nil
}

func (f *fakeTriggerRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ triggerdomain.TriggerRepository = (*fakeTriggerRepo)(nil)

func newServiceWithActions(items map[uint]string) *trigger.TriggerService {
	agg := make([]*aggregate.Trigger, 0, len(items))
	for id, action := range items {
		b, _ := aggregate.CreateTrigger(id, fmt.Sprintf("word-%d", id), action)
		agg = append(agg, b)
	}
	svc := trigger.NewTriggerService(&fakeTriggerRepo{items: agg}, nil, nil)
	_ = svc.Rebuild(context.Background())
	return svc
}

func TestTriggerService_DenyIDs_Mixed(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: enum.TriggerActionDeny,
		2: enum.TriggerActionOmit,
		3: enum.TriggerActionDeny,
	})
	deny := svc.DenyIDs([]uint{1, 2, 3})
	if len(deny) != 2 {
		t.Fatalf("expected 2 deny ids, got %v", deny)
	}
}

func TestTriggerService_DenyIDs_AllOmit(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: enum.TriggerActionOmit,
		2: enum.TriggerActionOmit,
	})
	deny := svc.DenyIDs([]uint{1, 2})
	if len(deny) != 0 {
		t.Fatalf("expected no deny ids, got %v", deny)
	}
}

func TestTriggerService_DenyIDs_EmptyActionDefaultsDeny(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: "",
		2: enum.TriggerActionOmit,
	})
	deny := svc.DenyIDs([]uint{1, 2})
	if len(deny) != 1 || deny[0] != 1 {
		t.Fatalf("expected empty action id 1 as deny, got %v", deny)
	}
}

func TestTriggerService_DenyIDs_EmptyInput(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{1: enum.TriggerActionDeny})
	deny := svc.DenyIDs(nil)
	if len(deny) != 0 {
		t.Fatalf("expected [], got %v", deny)
	}
}

func TestCreateTrigger_ActionDefaultsToDeny(t *testing.T) {
	t.Parallel()
	b, err := aggregate.CreateTrigger(1, "foo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Action() != enum.TriggerActionDeny {
		t.Fatalf("expected deny, got %q", b.Action())
	}
}

func TestCreateTrigger_ActionPreserved(t *testing.T) {
	t.Parallel()
	b, err := aggregate.CreateTrigger(1, "foo", enum.TriggerActionOmit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Action() != enum.TriggerActionOmit {
		t.Fatalf("expected omit, got %q", b.Action())
	}
}
