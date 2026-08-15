// Package user_review 用户审核闭环的单元测试 —— Demo 账户设置/恢复
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

func TestSetDemoUser_PendingToDemo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "alice", "alice@example.com", enum.PermissionPending))
	target := repo.users[0]
	handler := command.NewSetDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("set demo failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil || updated == nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated.Permission() != enum.PermissionDemo {
		t.Fatalf("expected permission demo, got %s", updated.Permission())
	}
}

func TestSetDemoUser_ReplacesExistingDemo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "old-demo", "old@example.com", enum.PermissionDemo),
		newUser(t, "bob", "bob@example.com", enum.PermissionUser),
	)
	oldDemo, bob := repo.users[0], repo.users[1]
	handler := command.NewSetDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: 99, UserID: bob.AggregateID()}); err != nil {
		t.Fatalf("set demo failed: %v", err)
	}
	demo, err := repo.FindByPermission(ctx, enum.PermissionDemo)
	if err != nil || demo == nil {
		t.Fatalf("find demo user failed: %v", err)
	}
	if demo.AggregateID() != bob.AggregateID() {
		t.Fatalf("expected new demo to be bob, got id %d", demo.AggregateID())
	}
	oldUpdated, err := repo.FindByID(ctx, oldDemo.AggregateID())
	if err != nil || oldUpdated == nil {
		t.Fatalf("find old demo failed: %v", err)
	}
	if oldUpdated.Permission() != enum.PermissionPending {
		t.Fatalf("expected previous demo to become pending, got %s", oldUpdated.Permission())
	}
}

func TestSetDemoUser_RejectsAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "root", "root@example.com", enum.PermissionAdmin))
	target := repo.users[0]
	handler := command.NewSetDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: 99, UserID: target.AggregateID()}); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation for admin target, got %v", err)
	}
}

func TestSetDemoUser_RejectsSelf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "self", "self@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewSetDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: target.AggregateID(), UserID: target.AggregateID()}); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation for self target, got %v", err)
	}
}

func TestSetDemoUser_RejectsAlreadyDemo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "demo", "demo@example.com", enum.PermissionDemo))
	target := repo.users[0]
	handler := command.NewSetDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: 99, UserID: target.AggregateID()}); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation for demo target, got %v", err)
	}
}

func TestSetDemoUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := command.NewSetDemoUserHandler(newFakeUserRepo(), nil)

	if err := handler.Handle(ctx, port.SetDemoUserCommand{OperatorID: 99, UserID: 1}); !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}

func TestRestoreDemoUser_DemoToUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "demo", "demo@example.com", enum.PermissionDemo))
	target := repo.users[0]
	handler := command.NewRestoreDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.RestoreDemoUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("restore demo failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil || updated == nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated.Permission() != enum.PermissionUser {
		t.Fatalf("expected permission user, got %s", updated.Permission())
	}
}

func TestRestoreDemoUser_RejectsNonDemo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionUser))
	target := repo.users[0]
	handler := command.NewRestoreDemoUserHandler(repo, nil)

	if err := handler.Handle(ctx, port.RestoreDemoUserCommand{OperatorID: 99, UserID: target.AggregateID()}); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation for non-demo target, got %v", err)
	}
}
