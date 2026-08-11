package blocked_matcher_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type fakeBlockedRepo struct {
	items []*aggregate.Blocked
}

func (f *fakeBlockedRepo) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) {
	return nil, nil
}

func (f *fakeBlockedRepo) Create(ctx context.Context, word *aggregate.Blocked) (uint, error) {
	return 0, nil
}

func (f *fakeBlockedRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *fakeBlockedRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *fakeBlockedRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *fakeBlockedRepo) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	return f.items, nil
}

func (f *fakeBlockedRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ blockeddomain.BlockedRepository = (*fakeBlockedRepo)(nil)

func newServiceWithActions(items map[uint]string) *blocked.BlockedService {
	agg := make([]*aggregate.Blocked, 0, len(items))
	for id, action := range items {
		b, _ := aggregate.CreateBlocked(id, fmt.Sprintf("word-%d", id), action)
		agg = append(agg, b)
	}
	svc := blocked.NewBlockedService(&fakeBlockedRepo{items: agg}, nil)
	svc.Rebuild(context.Background())
	return svc
}

func TestBlockedService_DenyIDs_Mixed(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: enum.BlockedActionDeny,
		2: enum.BlockedActionAllow,
		3: enum.BlockedActionDeny,
	})
	deny := svc.DenyIDs([]uint{1, 2, 3})
	if len(deny) != 2 {
		t.Fatalf("expected 2 deny ids, got %v", deny)
	}
}

func TestBlockedService_DenyIDs_AllAllow(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: enum.BlockedActionAllow,
		2: enum.BlockedActionAllow,
	})
	deny := svc.DenyIDs([]uint{1, 2})
	if len(deny) != 0 {
		t.Fatalf("expected no deny ids, got %v", deny)
	}
}

func TestBlockedService_DenyIDs_EmptyActionDefaultsDeny(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{
		1: "",
		2: enum.BlockedActionAllow,
	})
	deny := svc.DenyIDs([]uint{1, 2})
	if len(deny) != 1 || deny[0] != 1 {
		t.Fatalf("expected empty action id 1 as deny, got %v", deny)
	}
}

func TestBlockedService_DenyIDs_EmptyInput(t *testing.T) {
	t.Parallel()
	svc := newServiceWithActions(map[uint]string{1: enum.BlockedActionDeny})
	deny := svc.DenyIDs(nil)
	if len(deny) != 0 {
		t.Fatalf("expected [], got %v", deny)
	}
}

func TestCreateBlocked_ActionDefaultsToDeny(t *testing.T) {
	t.Parallel()
	b, err := aggregate.CreateBlocked(1, "foo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Action() != enum.BlockedActionDeny {
		t.Fatalf("expected deny, got %q", b.Action())
	}
}

func TestCreateBlocked_ActionPreserved(t *testing.T) {
	t.Parallel()
	b, err := aggregate.CreateBlocked(1, "foo", enum.BlockedActionAllow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Action() != enum.BlockedActionAllow {
		t.Fatalf("expected allow, got %q", b.Action())
	}
}
