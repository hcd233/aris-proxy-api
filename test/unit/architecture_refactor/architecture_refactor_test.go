package architecture_refactor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLLMProxyUsecaseDoesNotImportInfrastructureTransport(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	usecaseDir := filepath.Join(root, "internal", "application", "llmproxy", "usecase")
	forbidden := "github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"

	var violations []string
	err := filepath.WalkDir(usecaseDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), forbidden) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("llmproxy usecase must depend on application ports, not infrastructure transport: %s", strings.Join(violations, ", "))
	}
}

func TestProtocolUtilitiesMovedOutOfInternalUtil(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	movedFiles := []string{"anthropic.go", "openai.go", "sse.go", "model.go", "openai_stream.go"}
	for _, name := range movedFiles {
		if _, err := os.Stat(filepath.Join(root, "internal", "util", name)); err == nil {
			t.Fatalf("protocol util %s should not remain in internal/util", name)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat old protocol util %s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, "internal", "application", "llmproxy", "util", name)); err != nil {
			t.Fatalf("protocol util %s should exist in application/llmproxy/util: %v", name, err)
		}
	}
}

func TestHTTPResponseUtilitiesMovedToAPIUtil(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "util", "http.go")); err == nil {
		t.Fatal("HTTP response util should not remain in internal/util/http.go")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat old HTTP util: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "api", "util", "http.go")); err != nil {
		t.Fatalf("HTTP response util should exist in internal/api/util/http.go: %v", err)
	}
}

func TestConverterAndUsecaseFilesAreSplitByResponsibility(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	files := []string{
		"internal/application/llmproxy/converter/anthropic_response.go",
		"internal/application/llmproxy/converter/anthropic_sse.go",
		"internal/application/llmproxy/converter/openai_sse.go",
		"internal/application/llmproxy/usecase/anthropic_message.go",
		"internal/application/llmproxy/usecase/openai_chat.go",
		"internal/application/llmproxy/usecase/openai_response.go",
	}
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); err != nil {
			t.Fatalf("expected file %s to exist: %v", file, err)
		}
	}
}
