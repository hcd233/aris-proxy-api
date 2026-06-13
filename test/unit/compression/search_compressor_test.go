package compression

import (
	"strings"
	"testing"

	compression "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSearchCompressorParseAndSelect(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor(compression.DefaultSearchCompressorConfig())
	var lines []string
	for i := 0; i < 20; i++ {
		line := "src/main.go:" + itoaSearch(i+1) + ":line content " + itoaSearch(i)
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content, "")
	t.Logf("original matches: %d, compressed: %d", result.OriginalMatchCount, result.CompressedMatchCount)
	if result.OriginalMatchCount == 0 {
		t.Error("should have original matches")
	}
}

func TestSearchCompressorKeepsFirstAndLast(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor(compression.DefaultSearchCompressorConfig())
	var lines []string
	for i := 0; i < 50; i++ {
		line := "src/main.go:" + itoaSearch(i+1) + ":line content " + itoaSearch(i)
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content, "")
	if !strings.Contains(result.Compressed, "line content 0") {
		t.Error("should keep first match")
	}
	if !strings.Contains(result.Compressed, "line content 49") {
		t.Error("should keep last match")
	}
}

func TestSearchCompressorEmptyContent(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor(compression.DefaultSearchCompressorConfig())
	result := sc.Compress("", "")
	if result.Compressed != "" {
		t.Error("compressed should be empty")
	}
}

func TestSearchCompressorContextBoost(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor(compression.DefaultSearchCompressorConfig())
	content := "src/main.go:1:normal line\nsrc/main.go:2:error handler\nsrc/main.go:3:normal line\nsrc/main.go:4:normal line\nsrc/main.go:5:normal line"
	result := sc.Compress(content, "error")
	if !strings.Contains(result.Compressed, "error handler") {
		t.Error("error match should be kept when context matches")
	}
}

func itoaSearch(n int) string {
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
