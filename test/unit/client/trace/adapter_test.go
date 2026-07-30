package trace

import (
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestLookupAdapter_KnownAndUnknown(t *testing.T) {
	t.Parallel()
	if _, err := client.LookupAdapter(constant.TraceAgentCodex); err != nil {
		t.Fatalf("codex adapter must be registered: %v", err)
	}
	if _, err := client.LookupAdapter("nope"); err == nil {
		t.Fatal("unknown agent must return error")
	}
}

func TestCodexAdapter_ParseHook(t *testing.T) {
	t.Parallel()
	adapter := codexAdapter(t)
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"s1","turn_id":"t1","tool_use_id":"call-1","transcript_path":"/tmp/r.jsonl","model":"gpt-5","cwd":"/repo","source":"startup"}`)
	info, err := adapter.ParseHook(raw)
	if err != nil {
		t.Fatalf("parse hook: %v", err)
	}
	if info.EventName != "PreToolUse" || info.SessionID != "s1" || info.TurnID != "t1" ||
		info.CallID != "call-1" || info.TranscriptPath != "/tmp/r.jsonl" || info.Model != "gpt-5" {
		t.Fatalf("unexpected hook info: %+v", info)
	}
}

func TestCodexAdapter_StdoutAck(t *testing.T) {
	t.Parallel()
	adapter := codexAdapter(t)
	if got := adapter.StdoutAck(client.HookInfo{EventName: constant.TraceEventStop}); got != constant.EmptyJSONObject {
		t.Fatalf("Stop ack = %q, want {}", got)
	}
	if got := adapter.StdoutAck(client.HookInfo{EventName: constant.TraceEventSessionStart}); got != "" {
		t.Fatalf("SessionStart ack = %q, want empty", got)
	}
}

func TestCodexAdapter_ClassifyTranscriptLine(t *testing.T) {
	t.Parallel()
	adapter := codexAdapter(t)

	meta := adapter.ClassifyTranscriptLine([]byte(`{"timestamp":"2026-07-09T07:53:04.719Z","type":"response_item","payload":{"type":"function_call","call_id":"call-1","internal_chat_message_metadata_passthrough":{"turn_id":"t1"}}}`))
	if meta.RecordType != constant.TraceRecordTypeResponseItem || meta.Event != "function_call" ||
		meta.TurnID != "t1" || meta.CallID != "call-1" {
		t.Fatalf("unexpected meta: %+v", meta)
	}

	meta = adapter.ClassifyTranscriptLine([]byte(`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t2"}}`))
	if meta.RecordType != constant.TraceRecordTypeEventMsg || meta.Event != "task_complete" || meta.TurnID != "t2" {
		t.Fatalf("unexpected meta: %+v", meta)
	}

	meta = adapter.ClassifyTranscriptLine([]byte(`{"type":"future_type","payload":{"type":"x"}}`))
	if meta.RecordType != constant.TraceRolloutTypeUnknown {
		t.Fatalf("unknown envelope type must map to unknown record type, got %+v", meta)
	}
}
