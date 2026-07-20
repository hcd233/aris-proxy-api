package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestReportTraceEvent_ReturnsPerRecordResults(t *testing.T) {
	t.Parallel()
	handler := command.NewReportTraceEventHandler(NewFakeRepo())
	cmd := port.ReportTraceEventCommand{
		SessionID:  "s1",
		APIKeyName: "key1",
		UserID:     1,
		Records: []port.ReportTraceRecord{{
			Source:        constant.TraceRecordSourceHook,
			RecordType:    constant.TraceRecordTypeHookEvent,
			HookEventName: constant.TraceEventSessionStart,
			DedupKey:      "hook:spool:1",
			Payload:       []byte(`{"session_id":"s1","hook_event_name":"SessionStart"}`),
		}},
	}

	results, err := handler.Handle(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != constant.TraceRecordStatusAccepted {
		t.Fatalf("first results = %+v", results)
	}
	results, err = handler.Handle(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != constant.TraceRecordStatusDuplicate {
		t.Fatalf("duplicate results = %+v", results)
	}
}
