package compression

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func TestDetectJsonArray(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := `[{"name":"error","code":500},{"name":"warn","code":0}]`
	if got := d.Detect(content); got != enum.ContentTypeJsonArray {
		t.Errorf("Detect() = %v, want JsonArray", got)
	}
}

func TestDetectSearchResults(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := "src/main.go:42:func main() {\nsrc/main.go:43:    fmt.Println()\nsrc/utils.go:10:func helper() {"
	if got := d.Detect(content); got != enum.ContentTypeSearchResults {
		t.Errorf("Detect() = %v, want SearchResults", got)
	}
}

func TestDetectBuildOutput(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := "[2024-01-01 10:00:00] INFO Starting server\n[2024-01-01 10:00:01] ERROR Connection failed\n[2024-01-01 10:00:02] WARN Retrying"
	if got := d.Detect(content); got != enum.ContentTypeBuildOutput {
		t.Errorf("Detect() = %v, want BuildOutput", got)
	}
}

func TestDetectGitDiff(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@"
	if got := d.Detect(content); got != enum.ContentTypeGitDiff {
		t.Errorf("Detect() = %v, want GitDiff", got)
	}
}

func TestDetectSourceCode(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}"
	if got := d.Detect(content); got != enum.ContentTypeSourceCode {
		t.Errorf("Detect() = %v, want SourceCode", got)
	}
}

func TestDetectPlainText(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	content := "This is just a plain text message without any special format."
	if got := d.Detect(content); got != enum.ContentTypePlainText {
		t.Errorf("Detect() = %v, want PlainText", got)
	}
}

func TestDetectEmpty(t *testing.T) {
	t.Parallel()
	d := compression.NewContentDetector()
	if got := d.Detect(""); got != enum.ContentTypePlainText {
		t.Errorf("Detect(\"\") = %v, want PlainText", got)
	}
}

func TestPassthroughCompressor(t *testing.T) {
	t.Parallel()
	p := compression.NewPassthroughCompressor()
	result := p.Compress("hello world")
	if result.Applied {
		t.Error("Passthrough should not apply compression")
	}
	if result.Output != "hello world" {
		t.Errorf("Output = %q, want %q", result.Output, "hello world")
	}
}
