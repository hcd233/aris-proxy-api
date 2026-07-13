package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestReportTraceEvent_SessionStartThenStop(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	start := []byte(`{"hook_event_name":"SessionStart","session_id":"s1","model":"gpt-4o","source":"startup","cwd":"/work"}`)
	if err := handler.Handle(ctx, port.ReportTraceEventCommand{RawPayload: start, APIKeyName: "key1", UserID: 1}); err != nil {
		t.Fatalf("SessionStart failed: %v", err)
	}
	stop := []byte(`{"hook_event_name":"Stop","session_id":"s1"}`)
	if err := handler.Handle(ctx, port.ReportTraceEventCommand{RawPayload: stop, APIKeyName: "key1", UserID: 1}); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	tr, _ := repo.FindBySessionID(ctx, "s1")
	if tr == nil {
		t.Fatal("trace not created")
	}
	if tr.Status != "done" {
		t.Fatalf("expected done, got %s", tr.Status)
	}
	if tr.APIKeyName != "key1" || tr.Model != "gpt-4o" || tr.CWD != "/work" {
		t.Fatalf("unexpected trace fields: %+v", tr)
	}
	if n, _ := repo.CountEvents(ctx, tr.ID); n != 1 {
		t.Fatalf("expected 1 event (Stop), got %d", n)
	}
	events, _, _ := repo.ListEvents(ctx, tr.ID, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if events[0].Event != "Stop" {
		t.Fatalf("expected Stop event, got %s", events[0].Event)
	}
}

func TestReportTraceEvent_MissingSessionID(t *testing.T) {
	t.Parallel()
	handler := command.NewReportTraceEventHandler(newFakeRepo())
	err := handler.Handle(context.Background(), port.ReportTraceEventCommand{
		RawPayload: []byte(`{"hook_event_name":"SessionStart"}`),
		APIKeyName: "key1",
		UserID:     1,
	})
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestReportTraceEvent_CreatesTraceOnFirstEvent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	// First event for an unknown session still creates an owned trace.
	payload := []byte(`{"hook_event_name":"PreToolUse","session_id":"u1","turn_id":"t1"}`)
	if err := handler.Handle(ctx, port.ReportTraceEventCommand{RawPayload: payload, APIKeyName: "key1", UserID: 1}); err != nil {
		t.Fatalf("PreToolUse failed: %v", err)
	}
	tr, _ := repo.FindBySessionID(ctx, "u1")
	if tr == nil {
		t.Fatal("trace should be auto-created on first event")
	}
	if tr.APIKeyName != "key1" {
		t.Fatalf("expected trace owned by key1, got %s", tr.APIKeyName)
	}
	if n, _ := repo.CountEvents(ctx, tr.ID); n != 1 {
		t.Fatalf("expected 1 event, got %d", n)
	}
}

func TestListTraces_OwnerIsolation(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	ctx := context.Background()
	repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1", Status: "active"})
	repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s2", APIKeyName: "key2", Status: "active"})

	listHandler := query.NewListTracesHandler(repo, newFakeAPIKeyRepo(map[uint][]string{
		1: {"key1"},
	}))

	userViews, _, err := listHandler.Handle(ctx, port.ListTracesQuery{UserID: 1, IsAdmin: false, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list user: %v", err)
	}
	if len(userViews) != 1 || userViews[0].SessionID != "s1" {
		t.Fatalf("expected only s1 (key1) for user1, got %+v", userViews)
	}

	adminViews, _, err := listHandler.Handle(ctx, port.ListTracesQuery{UserID: 1, IsAdmin: true, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list admin: %v", err)
	}
	if len(adminViews) != 2 {
		t.Fatalf("expected 2 traces for admin, got %d", len(adminViews))
	}
}
