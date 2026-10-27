// Package agent_harness 钉住 agent harness 的 UA 前缀匹配与规则校验语义。
package agent_harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

// ruleSet 构造一组按 (Sort, ID) 升序排列的规则：
// sort=5 的窄前缀先命中，sort=10/20 的宽前缀兜底。
func ruleSet() []agentharness.Rule {
	return []agentharness.Rule{
		{ID: 3, Pattern: "deepseek-harness/0.2.1-rc.2", Name: "DSH-RC2", Sort: 5},
		{ID: 2, Pattern: "zcode/", Name: "ZCode", Sort: 10},
		{ID: 1, Pattern: "deepseek-harness/", Name: "DSH", Sort: 20},
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		ua       string
		expected string
	}{
		{"sort priority wins", "deepseek-harness/0.2.1-rc.2 (+https://github.com/deepseek-ai/deepseek-harness)", "DSH-RC2"},
		{"broader prefix fallback", "deepseek-harness/0.2.1-rc.3 (+https://github.com/deepseek-ai/deepseek-harness)", "DSH"},
		{"case insensitive", "ZCode/3.15.3 ai-sdk/provider-utils/4.0.39", "ZCode"},
		{"tolerates surrounding spaces", "  zcode/3.14.5", "ZCode"},
		{"unknown ua", "OpenAI/Python 2.24.0", "Other"},
		{"empty ua", "", "Other"},
		{"blank ua", "   ", "Other"},
	}

	rules := ruleSet()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := agentharness.Match(c.ua, rules); got != c.expected {
				t.Errorf("Match(%q) = %q, want %q", c.ua, got, c.expected)
			}
		})
	}
}

func TestMatchWithoutRules(t *testing.T) {
	t.Parallel()

	if got := agentharness.Match("zcode/3.15.3", nil); got != constant.AgentHarnessOther {
		t.Errorf("Match without rules = %q, want %q", got, constant.AgentHarnessOther)
	}
}

func TestNormalizePattern(t *testing.T) {
	t.Parallel()

	t.Run("trims and lowercases", func(t *testing.T) {
		t.Parallel()

		got, err := agentharness.NormalizePattern("  ZCode/ ")
		if err != nil {
			t.Fatalf("NormalizePattern returned error: %v", err)
		}
		if got != "zcode/" {
			t.Errorf("NormalizePattern = %q, want %q", got, "zcode/")
		}
	})

	t.Run("keeps inner space prefix", func(t *testing.T) {
		t.Parallel()

		got, err := agentharness.NormalizePattern("pi (")
		if err != nil {
			t.Fatalf("NormalizePattern returned error: %v", err)
		}
		if got != "pi (" {
			t.Errorf("NormalizePattern = %q, want %q", got, "pi (")
		}
	})

	invalid := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"percent wildcard", "a%b"},
		{"underscore wildcard", "a_b"},
		{"backslash escape", `a\b`},
		{"too long", strings.Repeat("x", 201)},
	}
	for _, c := range invalid {
		t.Run("rejects "+c.name, func(t *testing.T) {
			t.Parallel()

			if _, err := agentharness.NormalizePattern(c.input); !errors.Is(err, ierr.ErrValidation) {
				t.Errorf("NormalizePattern(%q) err = %v, want ErrValidation", c.input, err)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	t.Parallel()

	t.Run("keeps display case", func(t *testing.T) {
		t.Parallel()

		got, err := agentharness.NormalizeName("Codex")
		if err != nil {
			t.Fatalf("NormalizeName returned error: %v", err)
		}
		if got != "Codex" {
			t.Errorf("NormalizeName = %q, want %q", got, "Codex")
		}
	})

	t.Run("trims", func(t *testing.T) {
		t.Parallel()

		got, err := agentharness.NormalizeName("  DSH  ")
		if err != nil {
			t.Fatalf("NormalizeName returned error: %v", err)
		}
		if got != "DSH" {
			t.Errorf("NormalizeName = %q, want %q", got, "DSH")
		}
	})

	invalid := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"reserved other", "other"},
		{"reserved other upper", "OTHER"},
		{"too long", strings.Repeat("x", 65)},
	}
	for _, c := range invalid {
		t.Run("rejects "+c.name, func(t *testing.T) {
			t.Parallel()

			if _, err := agentharness.NormalizeName(c.input); !errors.Is(err, ierr.ErrValidation) {
				t.Errorf("NormalizeName(%q) err = %v, want ErrValidation", c.input, err)
			}
		})
	}
}
