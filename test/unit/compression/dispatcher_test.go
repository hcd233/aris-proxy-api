package compression

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestDispatcherJsonArray(t *testing.T) {
	t.Parallel()
	d := compression.NewDispatcher()
	content := `[{"a":1},{"a":2},{"a":3}]`
	result := d.Compress(content)
	if !result.Applied {
		t.Error("expected JSON array to be compressed")
	}
	if result.Strategy != "smart_crusher" {
		t.Errorf("strategy = %s, want smart_crusher", result.Strategy)
	}
}

func TestDispatcherPlainTextPassthrough(t *testing.T) {
	t.Parallel()
	d := compression.NewDispatcher()
	result := d.Compress("just some plain text")
	if result.Applied {
		t.Error("plain text should passthrough")
	}
}

func TestDispatcherSearchResults(t *testing.T) {
	t.Parallel()
	d := compression.NewDispatcher()
	var lines []string
	for i := 1; i <= 15; i++ {
		lines = append(lines, fmt.Sprintf("file1.go:%d:line%d", i, i))
	}
	content := strings.Join(lines, "\n")
	result := d.Compress(content)
	if !result.Applied {
		t.Error("expected search results to be compressed")
	}
}
