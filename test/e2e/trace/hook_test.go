package trace_e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

func TestCodexHook_PersistsAndReportsAllEvents(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []struct {
		Event string `json:"hook_event_name"`
	}
	requestCh := make(chan struct{}, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		var request struct {
			Event string `json:"hook_event_name"`
		}
		if err := sonic.Unmarshal(body, &request); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, request)
		mu.Unlock()
		requestCh <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	root := t.TempDir()
	script := filepath.Join("..", "..", "..", "web", "src", "scripts", "codex-hook.sh")
	events := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	for i, event := range events {
		payload := []byte(`{"hook_event_name":"` + event + `","session_id":"hook-test-session","turn_id":"turn-1"}`)
		stdout := runHook(t, script, root, server.URL, payload)
		if event == "Stop" && stdout != "{}" {
			t.Fatalf("Stop stdout = %q, want {}", stdout)
		}
		if i < len(events)-1 && stdout != "" {
			t.Fatalf("%s stdout = %q, want empty", event, stdout)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-requestCh:
	case <-ctx.Done():
		t.Fatal("hook did not report a request")
	}

	mu.Lock()
	defer mu.Unlock()
	seen := map[string]bool{}
	for _, request := range requests {
		seen[request.Event] = true
	}
	for _, event := range events {
		if !seen[event] {
			t.Errorf("event %s was not reported; requests=%d", event, len(requests))
		}
	}
}

func runHook(t *testing.T, script, root, traceURL string, payload []byte) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "bash", script)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(),
		"TRACE_URL="+traceURL,
		"API_KEY=test-key",
		"TRACE_ROOT="+root,
		"LOG_DIR="+filepath.Join(root, "logs"),
	)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run hook: %v", err)
	}
	return string(output)
}
