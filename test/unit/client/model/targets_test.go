package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	s := string(data)
	for _, want := range []string{"contextWindow", "maxTokens", "reasoning"} {
		if !strings.Contains(s, want) {
			t.Fatalf("pi models.json missing %s:\n%s", want, s)
		}
	}
}
