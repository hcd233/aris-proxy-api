package compression

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSearchCompressorTruncate(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor()
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("src/main.go:%d:    some code line number %d", i, i))
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.Contains(result.Output, "省略") {
		t.Error("expected truncation summary")
	}
	if !strings.Contains(result.Output, "共 1 个文件") {
		t.Error("expected file count summary")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestSearchCompressorSmallPassthrough(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor()
	content := "src/main.go:42:func main() {\nsrc/utils.go:10:func helper() {"
	result := sc.Compress(content)
	if result.Applied {
		t.Error("small result should passthrough")
	}
}

func TestSearchCompressorNonSearchPassthrough(t *testing.T) {
	t.Parallel()
	sc := compression.NewSearchCompressor()
	content := "just some text without file:line format"
	result := sc.Compress(content)
	if result.Applied {
		t.Error("non-search content should passthrough")
	}
}
