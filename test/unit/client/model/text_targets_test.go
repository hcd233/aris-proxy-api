package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/model"
)

func TestClaudeCodeWrite_EnvAndOneMSuffix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	target := model.ClaudeCodeTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "sonnet[1m]"} {
		if !strings.Contains(s, want) {
			t.Fatalf("claude settings.json missing %s:\n%s", want, s)
		}
	}
	// gpt-4o context=128K 不加 [1m]
	if strings.Contains(s, "gpt-4o[1m]") {
		t.Fatalf("1M suffix must only apply to >=1M context models:\n%s", s)
	}
}

func TestClaudeCodeWrite_MergesExistingEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := `{"permissions":{"allow":["Bash"]},"env":{"ANTROPIC_BASE_URL":"https://old"}}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	target := model.ClaudeCodeTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, `"allow"`) || !strings.Contains(s, "aris.example.com") {
		t.Fatalf("existing settings must be preserved with env merged:\n%s", s)
	}
}

func TestCodexWrite_RootAndProviderBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	stale := "model = \"old-model\"\nmodel_provider = \"old\"\n\n[model_providers.\"aris-proxy\"]\nname = \"Old\"\nbase_url = \"https://old\"\n"
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	target := model.CodexTarget{}
	if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels[:1]); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`[model_providers."aris-proxy"]`, `wire_api = "responses"`, `experimental_bearer_token`, `model_context_window`} {
		if !strings.Contains(s, want) {
			t.Fatalf("codex config.toml missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, `model = "old-model"`) || strings.Contains(s, `model_provider = "old"`) {
		t.Fatalf("codex stale root model keys not cleaned:\n%s", s)
	}
	// 旧同名 provider 段应被清理
	count := strings.Count(s, `[model_providers."aris-proxy"]`)
	if count != 1 {
		t.Fatalf("expected exactly one aris-proxy provider block, got %d:\n%s", count, s)
	}
}
