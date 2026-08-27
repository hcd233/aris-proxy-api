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
	if provider["baseUrl"] != "https://aris.example.com" {
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
	// 既有 provider 保留原 baseUrl，不覆盖
	if provider["baseUrl"] != "https://old.example.com" {
		t.Fatalf("existing provider baseUrl should be preserved: %v", provider["baseUrl"])
	}
	models := provider["models"].([]any)
	if len(models) != len(fixtureModels)+1 {
		t.Fatalf("expected existing + new models, got %d:\n%s", len(models), data)
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
