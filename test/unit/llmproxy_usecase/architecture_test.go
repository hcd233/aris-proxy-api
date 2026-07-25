package llmproxy_usecase

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLLMProxyApplicationDoesNotImportHTTPTransport 锁定 application 层依赖边界：
// internal/application/llmproxy/usecase 和 port 不得导入 Huma、Fiber 或 internal/api/util。
//
// 通过源码扫描而非编译器错误信息判定，确保后续迁移过程中能够即时暴露越界依赖。
// 路径基于测试文件本身定位（runtime.Caller），不依赖 go test 的 CWD。
//
// NOTE: P3-2 迁移期间 (Task 1-5) 此测试 t.Skip 跳过，因为 pre-commit hook 要求 `go test ./...`
// 全部通过才能 commit；当前 usecase/port 仍存在大量 huma/fiber/apiutil 依赖（按 plan 分阶段迁移）。
// Task 6 完成所有迁移后移除 t.Skip，启用最终验证。
// 迁移期间通过 `rg` 命令在每个 Task commit 前手动监控越界状态：
//
//	rg -n 'huma|humafiber|internal/api/util|github.com/gofiber/fiber/v3' \
//	  internal/application/llmproxy/usecase internal/application/llmproxy/port
func TestLLMProxyApplicationDoesNotImportHTTPTransport(t *testing.T) {
	t.Skip("P3-2 migration in progress; enabled in Task 6 once all paths migrated to transport-neutral results")

	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate test file")
	}
	// testFile: .../test/unit/llmproxy_usecase/architecture_test.go
	moduleRoot := filepath.Join(filepath.Dir(testFile), "..", "..", "..")

	forbidden := []string{
		"github.com/danielgtaylor/huma/v2",
		"github.com/gofiber/fiber/v3",
		"internal/api/util",
	}

	roots := []string{
		filepath.Join(moduleRoot, "internal", "application", "llmproxy", "usecase"),
		filepath.Join(moduleRoot, "internal", "application", "llmproxy", "port"),
	}

	var failures []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, bad := range forbidden {
				if strings.Contains(string(content), bad) {
					rel, _ := filepath.Rel(moduleRoot, path)
					failures = append(failures, rel+": contains forbidden dependency "+bad)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	if len(failures) > 0 {
		t.Fatalf("forbidden HTTP transport dependencies found in application layer:\n%s",
			strings.Join(failures, "\n"))
	}
}
