package audit_architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", ".."))
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestAuditRouteUsesLogListPath(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/router/audit.go")
	if !strings.Contains(content, `Path:        "/log/list"`) {
		t.Fatalf("audit list route should use /log/list path")
	}
	if strings.Contains(content, `Path:        "/logs"`) {
		t.Fatalf("audit list route should not keep legacy /logs path")
	}
}

func TestAuditHandlerAndApplicationDoNotDependOnInfrastructure(t *testing.T) {
	t.Parallel()
	files := []string{
		"internal/handler/audit.go",
		"internal/application/audit/query/list_audit_logs.go",
	}
	for _, rel := range files {
		content := readRepoFile(t, rel)
		if strings.Contains(content, "internal/infrastructure/") {
			t.Fatalf("%s must not import infrastructure packages", rel)
		}
		if strings.Contains(content, "gorm.io/gorm") {
			t.Fatalf("%s must not import gorm", rel)
		}
	}
}
