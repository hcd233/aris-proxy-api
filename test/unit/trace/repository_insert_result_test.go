package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestFakeRepo_InsertEventReportsDuplicate(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	event := &trace.TraceEvent{SessionID: "s1", DedupKey: "hook:spool:1", Payload: []byte(`{}`)}

	inserted, err := repo.InsertEvent(context.Background(), event)
	if err != nil || !inserted {
		t.Fatalf("first insert = %v, %v", inserted, err)
	}
	inserted, err = repo.InsertEvent(context.Background(), event)
	if err != nil || inserted {
		t.Fatalf("duplicate insert = %v, %v", inserted, err)
	}
}

func TestFakeRepo_InsertEventDoesNotDeduplicateLegacyRecords(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()

	for range 2 {
		inserted, err := repo.InsertEvent(ctx, &trace.TraceEvent{SessionID: "legacy", Payload: []byte(`{"same":true}`)})
		if err != nil || !inserted {
			t.Fatalf("legacy insert = %v, %v", inserted, err)
		}
	}
	events, _, err := repo.ListEvents(ctx, 0, model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}})
	if err != nil || len(events) != 2 {
		t.Fatalf("legacy events = %d, %v", len(events), err)
	}
}
