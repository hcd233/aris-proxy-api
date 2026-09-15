package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

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
	// base URL 必须带 Anthropic 协议分区前缀，且不能带 /v1：Claude Code 会在 base 之后
	// 再追加 /v1/messages，缺前缀会打到不存在的 {host}/v1/messages，多一段则打到
	// {host}/api/anthropic/v1/v1/messages（均实测 404）。
	var cfg map[string]any
	if err := sonic.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	env, ok := cfg["env"].(map[string]any)
	if !ok {
		t.Fatalf("settings.json missing env block:\n%s", s)
	}
	if got := env["ANTHROPIC_BASE_URL"]; got != "https://aris.example.com/api/anthropic" {
		t.Fatalf("ANTHROPIC_BASE_URL must be host + /api/anthropic (Claude Code appends /v1/messages), got %v", got)
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
	for _, want := range []string{
		`[model_providers."aris-proxy"]`,
		`wire_api = "responses"`,
		`experimental_bearer_token`,
		`model_context_window`,
		// base URL 必须带 OpenAI 协议分区前缀：Codex 会在 base 之后追加 /responses，
		// 缺前缀会打到不存在的 {host}/responses。
		`base_url = "https://aris.example.com/api/openai/v1"`,
	} {
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

func TestCodexWrite_RepairsNestedRootKeysAndDuplicatedMemories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// 回放故障现场：root 键被旧版写进了 [model_providers."deepseek"] 段，[memories] 表头逐次导出累积。
	// Codex 读取时报 `duplicate key`，整份配置不可用。
	broken := strings.Join([]string{
		`notify = ["x"]`,
		`model_reasoning_effort = "max"`,
		"",
		"[features]",
		"memories = true",
		"",
		`[model_providers."deepseek"]`,
		`name = "DeepSeek"`,
		`model = "gpt-4o"`,
		`model_provider = "stale-provider"`,
		"",
		"[memories]",
		"[memories]",
		"generate_memories = true",
		`extract_model = "gpt-4o"`,
		`consolidation_model = "gpt-4o"`,
		"",
		`[model_providers."aris-proxy"]`,
		`name = "Old"`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}

	target := model.CodexTarget{}
	outputs := make([]string, 0, 3)
	for range 3 { // 反复导出必须稳定：旧实现每导一次就多一个 [memories] 表头和一个 root 块
		if err := target.Write(path, "https://aris.example.com", "sk-test", fixtureModels[:1]); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, string(data))
	}
	for i, got := range outputs[1:] {
		if got != outputs[0] {
			t.Fatalf("write #%d differs from #1:\n--- #1 ---\n%s\n--- #%d ---\n%s", i+2, outputs[0], i+2, got)
		}
	}

	got := outputs[0]
	if n := strings.Count(got, "[memories]"); n != 1 {
		t.Fatalf("expected exactly one [memories] table, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, `[model_providers."aris-proxy"]`); n != 1 {
		t.Fatalf("expected exactly one aris-proxy provider block, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, `name = "DeepSeek"`) || !strings.Contains(got, "generate_memories = true") {
		t.Fatalf("unrelated config must be preserved:\n%s", got)
	}
	// root 键必须唯一且位于首个表头之前，否则 TOML 会把它们归入上一个表（旧实现正是落进了 provider 段）
	lines := strings.Split(got, "\n")
	firstTable := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			firstTable = i
			break
		}
	}
	for _, key := range []string{"model", "model_provider", "model_context_window"} {
		n := 0
		for i, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), key+" = ") {
				continue
			}
			n++
			if i > firstTable {
				t.Fatalf("root key %s must precede the first table header (line %d):\n%s", key, i, got)
			}
		}
		if n != 1 {
			t.Fatalf("expected exactly one root key %s, got %d:\n%s", key, n, got)
		}
	}
}
