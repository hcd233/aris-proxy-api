package trace

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

// TestIngestSubagentStop_ReportsChildTranscriptWithParentSession 验证 SubagentStop
// hook 触发时：只上报子代理 transcript（SessionID=子代理 id），batch 携带父 session_id。
func TestIngestSubagentStop_ReportsChildTranscriptWithParentSession(t *testing.T) {
	t.Parallel()
	paths := traceclient.Paths{Root: t.TempDir()}
	childID := "019f6f10-7524-7891-96a2-fe7aa659430c"
	transcriptDir := filepath.Join(t.TempDir(), "sessions")
	transcriptPath := filepath.Join(transcriptDir, "rollout-2026-07-17T15-52-57-"+childID+".jsonl")
	if err := os.MkdirAll(transcriptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta := `{"type":"session_meta","payload":{"id":"` + childID + `","session_id":"` + childID + `","source":"vscode"}}` + "\n"
	complete := `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(meta+complete), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter, err := traceclient.LookupAdapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	var gotBody traceclient.IngestBatchJSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := sonic.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("bad ingest body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"dedupKey":"x","status":"accepted"}]}`))
	}))
	defer server.Close()

	cfg := traceclient.Config{Host: server.URL, APIKey: "k"}
	configStore := traceclient.NewConfigStore(paths)
	if err := configStore.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	ing := traceclient.NewIngestor(paths, server.Client(), adapter)
	hookInput := []byte(`{"session_id":"parent-s1","hook_event_name":"SubagentStop","turn_id":"t1","agent_id":"agent-1","agent_type":"worker","agent_transcript_path":"` + transcriptPath + `"}`)
	if err := ing.Ingest(context.Background(), hookInput); err != nil {
		t.Fatalf("ingest subagent stop: %v", err)
	}

	if gotBody.ParentSessionID != "parent-s1" {
		t.Fatalf("expected ParentSessionID=parent-s1, got %q", gotBody.ParentSessionID)
	}
	if gotBody.SessionID != childID {
		t.Fatalf("expected SessionID=%s, got %q", childID, gotBody.SessionID)
	}
	if gotBody.AgentID != "agent-1" || gotBody.AgentType != "worker" {
		t.Fatalf("expected agent metadata agent-1/worker, got %q/%q", gotBody.AgentID, gotBody.AgentType)
	}
	if len(gotBody.Records) != 2 {
		t.Fatalf("expected 2 rollout records, got %d", len(gotBody.Records))
	}
	for _, rec := range gotBody.Records {
		if rec.Source != "rollout" {
			t.Fatalf("expected all records source=rollout, got %q", rec.Source)
		}
	}
}

// TestBatchForSession_IgnoresOtherSessions 验证混合会话 spool 下 BatchForSession 只取目标会话记录。
func TestBatchForSession_IgnoresOtherSessions(t *testing.T) {
	t.Parallel()
	paths := traceclient.Paths{Root: t.TempDir()}
	spool := traceclient.NewSpool(paths, 1<<20)
	ctx := context.Background()

	parent := traceclient.PendingRecord{
		SessionID: "parent-s1", Agent: "codex", Source: "hook", RecordType: "hook_event",
		Event: "Stop", DedupKey: "hook:parent:1", Payload: []byte(`{"x":1}`),
	}
	child := traceclient.PendingRecord{
		SessionID: "child-s1", ParentSessionID: "parent-s1", AgentID: "agent-1", AgentType: "worker",
		Agent: "codex", Source: "rollout", RecordType: "event_msg",
		Event: "task_complete", DedupKey: "rollout:child-s1:1", Payload: []byte(`{"x":2}`),
	}
	if err := spool.Append(ctx, parent); err != nil {
		t.Fatalf("append parent: %v", err)
	}
	if err := spool.Append(ctx, child); err != nil {
		t.Fatalf("append child: %v", err)
	}

	batch, err := spool.BatchForSession(ctx, "child-s1", 100, 1<<20)
	if err != nil {
		t.Fatalf("BatchForSession: %v", err)
	}
	if len(batch) != 1 || batch[0].DedupKey != "rollout:child-s1:1" {
		t.Fatalf("expected only child record, got %+v", batch)
	}
	if batch[0].ParentSessionID != "parent-s1" || batch[0].AgentID != "agent-1" {
		t.Fatalf("child metadata lost: %+v", batch[0])
	}
}
