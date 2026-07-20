package tracecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

func TestRolloutReader_ReadsOnlyNewCompleteLines(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool)
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	fixture, err := os.ReadFile("./fixtures/rollout.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(first) != 3 {
		t.Fatalf("first = %d, %v", len(first), err)
	}
	if string(first[0].Payload) != `{"timestamp":"2026-07-18T00:00:00Z","type":"session_meta","payload":{"id":"s1","cwd":"/work"}}` {
		t.Fatalf("first payload changed: %s", first[0].Payload)
	}
	file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`lo"}}` + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(second) != 1 || second[0].Event != "agent_message" {
		t.Fatalf("second = %+v, %v", second, err)
	}
	third, err := reader.ReadNew(context.Background(), "s1", transcript)
	if err != nil || len(third) != 0 {
		t.Fatalf("third = %d, %v", len(third), err)
	}
}

func TestRolloutReader_ResetsAfterTruncate(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	reader := client.NewRolloutReader(paths, spool)
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	line := []byte("{\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"turn_id\":\"t1\"}}\n")
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
