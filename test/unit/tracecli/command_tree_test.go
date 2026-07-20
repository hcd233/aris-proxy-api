package tracecli

import (
	"context"
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
	cmd := exec.CommandContext(context.Background(), "go", args...)
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
	for _, name := range []string{"server", "database", "object", "lint"} {
		if !strings.Contains(out, name) {
			t.Errorf("help missing %q:\n%s", name, out)
		}
	}
}

func TestClientCommandTreeContainsOnlyTrace(t *testing.T) {
	t.Parallel()
	out := runGo(t, "run", "./cmd/client", "--help")
	if !strings.Contains(out, "trace") {
		t.Fatalf("client help missing trace:\n%s", out)
	}
	for _, forbidden := range []string{"server", "database", "object", "lint"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("client help contains %q:\n%s", forbidden, out)
		}
	}
	traceOut := runGo(t, "run", "./cmd/client", "trace", "--help")
	for _, name := range []string{"init", "ingest"} {
		if !strings.Contains(traceOut, name) {
			t.Errorf("trace help missing %q:\n%s", name, traceOut)
		}
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
		cmd := exec.CommandContext(context.Background(), "go", "build",
			"-trimpath", "-ldflags=-s -w", "-o", out, "./cmd/client")
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
