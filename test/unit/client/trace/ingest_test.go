package trace

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestRunIngestCommand_FailOpenStdoutContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload string
		stdout  string
	}{
		{name: "stop", payload: `{"session_id":"s1","hook_event_name":"Stop"}`, stdout: "{}"},
		{name: "other", payload: `{"session_id":"s1","hook_event_name":"PreToolUse"}`},
		{name: "malformed", payload: `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
			var out bytes.Buffer
			err := client.RunIngestCommand(context.Background(), client.IngestCommandOptions{
				Paths:     paths,
				In:        bytes.NewBufferString(tc.payload),
				Out:       &out,
				AgentName: "codex",
			})
			if err != nil {
				t.Fatalf("command returned error: %v", err)
			}
			if out.String() != tc.stdout {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.stdout)
			}
		})
	}
}

func TestRunIngestCommand_MissingOrUnknownAgentFailOpen(t *testing.T) {
	t.Parallel()
	for _, agentName := range []string{"", "nope"} {
		t.Run("agent="+agentName, func(t *testing.T) {
			t.Parallel()
			paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
			var out bytes.Buffer
			err := client.RunIngestCommand(context.Background(), client.IngestCommandOptions{
				Paths:     paths,
				In:        bytes.NewBufferString(`{"session_id":"s1","hook_event_name":"Stop"}`),
				Out:       &out,
				AgentName: agentName,
			})
			if err != nil {
				t.Fatalf("command must fail-open, got error: %v", err)
			}
			if out.String() != "" {
				t.Fatalf("stdout = %q, want empty (no ack without adapter)", out.String())
			}
		})
	}
}

func TestRunIngestCommand_FlushesAcceptedRecord(t *testing.T) {
	t.Parallel()
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		var request struct {
			Records []struct {
				DedupKey string `json:"dedup_key"`
			} `json:"records"`
		}
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		response := struct {
			Results []client.RecordResult `json:"results"`
		}{Results: []client.RecordResult{{DedupKey: request.Records[0].DedupKey, Status: "accepted"}}}
		data, err := sonic.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	configDir := paths.TraceDir()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","agent":"codex","apiKey":"proxy-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := client.RunIngestCommand(context.Background(), client.IngestCommandOptions{
		Paths: paths,
		In: bytes.NewBufferString(
			`{"session_id":"s1","hook_event_name":"UserPromptSubmit","turn_id":"t1"}`,
		),
		Out:        &out,
		HTTPClient: server.Client(),
		AgentName:  "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer proxy-key" {
		t.Fatalf("authorization = %q", authorization)
	}
	entries, err := os.ReadDir(paths.PendingDir())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("pending records = %d", len(entries))
	}
}

func TestRunIngestCommand_CodexHookTriggersWithoutHookRecord(t *testing.T) {
	t.Parallel()
	var gotRequest struct {
		SessionID string `json:"session_id"`
		Model     string `json:"model"`
		CWD       string `json:"cwd"`
		Source    string `json:"source"`
		Records   []struct {
			Source     string `json:"source"`
			RecordType string `json:"record_type"`
			Event      string `json:"hook_event_name"`
		} `json:"records"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatal(err)
		}
		results := make([]client.RecordResult, 0, len(gotRequest.Records))
		for range gotRequest.Records {
			results = append(results, client.RecordResult{DedupKey: "k", Status: "accepted"})
		}
		data, _ := sonic.Marshal(struct {
			Results []client.RecordResult `json:"results"`
		}{Results: results})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	if err := os.MkdirAll(paths.TraceDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","agent":"codex","apiKey":"proxy-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(t.TempDir(), "rollout.jsonl")
	transcript := `{"type":"session_meta","payload":{"id":"sid-1","type":"session_meta"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":10}}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	hookInput := `{"hook_event_name":"SessionStart","session_id":"sess-1","model":"glm-5.2","cwd":"/tmp/x","source":"startup","transcript_path":"` + transcriptPath + `"}`

	var out bytes.Buffer
	if err := client.RunIngestCommand(context.Background(), client.IngestCommandOptions{
		Paths:      paths,
		In:         bytes.NewBufferString(hookInput),
		Out:        &out,
		HTTPClient: server.Client(),
		AgentName:  "codex",
	}); err != nil {
		t.Fatal(err)
	}
	if gotRequest.SessionID != "sess-1" {
		t.Fatalf("session_id = %q", gotRequest.SessionID)
	}
	if gotRequest.Model != "glm-5.2" || gotRequest.CWD != "/tmp/x" {
		t.Fatalf("batch 元数据 = %+v，期望 model=glm-5.2 cwd=/tmp/x", gotRequest)
	}
	for _, rec := range gotRequest.Records {
		if rec.Source != "rollout" {
			t.Fatalf("出现非 rollout 记录: %+v", rec)
		}
	}
	events := make([]string, 0, len(gotRequest.Records))
	for _, rec := range gotRequest.Records {
		events = append(events, rec.RecordType+":"+rec.Event)
	}
	joined := strings.Join(events, "|")
	if !strings.Contains(joined, "event_msg:task_complete") {
		t.Fatalf("task_complete 应上报: %s", joined)
	}
	if !strings.Contains(joined, "event_msg:token_count") {
		t.Fatalf("token_count 应上报（服务端覆盖保留最后一条）: %s", joined)
	}
}
