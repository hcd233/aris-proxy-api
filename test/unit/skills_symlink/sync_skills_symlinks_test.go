package skills_symlink_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSyncSkillsSymlinksCreatesLinksAndKeepsConflicts(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptData, err := os.ReadFile(filepath.Join(repoRoot, "script", "sync-skills-symlinks.sh"))
	if err != nil {
		t.Fatalf("read sync script: %v", err)
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".agents", "skills", "alpha", "README.md"), "alpha")
	writeFile(t, filepath.Join(root, ".agents", "skills", "existing", "README.md"), "existing")
	writeFile(t, filepath.Join(root, ".claude", "skills", "existing"), "keep")
	writeFile(t, filepath.Join(root, "script", "sync-skills-symlinks.sh"), string(scriptData))

	runSyncScript(t, root)
	runSyncScript(t, root)

	assertSymlink(t, filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join("..", "..", ".agents", "skills", "alpha"))
	assertSymlink(t, filepath.Join(root, ".codebuddy", "skills", "alpha"), filepath.Join("..", "..", ".agents", "skills", "alpha"))
	assertSymlink(t, filepath.Join(root, ".codebuddy", "skills", "existing"), filepath.Join("..", "..", ".agents", "skills", "existing"))

	content, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "existing"))
	if err != nil {
		t.Fatalf("read conflict file: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("conflict file content = %q, want keep", string(content))
	}
}

func runSyncScript(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(root, "script", "sync-skills-symlinks.sh"))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run sync script: %v\n%s", err, string(output))
	}
}

func assertSymlink(t *testing.T, path string, wantTarget string) {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if target != wantTarget {
		t.Fatalf("symlink target for %s = %q, want %q", path, target, wantTarget)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatalf("repo root not found from %s", wd)
		}
		wd = parent
	}
}
