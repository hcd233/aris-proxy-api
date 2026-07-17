package trace

import (
	"bytes"
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

func TestFakeRepo_Events_PreserveRecordIdentity(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()

	tr, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "s1", APIKeyName: "key1", Status: "active"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	first := &trace.TraceEvent{
		TraceID:        tr.ID,
		SessionID:      "s1",
		Source:         constant.TraceRecordSourceHook,
		RecordType:     constant.TraceRecordTypeHookEvent,
		Event:          constant.TraceEventPostToolUse,
		TurnID:         "turn-1",
		CallID:         "call-1",
		ClientSequence: 1,
		DedupKey:       "hook:s1:1",
		Payload:        []byte(`{"unknown":{"value":"kept"}}`),
	}
	second := &trace.TraceEvent{
		TraceID:        tr.ID,
		SessionID:      "s1",
		Source:         constant.TraceRecordSourceHook,
		RecordType:     constant.TraceRecordTypeHookEvent,
		Event:          constant.TraceEventPostToolUse,
		TurnID:         "turn-1",
		CallID:         "call-2",
		ClientSequence: 2,
		DedupKey:       "hook:s1:2",
		Payload:        []byte(`{"unknown":{"value":"kept-2"}}`),
	}
	if err := repo.InsertEvent(ctx, first); err != nil {
		t.Fatalf("insert first: %v", err)
	}
	if err := repo.InsertEvent(ctx, second); err != nil {
		t.Fatalf("insert second: %v", err)
	}
	duplicate := &trace.TraceEvent{
		TraceID:   tr.ID,
		SessionID: "s1",
		Event:     constant.TraceEventPostToolUse,
		DedupKey:  first.DedupKey,
		Payload:   []byte(`{"duplicate":true}`),
	}
	if err := repo.InsertEvent(ctx, duplicate); err != nil {
		t.Fatalf("insert duplicate: %v", err)
	}

	events, _, err := repo.ListEvents(ctx, tr.ID, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 unique events, got %d", len(events))
	}
	if events[0].CallID != "call-1" || events[1].CallID != "call-2" {
		t.Fatalf("same event name collapsed records: %+v", events)
	}
	if events[0].Source != first.Source || events[0].RecordType != first.RecordType || events[0].DedupKey != first.DedupKey {
		t.Fatalf("record identity not preserved: %+v", events[0])
	}
	if !bytes.Equal(events[0].Payload, first.Payload) {
		t.Fatalf("raw payload changed: %s", events[0].Payload)
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
