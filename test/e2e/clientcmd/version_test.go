package clientcmd_e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionCommand_PrintsInjectedVersion 验证版本号注入链路：
// 默认构建回退 dev；release 构建经 -ldflags -X main.version 注入 tag。
func TestVersionCommand_PrintsInjectedVersion(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)

	cases := []struct {
		name           string
		ldflags        string
		expectedOutput string
	}{
		{name: "default build falls back to dev", ldflags: "", expectedOutput: "dev"},
		{name: "release build injects tag via ldflags", ldflags: "-X main.version=v9.9.9", expectedOutput: "v9.9.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			binary := buildClient(t, root, tc.ldflags)
			assertVersionOutput(t, binary, tc.expectedOutput)
		})
	}
}

func buildClient(t *testing.T, root, ldflags string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "aris")
	args := []string{"build", "-buildvcs=false"}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", binary, "./cmd/client")
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build client: %v\n%s", err, output)
	}
	return binary
}

// assertVersionOutput 断言 version 子命令与 --version 输出均为裸版本字符串。
func assertVersionOutput(t *testing.T, binary, expected string) {
	t.Helper()
	for _, args := range [][]string{{"version"}, {"--version"}} {
		out, err := exec.CommandContext(t.Context(), binary, args...).Output()
		if err != nil {
			t.Fatalf("aris %s: %v", args[0], err)
		}
		if got := strings.TrimSpace(string(out)); got != expected {
			t.Fatalf("aris %s output = %q, want %q", args[0], got, expected)
		}
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "..")
}
