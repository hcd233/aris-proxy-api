package config

import (
	"os"
	"path/filepath"
	"testing"

	clientconfig "github.com/hcd233/aris-proxy-api/internal/client/config"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func testPaths(t *testing.T) clientconfig.Paths {
	t.Helper()
	return clientconfig.Paths{Root: filepath.Join(t.TempDir(), constant.ArisClientRootDirName)}
}

func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)
	store := clientconfig.NewStore(paths)
	want := clientconfig.Config{Host: "https://aris.example.com", APIKey: "sk-test"}

	if err := store.Save(t.Context(), want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Load(t.Context())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Fatalf("load = %+v, want %+v", got, want)
	}
	if _, err := os.Stat(paths.ConfigFile()); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

// TestStoreLoadsLegacyTracePath 旧版把配置放在 ~/.aris/trace/config.json；
// 升级后首次 load 必须仍能读到，避免要求用户重新 init。
func TestStoreLoadsLegacyTracePath(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)
	legacy := filepath.Join(paths.Root, "trace", constant.ArisClientConfigFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte(`{"host":"https://legacy.example.com","apiKey":"sk-legacy"}`), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	got, err := clientconfig.NewStore(paths).Load(t.Context())
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	want := clientconfig.Config{Host: "https://legacy.example.com", APIKey: "sk-legacy"}
	if got != want {
		t.Fatalf("load legacy = %+v, want %+v", got, want)
	}
}
