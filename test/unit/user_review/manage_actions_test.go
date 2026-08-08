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

func TestDemoteUser_UserToPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewDemoteUserHandler(repo)

	if err := handler.Handle(ctx, port.DemoteUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("demote failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil || updated == nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated.Permission() != enum.PermissionPending {
		t.Fatalf("expected permission pending, got %s", updated.Permission())
	}
}

func TestDemoteUser_RejectsNonUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "alice", "alice@example.com", enum.PermissionPending),
		newUser(t, "carol", "carol@example.com", enum.PermissionAdmin),
	)
	handler := command.NewDemoteUserHandler(repo)

	for _, u := range repo.users {
		if err := handler.Handle(ctx, port.DemoteUserCommand{OperatorID: 99, UserID: u.AggregateID()}); err == nil {
			t.Fatalf("expected error for user %s (perm %s), got nil", u.Name(), u.Permission())
		} else if !errors.Is(err, ierr.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	}
}

func TestDemoteUser_RejectsSelf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "dave", "dave@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewDemoteUserHandler(repo)

	if err := handler.Handle(ctx, port.DemoteUserCommand{OperatorID: target.AggregateID(), UserID: target.AggregateID()}); err == nil {
		t.Fatalf("expected error for self-demote, got nil")
	} else if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDemoteUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := command.NewDemoteUserHandler(newFakeUserRepo())

	err := handler.Handle(ctx, port.DemoteUserCommand{OperatorID: 99, UserID: 404})
	if err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}

func TestDeleteUser_SoftDeletesUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewDeleteUserHandler(repo)

	if err := handler.Handle(ctx, port.DeleteUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated != nil {
		t.Fatalf("expected user to be removed, still present")
	}
}

func TestDeleteUser_RejectsAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "carol", "carol@example.com", enum.PermissionAdmin))
	target := repo.users[0]
	handler := command.NewDeleteUserHandler(repo)

	if err := handler.Handle(ctx, port.DeleteUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err == nil {
		t.Fatalf("expected error for deleting admin, got nil")
	} else if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDeleteUser_RejectsSelf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "dave", "dave@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewDeleteUserHandler(repo)

	if err := handler.Handle(ctx, port.DeleteUserCommand{OperatorID: target.AggregateID(), UserID: target.AggregateID()}); err == nil {
		t.Fatalf("expected error for self-delete, got nil")
	} else if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDeleteUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := command.NewDeleteUserHandler(newFakeUserRepo())

	err := handler.Handle(ctx, port.DeleteUserCommand{OperatorID: 99, UserID: 404})
	if err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}
