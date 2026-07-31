package trace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestTrimStopHookPayload_RemovesLastAssistantMessage(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"hook_event_name": "Stop",
		"session_id": "sess-1",
		"model": "gpt-5",
		"cwd": "/work",
		"last_assistant_message": {"role": "assistant", "content": "big blob"},
		"turn_id": "t1"
	}`)
	out := traceclient.TrimStopHookPayload(raw)
	if bytes.Equal(out, raw) {
		t.Fatal("payload should be modified")
	}
	var root map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(out, &root); err != nil {
		t.Fatalf("trimmed output not valid JSON: %v", err)
	}
	if _, ok := root["last_assistant_message"]; ok {
		t.Fatal("last_assistant_message should be removed")
	}
	for _, key := range []string{"hook_event_name", "session_id", "model", "cwd", "turn_id"} {
		if _, ok := root[key]; !ok {
			t.Fatalf("key %q should be preserved", key)
		}
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("trimmed payload must not be empty")
	}
}

func TestTrimStopHookPayload_NoChangeWhenAbsentOrInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"hook_event_name": "Stop", "session_id": "sess-2"}`,
		`not-json`,
		``,
	}
	for _, raw := range cases {
		out := traceclient.TrimStopHookPayload([]byte(raw))
		if !bytes.Equal(out, []byte(raw)) {
			t.Fatalf("input %q should pass through unchanged, got %q", raw, string(out))
		}
	}
}
