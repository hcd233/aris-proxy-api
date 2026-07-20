package tracecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookGroup struct {
	Hooks []hookCommand `json:"hooks"`
}

func TestCodexHookInstaller_PreservesExistingConfigAndIsIdempotent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	paths := client.Paths{Root: filepath.Join(home, ".aris")}
	if err := os.MkdirAll(paths.CodexDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("./fixtures/hooks_existing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CodexHooksFile(), fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	installer := client.NewCodexHookInstaller(paths)
	commandPath := "/home/user/.aris/bin/aris"
	for range 2 {
		if _, err := installer.Install(context.Background(), commandPath); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(paths.CodexHooksBackupFile()); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	data, err := os.ReadFile(paths.CodexHooksFile())
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	var future struct {
		Keep bool `json:"keep"`
	}
	if err := sonic.Unmarshal(root["future_top_level"], &future); err != nil || !future.Keep {
		t.Fatalf("unknown top-level field changed: %s, %v", root["future_top_level"], err)
	}
	var hooks map[string][]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(root["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	for _, event := range client.CodexHookEvents() {
		arisGroups := 0
		for _, raw := range hooks[event] {
			var group hookGroup
			if err := sonic.Unmarshal(raw, &group); err != nil {
				t.Fatal(err)
			}
			for _, hook := range group.Hooks {
				if hook.Command == commandPath+" trace ingest" {
					arisGroups++
					if hook.Type != "command" || hook.Timeout != 30 {
						t.Fatalf("unexpected Aris hook: %+v", hook)
					}
				}
			}
		}
		if arisGroups != 1 {
			t.Fatalf("event %s has %d Aris groups", event, arisGroups)
		}
	}
	var existing struct {
		FutureGroup string        `json:"future_group"`
		Hooks       []hookCommand `json:"hooks"`
	}
	if err := sonic.Unmarshal(hooks["PreToolUse"][0], &existing); err != nil {
		t.Fatal(err)
	}
	if existing.FutureGroup != "keep" || len(existing.Hooks) != 1 ||
		existing.Hooks[0].Command != "/usr/local/bin/other-hook" {
		t.Fatalf("existing hook data lost: %+v", existing)
	}
}
