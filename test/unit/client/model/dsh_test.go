package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/hcd233/aris-proxy-api/internal/client/model"
)

// dshPatchFixture 模拟用户手工维护的 dsh profile patch：含注释、无关条目与 aris provider
var dshPatchFixture = `# dsh profile patch layer
- id: agent-default-model
  name: "@deepseek-ai/dsh-agent-default-model"
  config:
    provider: aris
    model: gpt-4o

- id: llm-pi-ai
  name: "@deepseek-ai/dsh-llm-pi-ai"
  config:
    providers:
      other:
        displayName: Other
        api: openai-completions
        models:
          - id: keep-me
      aris:
        displayName: Aris
        apiKeyEnv: ARIS_API_KEY
        api: openai-completions
        baseURL: https://old.example.com/api/openai/v1
        models:
          - id: gpt-4o
            name: GPT-4o
            contextWindow: 1
            maxTokens: 1
            input:
              - text
            reasoningEfforts:
              off: null
              max: max
          - id: stale-model
            name: Stale

# mcp servers
- insert:
    - id: mcp-demo
      name: '@deepseek-ai/dsh-mcp-client'
`

var dshCredentialsFixture = `version: 1
records:
  demo/record:
    kind: api-key
    payload:
      secret: abc
refs:
  OTHER_KEY: other-value
  ARIS_API_KEY: old-value
`

// writeTempFile 写入测试用文件并返回路径
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readFileString 读取文件内容
func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// dshEntries 解析 patch 文件为条目列表
func dshEntries(t *testing.T, content string) []map[string]any {
	t.Helper()
	var entries []map[string]any
	if err := yaml.Unmarshal([]byte(content), &entries); err != nil {
		t.Fatalf("patch is not valid yaml: %v\n%s", err, content)
	}
	return entries
}

// findDSHEntry 按 id 取条目
func findDSHEntry(t *testing.T, entries []map[string]any, id string) map[string]any {
	t.Helper()
	for _, entry := range entries {
		if entry["id"] == id {
			return entry
		}
	}
	t.Fatalf("entry %q not found", id)
	return nil
}

// dshProvider 取 llm-pi-ai 条目下 providers.<id>
func dshProvider(t *testing.T, entries []map[string]any, providerID string) map[string]any {
	t.Helper()
	entry := findDSHEntry(t, entries, "llm-pi-ai")
	cfg, _ := entry["config"].(map[string]any)
	providers, _ := cfg["providers"].(map[string]any)
	provider, ok := providers[providerID].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found", providerID)
	}
	return provider
}

// dshModelByID 在 provider.models 中按 id 取模型
func dshModelByID(t *testing.T, provider map[string]any, id string) map[string]any {
	t.Helper()
	rawModels, _ := provider["models"].([]any)
	for _, raw := range rawModels {
		entry, _ := raw.(map[string]any)
		if entry["id"] == id {
			return entry
		}
	}
	return nil
}

func TestDSHWrite_CreatesEntryAndCredential(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := filepath.Join(home, ".dsh", "profiles", "desktop", "cordis.patch.yml")
	credentialsPath := filepath.Join(home, ".dsh", ".credentials.yaml")
	if err := os.MkdirAll(filepath.Dir(patchPath), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := (model.DSHTarget{}).Write(patchPath, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}

	provider := dshProvider(t, dshEntries(t, readFileString(t, patchPath)), "aris")
	if got := provider["baseURL"]; got != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("provider baseURL must carry the openai proxy prefix, got %v", got)
	}
	if got := provider["api"]; got != "openai-completions" {
		t.Fatalf("provider api must be openai-completions, got %v", got)
	}
	if got := provider["apiKeyEnv"]; got != "ARIS_API_KEY" {
		t.Fatalf("provider must reference the credential by name, got %v", got)
	}
	spec := dshModelByID(t, provider, "gpt-4o")
	if spec == nil {
		t.Fatalf("model gpt-4o missing: %v", provider["models"])
	}
	if got := spec["contextWindow"]; got != 128000 {
		t.Fatalf("contextWindow = %v, want 128000", got)
	}
	if got := spec["maxTokens"]; got != 16384 {
		t.Fatalf("maxTokens = %v, want 16384", got)
	}

	credentials := readFileString(t, credentialsPath)
	if !strings.Contains(credentials, "ARIS_API_KEY: sk-test") {
		t.Fatalf("credentials must carry the api key, got:\n%s", credentials)
	}
}

func TestDSHWrite_PreservesUnrelatedContent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := writeTempFile(t, filepath.Join(home, ".dsh", "profiles", "desktop"), "cordis.patch.yml", dshPatchFixture)
	writeTempFile(t, filepath.Join(home, ".dsh"), ".credentials.yaml", dshCredentialsFixture)

	if err := (model.DSHTarget{}).Write(patchPath, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	content := readFileString(t, patchPath)
	for _, want := range []string{
		"# dsh profile patch layer",
		"# mcp servers",
		"provider: aris",
		"model: gpt-4o",
		"keep-me",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("patch lost %q:\n%q", want, content)
		}
	}

	entries := dshEntries(t, content)
	if got := findDSHEntry(t, entries, "agent-default-model"); got["id"] != "agent-default-model" {
		t.Fatalf("agent-default-model entry damaged: %v", got)
	}
	other := dshProvider(t, entries, "other")
	if got := dshModelByID(t, other, "keep-me"); got == nil {
		t.Fatalf("unmanaged provider models must be preserved: %v", other)
	}

	provider := dshProvider(t, entries, "aris")
	if got := dshModelByID(t, provider, "stale-model"); got != nil {
		t.Fatalf("deselected model must be dropped, got %v", got)
	}
	if got := dshModelByID(t, provider, "sonnet"); got == nil {
		t.Fatalf("newly selected model missing: %v", provider["models"])
	}
}

func TestDSHWrite_PreservesHandEditedModelFields(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := writeTempFile(t, filepath.Join(home, ".dsh", "profiles", "desktop"), "cordis.patch.yml", dshPatchFixture)
	writeTempFile(t, filepath.Join(home, ".dsh"), ".credentials.yaml", dshCredentialsFixture)

	if err := (model.DSHTarget{}).Write(patchPath, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}

	provider := dshProvider(t, dshEntries(t, readFileString(t, patchPath)), "aris")
	spec := dshModelByID(t, provider, "gpt-4o")
	if got := spec["name"]; got != "GPT-4o" {
		t.Fatalf("hand-edited name must survive re-export, got %v", got)
	}
	if got := spec["contextWindow"]; got != 128000 {
		t.Fatalf("contextWindow must be refreshed, got %v", got)
	}
	efforts, ok := spec["reasoningEfforts"].(map[string]any)
	if !ok || len(efforts) != 2 {
		t.Fatalf("reasoningEfforts must survive re-export, got %v", spec["reasoningEfforts"])
	}
}

func TestDSHWrite_UpdatesCredentialRef(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := writeTempFile(t, filepath.Join(home, ".dsh", "profiles", "desktop"), "cordis.patch.yml", dshPatchFixture)
	credentialsPath := writeTempFile(t, filepath.Join(home, ".dsh"), ".credentials.yaml", dshCredentialsFixture)

	if err := (model.DSHTarget{}).Write(patchPath, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}

	var store map[string]any
	if err := yaml.Unmarshal([]byte(readFileString(t, credentialsPath)), &store); err != nil {
		t.Fatalf("credentials is not valid yaml: %v", err)
	}
	refs, _ := store["refs"].(map[string]any)
	if got := refs["ARIS_API_KEY"]; got != "sk-test" {
		t.Fatalf("refs.ARIS_API_KEY = %v, want sk-test", got)
	}
	if got := refs["OTHER_KEY"]; got != "other-value" {
		t.Fatalf("unrelated ref lost, got %v", got)
	}
	if _, ok := store["records"].(map[string]any); !ok {
		t.Fatalf("credential records must be preserved: %v", store)
	}
}

func TestDSHConfigPath_PrefersDesktopProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"desktop", "work"} {
		if err := os.MkdirAll(filepath.Join(dir, ".dsh", "profiles", name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(dir, ".dsh", "profiles", "desktop", "cordis.patch.yml")
	if got := (model.DSHTarget{}).ConfigPath(dir); got != want {
		t.Fatalf("ConfigPath = %q, want %q", got, want)
	}
}

func TestDSHConfigPath_FallsBackToSingleProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".dsh", "profiles", "laptop"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".dsh", "profiles", "laptop", "cordis.patch.yml")
	if got := (model.DSHTarget{}).ConfigPath(dir); got != want {
		t.Fatalf("ConfigPath = %q, want %q", got, want)
	}
}

func TestDSHWrite_IsIdempotent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := writeTempFile(t, filepath.Join(home, ".dsh", "profiles", "desktop"), "cordis.patch.yml", dshPatchFixture)
	credentialsPath := writeTempFile(t, filepath.Join(home, ".dsh"), ".credentials.yaml", dshCredentialsFixture)

	target := model.DSHTarget{}
	for range 2 {
		if err := target.Write(patchPath, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
			t.Fatal(err)
		}
	}

	entries := dshEntries(t, readFileString(t, patchPath))
	count := 0
	for _, entry := range entries {
		if entry["id"] == "llm-pi-ai" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("second export duplicated the llm-pi-ai entry: %d", count)
	}
	if got := strings.Count(readFileString(t, credentialsPath), "ARIS_API_KEY"); got != 1 {
		t.Fatalf("second export duplicated the credential ref: %d", got)
	}
	if _, err := os.Stat(patchPath + ".bak"); err != nil {
		t.Fatalf("re-export must keep a backup: %v", err)
	}
}

func TestDSHWrite_FallsBackToDefaultLimits(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	patchPath := filepath.Join(home, ".dsh", "profiles", "desktop", "cordis.patch.yml")
	if err := os.MkdirAll(filepath.Dir(patchPath), 0o700); err != nil {
		t.Fatal(err)
	}
	models := []model.TargetModel{{Alias: "bare", Capabilities: []string{"text"}}}
	if err := (model.DSHTarget{}).Write(patchPath, "https://aris.example.com", "sk-test", models); err != nil {
		t.Fatal(err)
	}

	spec := dshModelByID(t, dshProvider(t, dshEntries(t, readFileString(t, patchPath)), "aris"), "bare")
	if got := spec["contextWindow"]; got != 128000 {
		t.Fatalf("contextWindow fallback = %v, want 128000", got)
	}
	if got := spec["maxTokens"]; got != 16384 {
		t.Fatalf("maxTokens fallback = %v, want 16384", got)
	}
}
