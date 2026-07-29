package trace

import (
	"context"
	"os"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestConfigSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: t.TempDir()}
	store := trace.NewConfigStore(paths)
	want := trace.Config{Host: "https://aris.example.com", APIKey: "sk-test"}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}

	info, err := os.Stat(paths.ConfigFile())
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file mode = %o, want 600", perm)
	}
}
