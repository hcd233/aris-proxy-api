package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestReportTraceEvent_BatchPersistsAllRecordsAndDeduplicates(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	records := []port.ReportTraceRecord{
		{Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, HookEventName: "SessionStart", ClientSequence: 1, DedupKey: "hook:s1:1", Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"s1"}`)},
		{Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, HookEventName: "UserPromptSubmit", TurnID: "t1", ClientSequence: 2, DedupKey: "hook:s1:2", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s1","turn_id":"t1"}`)},
		{Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, HookEventName: "PreToolUse", TurnID: "t1", CallID: "call-1", ClientSequence: 3, DedupKey: "hook:s1:3", Payload: []byte(`{"hook_event_name":"PreToolUse","session_id":"s1","turn_id":"t1","tool_use_id":"call-1"}`)},
		{Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, HookEventName: "PostToolUse", TurnID: "t1", CallID: "call-1", ClientSequence: 4, DedupKey: "hook:s1:4", Payload: []byte(`{"hook_event_name":"PostToolUse","session_id":"s1","turn_id":"t1","tool_use_id":"call-1"}`)},
		{Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, HookEventName: "Stop", TurnID: "t1", ClientSequence: 5, DedupKey: "hook:s1:5", Payload: []byte(`{"hook_event_name":"Stop","session_id":"s1","turn_id":"t1"}`)},
		{Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: "function_call", TurnID: "t1", CallID: "call-1", ClientSequence: 6, DedupKey: "rollout:s1:6", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call","call_id":"call-1"}}`)},
		{Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: "function_call_output", TurnID: "t1", CallID: "call-1", ClientSequence: 7, DedupKey: "rollout:s1:7", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call_output","call_id":"call-1"}}`)},
		{Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg, Event: "task_complete", TurnID: "t1", ClientSequence: 8, DedupKey: "rollout:s1:8", Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1"}}`)},
	}
	cmd := port.ReportTraceEventCommand{SessionID: "s1", Model: "gpt-4o", CWD: "/work", APIKeyName: "key1", UserID: 1, Records: records}
	if _, err := handler.Handle(ctx, cmd); err != nil {
		t.Fatalf("first batch failed: %v", err)
	}
	if _, err := handler.Handle(ctx, cmd); err != nil {
		t.Fatalf("duplicate batch failed: %v", err)
	}

	tr, _ := repo.FindBySessionID(ctx, "s1")
	if tr == nil || tr.Status != constant.TraceStatusDone {
		t.Fatalf("expected done trace, got %+v", tr)
	}
	if n, _ := repo.CountEvents(ctx, tr.ID); n != int64(len(records)) {
		t.Fatalf("expected %d events, got %d", len(records), n)
	}
	events, _, _ := repo.ListEvents(ctx, tr.ID, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if events[2].CallID != "call-1" || events[5].CallID != "call-1" {
		t.Fatalf("call identity not preserved: %+v", events)
	}
}

func TestReportTraceEvent_SessionStartThenStop(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	start := []byte(`{"hook_event_name":"SessionStart","session_id":"s1","model":"gpt-4o","source":"startup","cwd":"/work"}`)
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		HookEventName: "SessionStart", SessionID: "s1", Model: "gpt-4o", Source: "startup", CWD: "/work",
		RawPayload: start, APIKeyName: "key1", UserID: 1,
	}); err != nil {
		t.Fatalf("SessionStart failed: %v", err)
	}
	stop := []byte(`{"hook_event_name":"Stop","session_id":"s1"}`)
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		HookEventName: "Stop", SessionID: "s1", RawPayload: stop, APIKeyName: "key1", UserID: 1,
	}); err != nil {
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
	if n, _ := repo.CountEvents(ctx, tr.ID); n != 2 {
		t.Fatalf("expected 2 events (SessionStart and Stop), got %d", n)
	}
	events, _, _ := repo.ListEvents(ctx, tr.ID, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if events[0].Event != "SessionStart" || events[1].Event != "Stop" {
		t.Fatalf("expected SessionStart and Stop events, got %+v", events)
	}
}

func TestReportTraceEvent_MissingSessionID(t *testing.T) {
	t.Parallel()
	handler := command.NewReportTraceEventHandler(NewFakeRepo())
	_, err := handler.Handle(context.Background(), port.ReportTraceEventCommand{
		HookEventName: "SessionStart",
		APIKeyName:    "key1",
		UserID:        1,
	})
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestReportTraceEvent_CreatesTraceOnFirstEvent(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	// First event for an unknown session still creates an owned trace.
	payload := []byte(`{"hook_event_name":"PreToolUse","session_id":"u1","turn_id":"t1"}`)
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		HookEventName: "PreToolUse", SessionID: "u1", TurnID: "t1",
		RawPayload: payload, APIKeyName: "key1", UserID: 1,
	}); err != nil {
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
	repo := NewFakeRepo()
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
