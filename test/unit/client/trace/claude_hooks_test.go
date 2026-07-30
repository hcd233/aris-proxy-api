package trace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/client/trace"
)

const testClaudeHookCommand = testBinPath + " trace ingest --agent claude"

func writeClaudeSettingsFile(t *testing.T, paths trace.Paths, content string) {
	t.Helper()
	if err := os.MkdirAll(paths.ClaudeDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ClaudeSettingsFile(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readClaudeHooks(t *testing.T, paths trace.Paths) map[string][]map[string]sonic.NoCopyRawMessage {
	t.Helper()
	data, err := os.ReadFile(paths.ClaudeSettingsFile())
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	root := map[string]sonic.NoCopyRawMessage{}
	if err := sonic.Unmarshal(data, &root); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}
	hooks := map[string][]map[string]sonic.NoCopyRawMessage{}
	if err := sonic.Unmarshal(root["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks field: %v", err)
	}
	return hooks
}

func TestInstallClaudeHooksOnMissingFile(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}

	registered, err := trace.InstallClaudeHooks(paths, testBinPath)
	if err != nil {
		t.Fatalf("InstallClaudeHooks error: %v", err)
	}
	if registered != 11 {
		t.Fatalf("registered = %d, want 11", registered)
	}

	hooks := readClaudeHooks(t, paths)
	if len(hooks) != 11 {
		t.Fatalf("hooks has %d events, want 11", len(hooks))
	}
	for event, groups := range hooks {
		if len(groups) != 1 {
			t.Fatalf("event %s has %d groups, want 1", event, len(groups))
		}
		if !strings.Contains(string(groups[0]["hooks"]), testClaudeHookCommand) {
			t.Fatalf("event %s group missing aris command", event)
		}
	}
	for _, event := range []string{"PostToolUseFailure", "SessionEnd"} {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("claude event %s must be registered", event)
		}
	}

	info, err := os.Stat(paths.ClaudeSettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("settings file mode = %o, want 600", perm)
	}
	if _, err := os.Stat(paths.ClaudeSettingsBackupFile()); !os.IsNotExist(err) {
		t.Fatal("backup file should not exist when settings file was missing")
	}
}

func TestInstallClaudeHooksPreservesExistingAndBacksUp(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeClaudeSettingsFile(t, paths, `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo hi", "timeout": 5}]}]
  },
  "model": "sonnet",
  "env": {"FOO": "bar"}
}`)

	if _, err := trace.InstallClaudeHooks(paths, testBinPath); err != nil {
		t.Fatalf("InstallClaudeHooks error: %v", err)
	}

	data, err := os.ReadFile(paths.ClaudeSettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "echo hi") {
		t.Fatal("existing non-aris hook group was dropped")
	}
	if !strings.Contains(content, `"model"`) || !strings.Contains(content, `"env"`) {
		t.Fatal("existing top-level settings were dropped")
	}
	hooks := readClaudeHooks(t, paths)
	if len(hooks["PreToolUse"]) != 2 {
		t.Fatalf("PreToolUse has %d groups, want 2 (existing + aris)", len(hooks["PreToolUse"]))
	}

	backup, err := os.ReadFile(paths.ClaudeSettingsBackupFile())
	if err != nil {
		t.Fatal("backup file should exist")
	}
	if !strings.Contains(string(backup), "echo hi") {
		t.Fatal("backup should contain original content")
	}
	info, err := os.Stat(paths.ClaudeSettingsBackupFile())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("backup file mode = %o, want 600", perm)
	}
}

func TestInstallClaudeHooksReplacesOldArisEntries(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeClaudeSettingsFile(t, paths, `{
  "hooks": {
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/old/path/aris trace ingest --agent claude", "timeout": 30}]}]
  }
}`)

	if _, err := trace.InstallClaudeHooks(paths, testBinPath); err != nil {
		t.Fatalf("InstallClaudeHooks error: %v", err)
	}

	hooks := readClaudeHooks(t, paths)
	if len(hooks["Stop"]) != 1 {
		t.Fatalf("Stop has %d groups, want 1 (old aris replaced)", len(hooks["Stop"]))
	}
	data, err := os.ReadFile(paths.ClaudeSettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/old/path/aris") {
		t.Fatal("old aris hook entry was not removed")
	}

	// 二次安装幂等：不产生重复 group
	if _, err := trace.InstallClaudeHooks(paths, testBinPath); err != nil {
		t.Fatalf("second InstallClaudeHooks error: %v", err)
	}
	hooks = readClaudeHooks(t, paths)
	if len(hooks["Stop"]) != 1 {
		t.Fatalf("after second install Stop has %d groups, want 1", len(hooks["Stop"]))
	}
}

func TestInstallClaudeHooksRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeClaudeSettingsFile(t, paths, `{broken`)

	if _, err := trace.InstallClaudeHooks(paths, testBinPath); err == nil {
		t.Fatal("invalid JSON must abort install")
	}
	data, err := os.ReadFile(paths.ClaudeSettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{broken` {
		t.Fatal("original file must be kept intact on parse failure")
	}
}

func TestInspectClaudeHooks(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}

	found, missing := trace.InspectClaudeHooks(paths, testBinPath)
	if found != 0 || len(missing) != 11 {
		t.Fatalf("missing file: found = %d, missing = %d, want 0/11", found, len(missing))
	}

	if _, err := trace.InstallClaudeHooks(paths, testBinPath); err != nil {
		t.Fatal(err)
	}
	found, missing = trace.InspectClaudeHooks(paths, testBinPath)
	if found != 11 || len(missing) != 0 {
		t.Fatalf("after install: found = %d, missing = %d, want 11/0", found, len(missing))
	}

	found, missing = trace.InspectClaudeHooks(paths, "/other/bin/aris")
	if found != 0 || len(missing) != 11 {
		t.Fatalf("wrong bin path: found = %d, missing = %d, want 0/11", found, len(missing))
	}
}
