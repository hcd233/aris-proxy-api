package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/client/model"
)

var fixtureModels = []model.TargetModel{
	{Alias: "gpt-4o", UpstreamModel: "gpt-4o-2024", ContextLength: 128000, MaxOutputTokens: 16384, Capabilities: []string{"text", "image"}},
	{Alias: "sonnet", UpstreamModel: "claude-sonnet", ContextLength: 1000000, MaxOutputTokens: 64000, Capabilities: []string{"text"}},
}

func TestOpenCodeWrite_CreatesAndMerges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	target := model.OpenCodeTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"@ai-sdk/openai-compatible"`, `"gpt-4o"`, `"attachment":true`, `"modalities"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("opencode.json missing %s:\n%s", want, s)
		}
	}
	// base URL 必须带 OpenAI 协议分区前缀：OpenCode 会在 base 之后追加 /chat/completions。
	var cfg map[string]any
	if err := sonic.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	providers, ok := cfg["provider"].(map[string]any)
	if !ok {
		t.Fatalf("opencode.json missing provider block:\n%s", s)
	}
	provider, ok := providers["aris-proxy"].(map[string]any)
	if !ok {
		t.Fatalf("opencode.json missing provider.aris-proxy:\n%s", s)
	}
	options, ok := provider["options"].(map[string]any)
	if !ok {
		t.Fatalf("opencode.json missing provider.aris-proxy.options:\n%s", s)
	}
	if got := options["baseURL"]; got != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("opencode provider baseURL must carry the openai proxy prefix, got %v", got)
	}
	// 幂等：二次写入不报错且产生备份
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected .bak backup after second write: %v", err)
	}
}

func TestPiWrite_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	target := model.PiTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := sonic.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	providers, ok := root["providers"].(map[string]any)
	if !ok {
		t.Fatalf("pi models.json missing top-level providers wrapper:\n%s", data)
	}
	provider, ok := providers["aris-proxy"].(map[string]any)
	if !ok {
		t.Fatalf("pi models.json missing providers.aris-proxy:\n%s", data)
	}
	if provider["baseUrl"] != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("pi provider baseUrl mismatch: %v", provider["baseUrl"])
	}
	models, ok := provider["models"].([]any)
	if !ok || len(models) != len(fixtureModels) {
		t.Fatalf("pi provider models mismatch:\n%s", data)
	}
	for _, want := range []string{"contextWindow", "maxTokens", "reasoning"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("pi models.json missing %s:\n%s", want, data)
		}
	}
}

func TestPiWrite_MergesExistingModels(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	if err := os.WriteFile(path, []byte(`{"providers":{"aris-proxy":{"baseUrl":"https://old.example.com","models":[{"id":"existing"}]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := model.PiTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := sonic.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	providers := root["providers"].(map[string]any)
	provider := providers["aris-proxy"].(map[string]any)
	// provider 已存在时 baseUrl/apiKey 仍由本工具覆盖：保留旧值会让存量配置永远停在错误地址
	if provider["baseUrl"] != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("stale provider baseUrl must be overwritten: %v", provider["baseUrl"])
	}
	if provider["apiKey"] != "sk-test" {
		t.Fatalf("provider apiKey must be refreshed: %v", provider["apiKey"])
	}
	models := provider["models"].([]any)
	if len(models) != len(fixtureModels)+1 {
		t.Fatalf("expected existing + new models, got %d:\n%s", len(models), data)
	}
}

func TestOpenCodeWrite_OverwritesStaleBaseURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	stale := `{"provider":{"aris-proxy":{"options":{"baseURL":"https://old.example.com"},"models":{"existing":{"name":"Existing"}}}}}`
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	target := model.OpenCodeTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := sonic.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	providers := cfg["provider"].(map[string]any)
	provider := providers["aris-proxy"].(map[string]any)
	options := provider["options"].(map[string]any)
	// provider 已存在时 baseURL/apiKey 仍由本工具覆盖：保留旧值会让存量配置永远停在错误地址
	if options["baseURL"] != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("stale provider baseURL must be overwritten: %v", options["baseURL"])
	}
	headers := options["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer sk-test" {
		t.Fatalf("api key header must be refreshed: %v", headers["Authorization"])
	}
	models := provider["models"].(map[string]any)
	if len(models) != len(fixtureModels)+1 {
		t.Fatalf("existing models must be preserved, got %d:\n%s", len(models), data)
	}
}

// opencode.json 里非本工具管理的顶层键与 provider 子键必须原样保留（此前只留 provider 一个键）。
func TestOpenCodeWrite_PreservesUnmanagedKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	existing := `{"$schema":"https://opencode.ai/config.json","theme":"tokyonight","mcp":{"docs":{"type":"local"}},` +
		`"provider":{"aris-proxy":{"options":{"timeout":30000,"baseURL":"https://old"},"models":{"mine":{"name":"Mine"}}},` +
		`"anthropic":{"options":{"apiKey":"user-key"}}}}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}

	target := model.OpenCodeTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels[:1]); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := sonic.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if cfg["$schema"] != "https://opencode.ai/config.json" || cfg["theme"] != "tokyonight" {
		t.Fatalf("top-level keys must be preserved:\n%s", data)
	}
	if _, ok := cfg["mcp"].(map[string]any); !ok {
		t.Fatalf("unmanaged top-level block must be preserved:\n%s", data)
	}
	providers := cfg["provider"].(map[string]any)
	if _, ok := providers["anthropic"].(map[string]any); !ok {
		t.Fatalf("other providers must be preserved:\n%s", data)
	}
	provider := providers["aris-proxy"].(map[string]any)
	options := provider["options"].(map[string]any)
	if options["timeout"] != float64(30000) {
		t.Fatalf("unmanaged provider option must be preserved: %v", options["timeout"])
	}
	if options["baseURL"] != "https://aris.example.com/api/openai/v1" {
		t.Fatalf("managed baseURL must be overwritten: %v", options["baseURL"])
	}
	if models := provider["models"].(map[string]any); len(models) != len(fixtureModels[:1])+1 {
		t.Fatalf("existing models must be merged, got %d:\n%s", len(models), data)
	}
}

func TestTargets_IncludesAllAgents(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, tg := range model.Targets() {
		keys[tg.Key()] = true
	}
	for _, want := range []string{"opencode", "pi", "codex", "claude-code"} {
		if !keys[want] {
			t.Fatalf("Targets() missing %q", want)
		}
	}
}
