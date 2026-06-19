package compression

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestLogCompressorDedup(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor()
	content := strings.Join([]string{
		"[2024-01-01 10:00:00] INFO Starting server on port 8080",
		"[2024-01-01 10:00:01] INFO Database connection established",
		"[2024-01-01 10:00:02] INFO Cache warmed with 500 entries",
		"[2024-01-01 10:00:03] INFO Request processed in 42ms",
		"[2024-01-01 10:00:04] INFO Request processed in 38ms",
		"[2024-01-01 10:00:05] INFO Request processed in 51ms",
		"[2024-01-01 10:00:06] ERROR Connection refused to database:5432",
		"[2024-01-01 10:00:07] WARN Retrying connection attempt 1",
		"[2024-01-01 10:00:08] INFO Request processed in 45ms",
	}, "\n")
	result := lc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.Contains(result.Output, "ERROR") {
		t.Error("ERROR line should be preserved")
	}
	if !strings.Contains(result.Output, "WARN") {
		t.Error("WARN line should be preserved")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestLogCompressorShortPassthrough(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor()
	content := "only one line"
	result := lc.Compress(content)
	if result.Applied {
		t.Error("short content should passthrough")
	}
}
