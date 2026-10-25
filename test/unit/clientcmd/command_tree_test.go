// Package clientcmd 守护 server/lint/aris 三入口的命令树契约与客户端跨平台构建。
package clientcmd

import (
	"os"
	"os/exec"
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

func runGo(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestServerCommandTreePreservesExistingCommands(t *testing.T) {
	t.Parallel()
	out := runGo(t, "run", "./cmd/server", "--help")
	for _, name := range []string{"server", "database", "object"} {
		if !strings.Contains(out, name) {
			t.Errorf("help missing %q:\n%s", name, out)
		}
	}
	// lint 已拆分到 cmd/lint 独立入口，不应再寄生在 server 命令树
	if strings.Contains(out, "lint") {
		t.Errorf("server help should not contain lint (moved to cmd/lint):\n%s", out)
	}
}

func TestLintCommandTree(t *testing.T) {
	t.Parallel()
	out := runGo(t, "run", "./cmd/lint", "--help")
	for _, name := range []string{"conv", "static"} {
		if !strings.Contains(out, name) {
			t.Errorf("lint help missing %q:\n%s", name, out)
		}
	}
	for _, forbidden := range []string{"server", "database", "object"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("lint help contains %q:\n%s", forbidden, out)
		}
	}
}

func TestClientCommandTree(t *testing.T) {
	t.Parallel()
	out := runGo(t, "run", "./cmd/client", "--help")
	for _, name := range []string{"init", "status", "model", "version", "update"} {
		if !strings.Contains(out, name) {
			t.Errorf("client help missing %q:\n%s", name, out)
		}
	}
	for _, forbidden := range []string{"trace", "server", "database", "object", "lint"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("client help contains %q:\n%s", forbidden, out)
		}
	}
}

func TestClientTraceCommandRemoved(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/client", "trace", "--help")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("aris trace should be removed, got help:\n%s", out)
	}
	if !strings.Contains(string(out), `unknown command "trace"`) {
		t.Fatalf("expected unknown command error, got:\n%s", out)
	}
}

func TestClientBuildsForSupportedPlatforms(t *testing.T) {
	t.Parallel()
	targets := []struct{ os, arch string }{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
	}
	for _, target := range targets {
		out := filepath.Join(t.TempDir(), "aris")
		cmd := exec.CommandContext(t.Context(), "go", "build",
			"-buildvcs=false", "-trimpath", "-ldflags=-s -w", "-o", out, "./cmd/client")
		cmd.Dir = repoRoot(t)
		cmd.Env = append(append([]string{}, os.Environ()...),
			"CGO_ENABLED=0",
			"GOOS="+target.os,
			"GOARCH="+target.arch,
		)
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s/%s: %v\n%s", target.os, target.arch, err, data)
		}
		info, err := os.Stat(out)
		if err != nil || info.Size() == 0 {
			t.Fatalf("build %s/%s produced empty binary", target.os, target.arch)
		}
	}
}
