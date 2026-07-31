package trace

import (
	"os"
	"path/filepath"
	"testing"

	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestParseHook_SubagentStop(t *testing.T) {
	t.Parallel()
	adapter, err := traceclient.LookupAdapter(constant.TraceAgentCodex)
	if err != nil {
		t.Fatalf("LookupAdapter: %v", err)
	}
	raw := []byte(`{
		"hook_event_name": "SubagentStop",
		"session_id": "parent-session-1",
		"agent_id": "019f6f10-7524-7891-96a2-fe7aa659430c",
		"agent_type": "general-purpose",
		"turn_id": "turn-42",
		"agent_transcript_path": "/home/u/.codex/sessions/rollout-2026-07-17T15-52-57-019f6f10-7524-7891-96a2-fe7aa659430c.jsonl"
	}`)
	info, err := adapter.ParseHook(raw)
	if err != nil {
		t.Fatalf("ParseHook: %v", err)
	}
	if info.SessionID != "parent-session-1" {
		t.Fatalf("SessionID = %q, want parent session", info.SessionID)
	}
	if info.EventName != constant.TraceEventSubagentStop {
		t.Fatalf("EventName = %q, want SubagentStop", info.EventName)
	}
	if info.AgentID != "019f6f10-7524-7891-96a2-fe7aa659430c" {
		t.Fatalf("AgentID = %q", info.AgentID)
	}
	if info.AgentType != "general-purpose" {
		t.Fatalf("AgentType = %q", info.AgentType)
	}
	if info.TurnID != "turn-42" {
		t.Fatalf("TurnID = %q", info.TurnID)
	}
	if info.AgentTranscriptPath != "/home/u/.codex/sessions/rollout-2026-07-17T15-52-57-019f6f10-7524-7891-96a2-fe7aa659430c.jsonl" {
		t.Fatalf("AgentTranscriptPath = %q", info.AgentTranscriptPath)
	}
}

func TestSubagentSessionIDFromPath_FileName(t *testing.T) {
	t.Parallel()
	got := traceclient.SubagentSessionIDFromPath("/home/u/.codex/sessions/2026/07/17/rollout-2026-07-17T15-52-57-019f6f10-7524-7891-96a2-fe7aa659430c.jsonl")
	want := "019f6f10-7524-7891-96a2-fe7aa659430c"
	if got != want {
		t.Fatalf("SubagentSessionIDFromPath = %q, want %q", got, want)
	}
}

func TestSubagentSessionIDFromPath_FallbackToSessionMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-unknown-name.jsonl")
	content := `{"timestamp":"2026-07-17T07:52:57.123Z","type":"session_meta","payload":{"id":"019f6f10-7524-7891-96a2-fe7aa659430c","session_id":"019f6f10-7524-7891-96a2-fe7aa659430c"}}
{"timestamp":"2026-07-17T07:52:58.000Z","type":"event_msg","payload":{"type":"task_started"}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got := traceclient.SubagentSessionIDFromPath(path)
	want := "019f6f10-7524-7891-96a2-fe7aa659430c"
	if got != want {
		t.Fatalf("fallback = %q, want %q", got, want)
	}
}
