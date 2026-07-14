package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestFakeRepo_PaginateByOwners_Isolation(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()

	if _, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1", Status: "active"}); err != nil {
		t.Fatalf("upsert s1: %v", err)
	}
	if _, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s2", APIKeyName: "key2", Status: "active"}); err != nil {
		t.Fatalf("upsert s2: %v", err)
	}

	// user1 owns only key1
	userTraces, _, err := repo.PaginateByOwners(ctx, []string{"key1"}, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}})
	if err != nil {
		t.Fatalf("paginate user: %v", err)
	}
	if len(userTraces) != 1 || userTraces[0].SessionID != "s1" {
		t.Fatalf("expected only s1 for user, got %d traces", len(userTraces))
	}

	// admin (empty owners) sees all
	adminTraces, _, err := repo.PaginateByOwners(ctx, []string{}, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}})
	if err != nil {
		t.Fatalf("paginate admin: %v", err)
	}
	if len(adminTraces) != 2 {
		t.Fatalf("expected 2 traces for admin, got %d", len(adminTraces))
	}
}

func TestFakeRepo_Events(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()

	tr, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1", Status: "active"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.InsertEvent(ctx, &trace.TraceEvent{SessionID: "s1", Event: constant.TraceEventSessionStart, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.InsertEvent(ctx, &trace.TraceEvent{SessionID: "s1", Event: constant.TraceEventStop, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("insert stop: %v", err)
	}

	if n, _ := repo.CountEvents(ctx, tr.ID); n != 2 {
		t.Fatalf("expected 2 events, got %d", n)
	}
	events, _, err := repo.ListEvents(ctx, tr.ID, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 || events[1].Event != constant.TraceEventStop {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestFakeRepo_MarkDone(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()
	if _, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1", Status: "active"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.MarkDone(ctx, "s1"); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	tr, _ := repo.FindBySessionID(ctx, "s1")
	if tr.Status != "done" {
		t.Fatalf("expected done, got %s", tr.Status)
	}
}
