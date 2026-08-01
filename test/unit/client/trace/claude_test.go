package trace

import (
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func claudeAdapter(t *testing.T) client.AgentAdapter {
	t.Helper()
	adapter, err := client.LookupAdapter(constant.TraceAgentClaude)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestClaudeAdapter_ParseHook(t *testing.T) {
	t.Parallel()
	adapter := claudeAdapter(t)
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"abc123","prompt_id":"p-1","tool_use_id":"toolu_01","transcript_path":"/home/u/.claude/projects/-x/abc123.jsonl","cwd":"/repo"}`)
	info, err := adapter.ParseHook(raw)
	if err != nil {
		t.Fatalf("parse hook: %v", err)
	}
	if info.EventName != "PreToolUse" || info.SessionID != "abc123" ||
		info.TurnID != "p-1" || info.CallID != "toolu_01" ||
		info.TranscriptPath != "/home/u/.claude/projects/-x/abc123.jsonl" || info.CWD != "/repo" {
		t.Fatalf("unexpected hook info: %+v", info)
	}

	sessionStart := []byte(`{"hook_event_name":"SessionStart","session_id":"abc123","source":"startup","model":"claude-sonnet-5"}`)
	info, err = adapter.ParseHook(sessionStart)
	if err != nil {
		t.Fatalf("parse session start: %v", err)
	}
	if info.Model != "claude-sonnet-5" {
		t.Fatalf("unexpected session start info: %+v", info)
	}
}

func TestClaudeAdapter_StdoutAckAlwaysSilent(t *testing.T) {
	t.Parallel()
	adapter := claudeAdapter(t)
	for _, event := range []string{constant.TraceEventStop, constant.TraceEventSessionStart, "UserPromptSubmit", constant.TraceEventSessionEnd} {
		if got := adapter.StdoutAck(client.HookInfo{EventName: event}); got != "" {
			t.Fatalf("claude %s ack = %q, must always be empty", event, got)
		}
	}
}

func TestClaudeAdapter_ClassifyTranscriptLine(t *testing.T) {
	t.Parallel()
	adapter := claudeAdapter(t)

	cases := []struct {
		name       string
		raw        string
		recordType string
		event      string
		callID     string
	}{
		{"user prompt", `{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"你好"}}`, constant.TraceClaudeRecordUser, constant.TraceClaudeEventUserPrompt, ""},
		{"tool result", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01","content":"ok"}]}}`, constant.TraceClaudeRecordUser, constant.TraceClaudeEventToolResult, "toolu_01"},
		{"assistant", `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"..."},{"type":"tool_use","id":"toolu_02","name":"Bash","input":{"command":"ls"}}]}}`, constant.TraceClaudeRecordAssistant, constant.TraceClaudeEventAssistantMessage, "toolu_02"},
		{"attachment", `{"type":"attachment","attachment":{"type":"skill_listing","content":"..."}}`, constant.TraceClaudeRecordAttachment, constant.TraceClaudeRecordAttachment, ""},
		{"system subtype", `{"type":"system","subtype":"turn_duration","durationMs":100}`, constant.TraceClaudeRecordSystem, "turn_duration", ""},
		{"permission mode passthrough", `{"type":"permission-mode","permissionMode":"default"}`, "permission-mode", "permission-mode", ""},
		{"unknown", `{"type":"future_record","x":1}`, constant.TraceRolloutTypeUnknown, "future_record", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta := adapter.ClassifyTranscriptLine([]byte(tc.raw))
			if meta.RecordType != tc.recordType || meta.Event != tc.event || meta.CallID != tc.callID {
				t.Fatalf("got %+v, want type=%s event=%s call=%s", meta, tc.recordType, tc.event, tc.callID)
			}
		})
	}
}
