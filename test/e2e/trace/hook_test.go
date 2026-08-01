package trace_e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestCodexHook_TriggersTranscriptReportWithoutHookRecords(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var gotBatch struct {
		Agent   string `json:"agent"`
		Model   string `json:"model"`
		CWD     string `json:"cwd"`
		Source  string `json:"source"`
		Records []struct {
			Source   string `json:"source"`
			Event    string `json:"hook_event_name"`
			DedupKey string `json:"dedup_key"`
		} `json:"records"`
	}
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Agent   string `json:"agent"`
			Model   string `json:"model"`
			CWD     string `json:"cwd"`
			Source  string `json:"source"`
			Records []struct {
				Source   string `json:"source"`
				Event    string `json:"hook_event_name"`
				DedupKey string `json:"dedup_key"`
			} `json:"records"`
		}
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := make([]client.RecordResult, 0, len(request.Records))
		for _, rec := range request.Records {
			results = append(results, client.RecordResult{DedupKey: rec.DedupKey, Status: "accepted"})
		}
		mu.Lock()
		requestCount++
		gotBatch = request
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
	configDir := paths.TraceDir()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","agent":"codex","apiKey":"test-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := buildTraceClient(t)

	// 预置 transcript：codex hook 纯触发后只上报 rollout 记录
	transcriptPath := filepath.Join(home, "rollout.jsonl")
	transcript := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"hook-test-session","type":"session_meta"}}`,
		`{"type":"response_item","payload":{"id":"r1","type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":10}}}}`,
		`{"type":"event_msg","payload":{"type":"task_complete"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"noise"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}

	// SessionStart 触发：写元数据 + 读 transcript → 上报 rollout 记录
	sessionStart := `{"hook_event_name":"SessionStart","session_id":"hook-test-session","model":"glm-5.2","cwd":"/work","source":"startup","transcript_path":"` + transcriptPath + `"}`
	if stdout := runTraceIngest(t, binary, home, "codex", sessionStart); stdout != "" {
		t.Fatalf("SessionStart stdout = %q, want empty", stdout)
	}
	// Stop 触发（无 transcript_path）：不产生新批次（无记录可发）
	stop := `{"hook_event_name":"Stop","session_id":"hook-test-session","turn_id":"turn-1"}`
	if stdout := runTraceIngest(t, binary, home, "codex", stop); stdout != "{}" {
		t.Fatalf("Stop stdout = %q, want {}", stdout)
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCount != 1 {
		t.Fatalf("requests = %d, want 1（仅 SessionStart 触发产生批次，Stop 无记录不发）", requestCount)
	}
	if gotBatch.Agent != "codex" {
		t.Fatalf("batch agent = %q, want codex", gotBatch.Agent)
	}
	if gotBatch.Model != "glm-5.2" || gotBatch.CWD != "/work" || gotBatch.Source != "startup" {
		t.Fatalf("batch 元数据 = %+v", gotBatch)
	}
	if len(gotBatch.Records) == 0 {
		t.Fatal("无记录上报")
	}
	var events []string
	for _, rec := range gotBatch.Records {
		if rec.Source != "rollout" {
			t.Fatalf("出现非 rollout 记录: %+v", rec)
		}
		events = append(events, rec.Event)
	}
	joined := strings.Join(events, "|")
	for _, want := range []string{"session_meta", "message", "token_count", "task_complete"} {
		if !strings.Contains(joined, want) {
			t.Errorf("事件 %s 未上报，实际: %s", want, joined)
		}
	}
	if strings.Contains(joined, "agent_message") || strings.Contains(joined, "SessionStart") || strings.Contains(joined, "Stop") {
		t.Fatalf("不应出现被过滤事件或 hook 事件: %s", joined)
	}
}

func TestClaudeHook_SilentStdoutAndReportsAgent(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var agentEnvelopes []string
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Agent   string `json:"agent"`
			Records []struct {
				Source   string `json:"source"`
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
		agentEnvelopes = append(agentEnvelopes, request.Agent)
		for _, record := range request.Records {
			if record.Source == "hook" {
				seen = append(seen, record.Event)
			}
			results = append(results, client.RecordResult{DedupKey: record.DedupKey, Status: "accepted"})
		}
		mu.Unlock()
		data, _ := sonic.Marshal(struct {
			Results []client.RecordResult `json:"results"`
		}{Results: results})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	home := t.TempDir()
	paths := client.Paths{Root: filepath.Join(home, ".aris")}
	if err := os.MkdirAll(paths.TraceDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","apiKey":"test-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := buildTraceClient(t)

	transcript := filepath.Join(home, "session.jsonl")
	transcriptLines := []string{
		`{"type":"permission-mode","permissionMode":"default","sessionId":"claude-session"}`,
		`{"type":"user","uuid":"u1","promptId":"p1","sessionId":"claude-session","message":{"role":"user","content":"列一下文件"}}`,
		`{"type":"assistant","uuid":"a1","sessionId":"claude-session","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","uuid":"u2","sessionId":"claude-session","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`,
	}
	if err := os.WriteFile(transcript, []byte(strings.Join(transcriptLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	hooks := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"}
	for _, event := range hooks {
		payload := `{"hook_event_name":"` + event + `","session_id":"claude-session","prompt_id":"p1","transcript_path":"` + transcript + `"}`
		if stdout := runTraceIngest(t, binary, home, "claude", payload); stdout != "" {
			t.Fatalf("claude %s stdout = %q, must always be empty", event, stdout)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(agentEnvelopes) == 0 {
		t.Fatal("no envelopes reported")
	}
	for _, agent := range agentEnvelopes {
		if agent != "claude" {
			t.Fatalf("batch agent = %q, want claude", agent)
		}
	}
	for _, event := range hooks {
		found := false
		for _, s := range seen {
			if s == event {
				found = true
			}
		}
		if !found {
			t.Errorf("hook event %s was not reported", event)
		}
	}
}

func buildTraceClient(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "aris")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-buildvcs=false", "-o", binary, "./cmd/client")
	cmd.Dir = projectRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build client: %v\n%s", err, output)
	}
	return binary
}

func runTraceIngest(t *testing.T, binary, home, agent, payload string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "trace", "ingest", "--agent", agent)
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
