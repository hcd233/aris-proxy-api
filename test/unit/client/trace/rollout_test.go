package trace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func codexAdapter(t *testing.T) client.AgentAdapter {
	t.Helper()
	adapter, err := client.LookupAdapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestRolloutReader_ReadsOnlyNewCompleteLines(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool, codexAdapter(t))
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	fixture, err := os.ReadFile("./fixtures/rollout.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(first) != 2 {
		t.Fatalf("first = %d, %v", len(first), err)
	}
	if string(first[0].Payload) != `{"timestamp":"2026-07-18T00:00:00Z","type":"session_meta","payload":{"id":"s1","cwd":"/work"}}` {
		t.Fatalf("first payload changed: %s", first[0].Payload)
	}
	file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n" + `{"timestamp":"2026-07-18T00:00:04Z","type":"response_item","payload":{"id":"m2","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}` + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(second) != 1 || second[0].Event != "message" {
		t.Fatalf("second = %+v, %v", second, err)
	}
	third, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(third) != 0 {
		t.Fatalf("third = %d, %v", len(third), err)
	}
}

func TestRolloutReader_ExtractsNestedTurnIDFromPassthroughMetadata(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool, codexAdapter(t))
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	line := []byte(`{"type":"response_item","payload":{"type":"function_call","call_id":"call-1","name":"bash","internal_chat_message_metadata_passthrough":{"turn_id":"t1"}}}` + "\n")
	if err := os.WriteFile(transcript, line, 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %d, %v", len(records), err)
	}
	if records[0].TurnID != "t1" {
		t.Fatalf("turn_id = %q, want %q", records[0].TurnID, "t1")
	}
}

func TestRolloutReader_ResetsAfterTruncate(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool, codexAdapter(t))
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	line := []byte("{\"type\":\"response_item\",\"payload\":{\"id\":\"m1\",\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"hi\"}]}}\n")
	if err := os.WriteFile(transcript, line, 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(first) != 1 {
		t.Fatalf("first = %d, %v", len(first), err)
	}
	replacement := transcript + ".new"
	if err := os.WriteFile(replacement, line, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, transcript); err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(second) != 1 || second[0].DedupKey != first[0].DedupKey {
		t.Fatalf("second = %+v, %v", second, err)
	}
}

func TestRolloutReaderKeepsTokenCountAndSkipsIgnored(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool, codexAdapter(t))

	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"sid-1","type":"session_meta"}}`,
		`{"type":"turn_context","payload":{"cwd":"/tmp","model":"m1","turn_id":"t1"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":10}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":20}}}}`,
		`{"type":"event_msg","payload":{"type":"task_started"}}`,
		`{"type":"event_msg","payload":{"type":"task_complete"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"hi"}}`,
		`{"type":"response_item","payload":{"id":"r1","type":"message"}}`,
		`{"type":"weird","payload":{}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcript, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	records, err := reader.ReadNew(context.Background(), "sess-1", transcript)
	if err != nil {
		t.Fatalf("ReadNew error: %v", err)
	}
	// 保留：session_meta / token_count×2（服务端按固定 dedup key 覆盖）/ task_started / task_complete / response_item / unknown(服务端告警丢弃)
	if len(records) != 7 {
		t.Fatalf("records = %d, want 7", len(records))
	}
	parts := make([]string, 0, len(records))
	for _, r := range records {
		parts = append(parts, r.RecordType+":"+r.Event)
	}
	joined := strings.Join(parts, "|")
	for _, want := range []string{"session_meta:", "event_msg:token_count", "event_msg:task_started", "event_msg:task_complete", "response_item:message", "unknown:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("records 缺少 %q，实际: %s", want, joined)
		}
	}
	if strings.Contains(joined, "turn_context") || strings.Contains(joined, "agent_message") {
		t.Fatalf("被忽略记录不应出现: %s", joined)
	}
	// token_count 固定 dedup key：两条 token_count 的 DedupKey 相同
	var tokenDedup []string
	for _, r := range records {
		if r.RecordType == "event_msg" && r.Event == "token_count" {
			tokenDedup = append(tokenDedup, r.DedupKey)
		}
	}
	if len(tokenDedup) != 2 || tokenDedup[0] == "" || tokenDedup[0] != tokenDedup[1] {
		t.Fatalf("token_count dedup key 应固定且相同，实际: %v", tokenDedup)
	}
}
