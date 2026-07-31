package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

// TestReportRejectsTraceDeleted 已软删 session 继续上报 → 全部 rejected 且不重建
func TestReportRejectsTraceDeleted(t *testing.T) {
	t.Parallel()

	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)

	ctx := context.Background()
	// 先建一条 trace，再软删
	created, err := repo.UpsertBySessionID(ctx, &trace.Trace{Agent: constant.TraceAgentCodex, SessionID: "s-deleted", APIKeyName: "k", UserID: 1})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	results, err := h.Handle(ctx, port.ReportTraceEventCommand{
		SessionID:  "s-deleted",
		Agent:      constant.TraceAgentCodex,
		APIKeyName: "k",
		UserID:     1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			Event: "UserPromptSubmit", DedupKey: "hook:deleted:1", Payload: []byte(`{"x":1}`),
		}},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(results) != 1 || results[0].Status != constant.TraceRecordStatusRejected || results[0].Message != constant.TraceRecordMessageTraceDeleted {
		t.Fatalf("expected rejected with trace deleted, got %+v", results)
	}
	// 不重建
	again, err := repo.FindBySessionIDIncludingDeleted(ctx, "s-deleted")
	if err != nil || again == nil || again.DeletedAt == 0 {
		t.Fatalf("trace should remain soft-deleted, got %+v err=%v", again, err)
	}
}
