package trace

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestParseRolloutRecord_FunctionCallKeepsCallIDAndArguments(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("./fixtures/rollout_records.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var lines []sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(data, &lines); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	record, err := trace.ParseRolloutRecord(lines[4])
	if err != nil {
		t.Fatalf("parse function call: %v", err)
	}
	if record.RecordType != "response_item" || record.Event != "function_call" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.TurnID != "t1" || record.CallID != "call-1" {
		t.Fatalf("unexpected identity: %+v", record)
	}
	if record.Arguments != `{"cmd":"pwd"}` {
		t.Fatalf("unexpected arguments: %s", record.Arguments)
	}
}

func TestParseRolloutRecord_UnknownTypeIsRetained(t *testing.T) {
	t.Parallel()

	record, err := trace.ParseRolloutRecord([]byte(`{"type":"future_record","payload":{"future_field":"kept"}}`))
	if err != nil {
		t.Fatalf("parse unknown record: %v", err)
	}
	if !record.Unknown || record.RecordType != "future_record" {
		t.Fatalf("unknown record not marked: %+v", record)
	}
	if len(record.Raw) == 0 {
		t.Fatal("unknown raw payload was lost")
	}
}
