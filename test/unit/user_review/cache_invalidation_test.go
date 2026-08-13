package user_review

import (
	"context"
	"sync"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// recordInvalidations 记录被失效缓存的 userID（验证回调被调用）。
type recordInvalidations struct {
	mu  sync.Mutex
	ids []uint
}

func (r *recordInvalidations) invalidate(_ context.Context, userID uint) {
	r.mu.Lock()
	r.ids = append(r.ids, userID)
	r.mu.Unlock()
}

func (r *recordInvalidations) contains(userID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range r.ids {
		if id == userID {
			return true
		}
	}
	return false
}

// TestApproveUser_InvalidatesJWTUserCache 审批成功后必须失效目标用户缓存。
func TestApproveUser_InvalidatesJWTUserCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionPending))
	target := repo.users[0]
	rec := &recordInvalidations{}
	handler := command.NewApproveUserHandler(repo, rec.invalidate)

	if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if !rec.contains(target.AggregateID()) {
		t.Fatalf("expected jwt user cache invalidation for user %d", target.AggregateID())
	}
}

// TestApproveUser_RejectedDoesNotInvalidate 审批被拒（目标非 pending）时不得失效缓存。
func TestApproveUser_RejectedDoesNotInvalidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "alice", "alice@example.com", enum.PermissionUser))
	target := repo.users[0]
	rec := &recordInvalidations{}
	handler := command.NewApproveUserHandler(repo, rec.invalidate)

	if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err == nil {
		t.Fatalf("expected error for non-pending approve, got nil")
	}
	if rec.contains(target.AggregateID()) {
		t.Fatalf("must not invalidate cache when approve rejected")
	}
}

// TestDemoteUser_InvalidatesJWTUserCache 降级成功后必须失效目标用户缓存。
func TestDemoteUser_InvalidatesJWTUserCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionUser))
	target := repo.users[0]
	rec := &recordInvalidations{}
	handler := command.NewDemoteUserHandler(repo, rec.invalidate)

	if err := handler.Handle(ctx, port.DemoteUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("demote failed: %v", err)
	}
	if !rec.contains(target.AggregateID()) {
		t.Fatalf("expected jwt user cache invalidation for user %d", target.AggregateID())
	}
}

// TestDeleteUser_InvalidatesJWTUserCache 删除成功后必须失效目标用户缓存。
func TestDeleteUser_InvalidatesJWTUserCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionUser))
	target := repo.users[0]
	rec := &recordInvalidations{}
	handler := command.NewDeleteUserHandler(repo, rec.invalidate)

	if err := handler.Handle(ctx, port.DeleteUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !rec.contains(target.AggregateID()) {
		t.Fatalf("expected jwt user cache invalidation for user %d", target.AggregateID())
	}
}
