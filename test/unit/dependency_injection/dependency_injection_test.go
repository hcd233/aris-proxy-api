package dependency_injection_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfrastructureClientsAreInjectedWithoutGlobalGetters(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	forbidden := []string{
		"GetDB(",
		"GetDBInstance(",
		"GetRedisClient(",
	}

	var violations []string
	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".worktrees" || name == "graphify-out" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		for _, symbol := range forbidden {
			if strings.Contains(text, symbol) {
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				violations = append(violations, rel+": "+symbol)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("基础设施客户端应通过依赖注入传递，禁止全局 getter: %s", strings.Join(violations, "; "))
	}
}
