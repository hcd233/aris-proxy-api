package trace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/client/trace"
)

const testBinPath = "/home/u/.aris/bin/aris"
const testHookCommand = testBinPath + " trace ingest --agent codex"

func writeHooksFile(t *testing.T, paths trace.Paths, content string) {
	t.Helper()
	if err := os.MkdirAll(paths.CodexDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CodexHooksFile(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInstallCodexHooksOnMissingFile(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}

	registered, err := trace.InstallCodexHooks(paths, testBinPath)
	if err != nil {
		t.Fatalf("InstallCodexHooks error: %v", err)
	}
	if registered != 10 {
		t.Fatalf("registered = %d, want 10", registered)
	}

	data, err := os.ReadFile(paths.CodexHooksFile())
	if err != nil {
		t.Fatalf("read hooks file: %v", err)
	}
	var parsed map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("hooks file is not valid JSON: %v", err)
	}
	var hooks map[string][]map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(parsed["hooks"], &hooks); err != nil {
		t.Fatalf("decode hooks field: %v", err)
	}
	if len(hooks) != 10 {
		t.Fatalf("hooks has %d events, want 10", len(hooks))
	}
	for event, groups := range hooks {
		if len(groups) != 1 {
			t.Fatalf("event %s has %d groups, want 1", event, len(groups))
		}
		if !strings.Contains(string(groups[0]["hooks"]), testHookCommand) {
			t.Fatalf("event %s group missing aris command", event)
		}
	}

	info, err := os.Stat(paths.CodexHooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("hooks file mode = %o, want 600", perm)
	}
	if _, err := os.Stat(paths.CodexHooksBackupFile()); !os.IsNotExist(err) {
		t.Fatal("backup file should not exist when hooks file was missing")
	}
}

func TestInstallCodexHooksPreservesExistingAndBacksUp(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeHooksFile(t, paths, `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo hi", "timeout": 5}]}]
  },
  "otherField": {"keep": true}
}`)

	if _, err := trace.InstallCodexHooks(paths, testBinPath); err != nil {
		t.Fatalf("InstallCodexHooks error: %v", err)
	}

	data, err := os.ReadFile(paths.CodexHooksFile())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "echo hi") {
		t.Fatal("existing non-aris hook group was dropped")
	}
	if !strings.Contains(content, "otherField") {
		t.Fatal("unknown top-level field was dropped")
	}
	var parsed map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	var hooks map[string][]map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(parsed["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	if len(hooks["PreToolUse"]) != 2 {
		t.Fatalf("PreToolUse has %d groups, want 2 (existing + aris)", len(hooks["PreToolUse"]))
	}

	backup, err := os.ReadFile(paths.CodexHooksBackupFile())
	if err != nil {
		t.Fatal("backup file should exist")
	}
	if !strings.Contains(string(backup), "echo hi") {
		t.Fatal("backup should contain original content")
	}
	info, err := os.Stat(paths.CodexHooksBackupFile())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("backup file mode = %o, want 600", perm)
	}
}

func TestInstallCodexHooksReplacesOldArisEntries(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeHooksFile(t, paths, `{
  "hooks": {
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/old/path/aris trace ingest", "timeout": 30}]}]
  }
}`)

	if _, err := trace.InstallCodexHooks(paths, testBinPath); err != nil {
		t.Fatalf("InstallCodexHooks error: %v", err)
	}

	data, err := os.ReadFile(paths.CodexHooksFile())
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	var hooks map[string][]map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(parsed["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	if len(hooks["Stop"]) != 1 {
		t.Fatalf("Stop has %d groups, want 1 (old aris replaced)", len(hooks["Stop"]))
	}
	if strings.Contains(string(data), "/old/path/aris") {
		t.Fatal("old aris hook entry was not removed")
	}

	// 二次安装幂等：不产生重复 group
	if _, err := trace.InstallCodexHooks(paths, testBinPath); err != nil {
		t.Fatalf("second InstallCodexHooks error: %v", err)
	}
	data, err = os.ReadFile(paths.CodexHooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := sonic.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if err := sonic.Unmarshal(parsed["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	if len(hooks["Stop"]) != 1 {
		t.Fatalf("after second install Stop has %d groups, want 1", len(hooks["Stop"]))
	}
}

func TestInspectCodexHooks(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}

	found, missing := trace.InspectCodexHooks(paths, testBinPath)
	if found != 0 || len(missing) != 10 {
		t.Fatalf("missing file: found = %d, missing = %d, want 0/10", found, len(missing))
	}

	if _, err := trace.InstallCodexHooks(paths, testBinPath); err != nil {
		t.Fatal(err)
	}
	found, missing = trace.InspectCodexHooks(paths, testBinPath)
	if found != 10 || len(missing) != 0 {
		t.Fatalf("after install: found = %d, missing = %d, want 10/0", found, len(missing))
	}

	found, missing = trace.InspectCodexHooks(paths, "/other/bin/aris")
	if found != 0 || len(missing) != 10 {
		t.Fatalf("wrong bin path: found = %d, missing = %d, want 0/10", found, len(missing))
	}
}

func TestInspectCodexHooksPartial(t *testing.T) {
	t.Parallel()
	paths := trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	writeHooksFile(t, paths, `{
  "hooks": {
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "/home/u/.aris/bin/aris trace ingest --agent codex", "timeout": 30}]}]
  }
}`)

	found, missing := trace.InspectCodexHooks(paths, testBinPath)
	if found != 1 || len(missing) != 9 {
		t.Fatalf("found = %d, missing = %d, want 1/9", found, len(missing))
	}
	if len(missing) > 0 && missing[0] == "Stop" {
		t.Fatal("Stop should not be in missing list")
	}
	// hooks.json 所在目录必须存在
	if _, err := os.Stat(filepath.Dir(paths.CodexHooksFile())); err != nil {
		t.Fatal(err)
	}
}
