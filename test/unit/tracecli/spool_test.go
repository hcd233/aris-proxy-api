package tracecli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

func TestSpool_ConcurrentAppendIsAtomicAndPrivate(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1<<20)
	errCh := make(chan error, 20)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			errCh <- spool.Append(context.Background(), client.PendingRecord{
				SessionID:  "session-1",
				Source:     "hook",
				RecordType: "hook_event",
				Event:      "PreToolUse",
				DedupKey:   fmt.Sprintf("hook:spool:%d", i),
				Payload:    sonic.NoCopyRawMessage(`{"session_id":"session-1"}`),
			})
		})
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	batch, err := spool.Batch(context.Background(), 500, 4<<20)
	if err != nil || len(batch) != 20 {
		t.Fatalf("batch = %d, %v", len(batch), err)
	}
	entries, err := os.ReadDir(paths.PendingDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			t.Fatalf("temporary file remained: %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", entry.Name(), info.Mode().Perm())
		}
	}
}

func TestSpool_HardLimitKeepsExistingPendingRecords(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := client.NewSpool(paths, 1)
	err := spool.Append(context.Background(), client.PendingRecord{
		SessionID: "session-1",
		DedupKey:  "hook:spool:1",
		Payload:   sonic.NoCopyRawMessage(`{"value":true}`),
	})
	if err == nil {
		t.Fatal("expected hard limit error")
	}
}
