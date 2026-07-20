package tracecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

func TestConfigStore_SaveUsesPrivatePermissions(t *testing.T) {
	t.Parallel()
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	store := client.NewConfigStore(paths)
	want := client.Config{Host: "https://example.com", Agent: "codex", APIKey: "secret"}
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, paths.TraceDir(), 0o700)
	assertPerm(t, paths.ConfigFile(), 0o600)
	got, err := store.Load(context.Background())
	if err != nil || got != want {
		t.Fatalf("load = %+v, %v", got, err)
	}
	entries, err := os.ReadDir(paths.TraceDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(paths.ConfigFile()) {
		t.Fatalf("unexpected trace files: %+v", entries)
	}
}

func assertPerm(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != expected {
		t.Fatalf("%s mode = %o, want %o", path, got, expected)
	}
}
