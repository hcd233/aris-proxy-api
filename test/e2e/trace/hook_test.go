package trace_e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

func TestCodexHook_PersistsAndReportsAllEvents(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Records []struct {
				Event    string `json:"hook_event_name"`
				DedupKey string `json:"dedup_key"`
			} `json:"records"`
		}
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := make([]client.RecordResult, 0, len(request.Records))
		mu.Lock()
		for _, record := range request.Records {
			seen[record.Event] = true
			results = append(results, client.RecordResult{DedupKey: record.DedupKey, Status: "accepted"})
		}
		mu.Unlock()
		data, err := sonic.Marshal(struct {
			Results []client.RecordResult `json:"results"`
		}{Results: results})
		if err != nil {
			http.Error(w, "encode response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	home := t.TempDir()
	paths := client.Paths{Root: filepath.Join(home, ".aris")}
	if err := client.NewConfigStore(paths).Save(context.Background(), client.Config{
		Host: server.URL, Agent: "codex", APIKey: "test-key",
	}); err != nil {
		t.Fatal(err)
	}
	binary := buildTraceClient(t)
	events := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	for _, event := range events {
		payload := `{"hook_event_name":"` + event + `","session_id":"hook-test-session","turn_id":"turn-1"}`
		stdout := runTraceIngest(t, binary, home, payload)
		if event == "Stop" && stdout != "{}" {
			t.Fatalf("Stop stdout = %q, want {}", stdout)
		}
		if event != "Stop" && stdout != "" {
			t.Fatalf("%s stdout = %q, want empty", event, stdout)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, event := range events {
		if !seen[event] {
			t.Errorf("event %s was not reported", event)
		}
	}
}

func buildTraceClient(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "aris")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/client")
	cmd.Dir = projectRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build client: %v\n%s", err, output)
	}
	return binary
}

func runTraceIngest(t *testing.T, binary, home, payload string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "trace", "ingest")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = strings.NewReader(payload)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("trace ingest: %v", err)
	}
	return string(output)
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "..")
}
