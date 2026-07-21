package tracecli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
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
				Paths: paths,
				In:    bytes.NewBufferString(tc.payload),
				Out:   &out,
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
