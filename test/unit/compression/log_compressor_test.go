package compression

import (
	"strings"
	"testing"

	compression "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestLogCompressorErrorPreservation(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor(compression.DefaultLogCompressorConfig())
	var lines []string
	lines = append(lines, "INFO starting build")
	for i := 0; i < 30; i++ {
		lines = append(lines, "INFO processing item "+itoaStr(i))
	}
	lines = append(lines, "ERROR build failed: connection refused")
	lines = append(lines, "WARN cleanup in progress")
	content := strings.Join(lines, "\n")
	result := lc.Compress(content)
	if !strings.Contains(result.Compressed, "ERROR build failed") {
		t.Error("should keep error line")
	}
	t.Logf("compressed lines: %d -> %d", result.OriginalLineCount, result.CompressedLineCount)
}

func TestLogCompressorStacktracePreservation(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor(compression.DefaultLogCompressorConfig())
	content := `ERROR something broke
Traceback (most recent call last):
  File "main.py", line 42, in <module>
    process()
  File "main.py", line 38, in process
    raise ValueError("bad input")
ValueError: bad input`
	result := lc.Compress(content)
	if !strings.Contains(result.Compressed, "Traceback") {
		t.Error("should keep traceback")
	}
	if !strings.Contains(result.Compressed, "ValueError") {
		t.Error("should keep exception line")
	}
}

func TestLogCompressorDedupWarnings(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor(compression.DefaultLogCompressorConfig())
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "WARN connection timeout after 30s")
	}
	content := strings.Join(lines, "\n")
	result := lc.Compress(content)
	t.Logf("dedup result: %s", result.Compressed)
}

func TestLogCompressorEmptyContent(t *testing.T) {
	t.Parallel()
	lc := compression.NewLogCompressor(compression.DefaultLogCompressorConfig())
	result := lc.Compress("")
	if result.Compressed != "" {
		t.Error("compressed should be empty")
	}
}

func itoaStr(n int) string {
	digits := "0123456789"
	s := ""
	v := n
	if v == 0 {
		return "0"
	}
	for v > 0 {
		s = string(digits[v%10]) + s
		v /= 10
	}
	return s
}
