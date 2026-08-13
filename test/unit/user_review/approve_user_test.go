package user_review

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestApproveUser_PendingToUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionPending))
	target := repo.users[0]
	handler := command.NewApproveUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil || updated == nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated.Permission() != enum.PermissionUser {
		t.Fatalf("expected permission user, got %s", updated.Permission())
	}
}

func TestApproveUser_RejectsNonPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "alice", "alice@example.com", enum.PermissionUser),
		newUser(t, "carol", "carol@example.com", enum.PermissionAdmin),
	)
	handler := command.NewApproveUserHandler(repo, nil)

	for _, u := range repo.users {
		if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: u.AggregateID()}); err == nil {
			t.Fatalf("expected error for user %s (perm %s), got nil", u.Name(), u.Permission())
		} else if !errors.Is(err, ierr.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	}
}

func TestApproveUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := command.NewApproveUserHandler(newFakeUserRepo(), nil)

	err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: 404})
	if err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}
