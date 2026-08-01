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
	if tr == nil {
		t.Fatalf("expected trace, got nil")
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
		SessionID: "s1", Model: "gpt-4o", Source: "startup", CWD: "/work",
		APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:s1:start", Payload: start,
		}},
	}); err != nil {
		t.Fatalf("SessionStart failed: %v", err)
	}
	stop := []byte(`{"hook_event_name":"Stop","session_id":"s1"}`)
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "s1", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "Stop", DedupKey: "hook:s1:stop", Payload: stop,
		}},
	}); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	tr, _ := repo.FindBySessionID(ctx, "s1")
	if tr == nil {
		t.Fatal("trace not created")
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
		APIKeyName: "key1",
		UserID:     1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:x:1", Payload: []byte(`{"hook_event_name":"SessionStart"}`),
		}},
	})
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestReportTraceEvent_EmptyRecords(t *testing.T) {
	t.Parallel()
	handler := command.NewReportTraceEventHandler(NewFakeRepo())
	_, err := handler.Handle(context.Background(), port.ReportTraceEventCommand{
		SessionID: "s1", APIKeyName: "key1", UserID: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty records")
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
		SessionID: "u1", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "PreToolUse", TurnID: "t1", DedupKey: "hook:u1:1", Payload: payload,
		}},
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

func TestReportTraceEvent_AgentDefaultsToCodex(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)
	_, err := h.Handle(context.Background(), port.ReportTraceEventCommand{
		SessionID: "s-agent-default",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:x:1",
			Payload: []byte(`{"session_id":"s-agent-default"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "s-agent-default")
	if tr == nil || tr.Agent != constant.TraceAgentCodex {
		t.Fatalf("agent must default to codex, got %+v", tr)
	}
}

func TestReportTraceEvent_RejectsUnknownAgent(t *testing.T) {
	t.Parallel()
	h := command.NewReportTraceEventHandler(NewFakeRepo())
	_, err := h.Handle(context.Background(), port.ReportTraceEventCommand{
		SessionID: "s-bad-agent",
		Agent:     "gemini",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:x:2",
			Payload: []byte(`{"session_id":"s-bad-agent"}`),
		}},
	})
	if err == nil {
		t.Fatal("unknown agent must be rejected")
	}
}

func TestReportTraceEvent_RejectsAgentMismatch(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)
	first := port.ReportTraceEventCommand{
		SessionID: "s-mismatch", Agent: constant.TraceAgentCodex,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:m:1", Payload: []byte(`{"session_id":"s-mismatch"}`),
		}},
	}
	if _, err := h.Handle(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Agent = constant.TraceAgentClaude
	second.Records = []port.ReportTraceRecord{{
		Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
		HookEventName: "UserPromptSubmit", DedupKey: "hook:m:2", Payload: []byte(`{"session_id":"s-mismatch"}`),
	}}
	if _, err := h.Handle(context.Background(), second); err == nil {
		t.Fatal("agent mismatch on existing session must be rejected")
	}
}

func TestListTraces_OwnerIsolation(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()
	repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1"})
	repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s2", APIKeyName: "key2"})

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

func TestReportTraceEvent_SubagentCommandCarriesParentSession(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "parent-s1", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:p1:1",
			Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"parent-s1"}`),
		}},
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	// 子代理批次：SessionID 为子代理 id，ParentSessionID 指向父
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "child-s1", ParentSessionID: "parent-s1", AgentType: "worker",
		APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", TurnID: "t1", DedupKey: "rollout:child-s1:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("report subagent batch: %v", err)
	}

	child, _ := repo.FindBySessionID(ctx, "child-s1")
	if child == nil || child.ParentTraceID == 0 {
		t.Fatalf("expected child trace linked to parent, got %+v", child)
	}
	parent, _ := repo.FindBySessionID(ctx, "parent-s1")
	if parent == nil || child.ParentTraceID != parent.ID {
		t.Fatalf("expected child.ParentTraceID=%d (parent id), got %d", parent.ID, child.ParentTraceID)
	}
}

func TestReportTraceEvent_SubagentChildMetadataAndDone(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "parent-s2", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:p2:1",
			Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"parent-s2"}`),
		}},
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "child-s2", ParentSessionID: "parent-s2", AgentID: "agent-1", AgentType: "worker",
		APIKeyName: "key1", UserID: 1, Model: "gpt-5", CWD: "/work",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", TurnID: "t1", DedupKey: "rollout:child-s2:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("report subagent: %v", err)
	}

	child, _ := repo.FindBySessionID(ctx, "child-s2")
	if child == nil {
		t.Fatal("child trace missing")
	}
	if child.Source != "subagent" {
		t.Fatalf("expected child Source=subagent, got %q", child.Source)
	}
	if child.Metadata["agent_type"] != "worker" || child.Metadata["agent_id"] != "agent-1" {
		t.Fatalf("unexpected child metadata: %+v", child.Metadata)
	}
	if child.Model != "gpt-5" || child.CWD != "/work" {
		t.Fatalf("unexpected child model/cwd: %+v", child)
	}
}

func TestReportTraceEvent_SubagentMissingParentIsTolerant(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "orphan-child", ParentSessionID: "no-such-parent", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", DedupKey: "rollout:orphan:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("orphan subagent should not error: %v", err)
	}
	child, _ := repo.FindBySessionID(ctx, "orphan-child")
	if child == nil || child.ParentTraceID != 0 {
		t.Fatalf("expected orphan child with ParentTraceID=0, got %+v", child)
	}
}

func TestReportTraceEvent_SubagentCrossTenantParentNotLinked(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "parent-tenant-a", APIKeyName: "key-a", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:pa:1",
			Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"parent-tenant-a"}`),
		}},
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "child-tenant-b", ParentSessionID: "parent-tenant-a", APIKeyName: "key-b", UserID: 2,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", DedupKey: "rollout:cb:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("report subagent: %v", err)
	}
	child, _ := repo.FindBySessionID(ctx, "child-tenant-b")
	if child == nil {
		t.Fatal("child trace missing")
	}
	if child.ParentTraceID != 0 {
		t.Fatalf("cross-tenant child must not link parent, got ParentTraceID=%d", child.ParentTraceID)
	}
}
