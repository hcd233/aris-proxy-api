package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	domaintrace "github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestTraceQueries_EnforceOwnerIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewFakeRepo()
	traceRecord, err := repo.UpsertBySessionID(ctx, &domaintrace.Trace{
		SessionID: "s1", APIKeyName: "key2", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	apiKeys := newFakeAPIKeyRepo(map[uint][]string{1: {"key1"}})

	getHandler := query.NewGetTraceHandler(repo, apiKeys)
	if _, err := getHandler.Handle(ctx, port.GetTraceQuery{
		UserID: 1, TraceID: traceRecord.ID,
	}); err == nil {
		t.Fatal("non-owner accessed trace detail")
	}
	if _, err := getHandler.Handle(ctx, port.GetTraceQuery{
		UserID: 1, IsAdmin: true, TraceID: traceRecord.ID,
	}); err != nil {
		t.Fatalf("admin detail: %v", err)
	}

	eventsHandler := query.NewListTraceEventsHandler(repo, apiKeys)
	if _, _, err := eventsHandler.Handle(ctx, port.ListTraceEventsQuery{
		UserID: 1, TraceID: traceRecord.ID, Page: 1, PageSize: 20,
	}); err == nil {
		t.Fatal("non-owner accessed trace events")
	}

	conversationHandler := query.NewListTraceConversationHandler(repo, apiKeys)
	if _, err := conversationHandler.Handle(ctx, port.ListTraceConversationQuery{
		UserID: 1, TraceID: traceRecord.ID,
	}); err == nil {
		t.Fatal("non-owner accessed trace conversation")
	}
}
