package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func mustUpsert(t *testing.T, repo *FakeRepo, sessionID, owner string) uint {
	t.Helper()
	tr, err := repo.UpsertBySessionID(context.Background(), &trace.Trace{
		Agent: constant.TraceAgentCodex, SessionID: sessionID, APIKeyName: owner,
	})
	if err != nil {
		t.Fatalf("upsert %s: %v", sessionID, err)
	}
	return tr.ID
}

// TestDeleteOwnerCanDeleteOwn 普通用户可删自己 API Key 名下的 trace
func TestDeleteOwnerCanDeleteOwn(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-own", "key-a")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: false, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 || len(result.Failures) != 0 {
		t.Fatalf("expected 1 deleted 0 failures, got %+v", result)
	}
	tr, _ := repo.FindBySessionIDIncludingDeleted(context.Background(), "s-own")
	if tr == nil || tr.DeletedAt == 0 {
		t.Fatal("trace should be soft deleted")
	}
}

// TestDeleteOwnerCannotDeleteOthers 普通用户不能删他人 trace
func TestDeleteOwnerCannotDeleteOthers(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-other", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: false, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 0 || len(result.Failures) != 1 || result.Failures[0].Error != constant.TraceDeleteErrorNoPermission {
		t.Fatalf("expected no-permission failure, got %+v", result)
	}
}

// TestDeleteAdminCanDeleteAny admin 可删任意 trace
func TestDeleteAdminCanDeleteAny(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-any", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: true, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 {
		t.Fatalf("expected 1 deleted, got %+v", result)
	}
}

// TestDeleteNotFound 不存在的 trace → NotFound 失败项
func TestDeleteNotFound(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: true, IDs: []uint{999}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(result.Failures) != 1 || result.Failures[0].Error != constant.TraceDeleteErrorNotFound {
		t.Fatalf("expected not-found failure, got %+v", result)
	}
}

// TestDeleteBatchMixed 批量混合成功/失败
func TestDeleteBatchMixed(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	okID := mustUpsert(t, repo, "s-ok", "key-a")
	otherID := mustUpsert(t, repo, "s-no", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{
		UserID: 1, IsAdmin: false, IDs: []uint{okID, otherID, 999},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 || len(result.Failures) != 2 {
		t.Fatalf("expected 1 deleted 2 failures, got %+v", result)
	}
}
