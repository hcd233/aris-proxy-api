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

// ruleSet 构造一组规则：一条窄前缀 + 两条宽前缀，故意把窄前缀排在最后。
// 命中结果只由前缀长度决定，与规则在切片里的顺序无关。
func ruleSet() []agentharness.Rule {
	return []agentharness.Rule{
		{ID: 1, Pattern: "deepseek-harness/", Name: "DSH"},
		{ID: 2, Pattern: "zcode/", Name: "ZCode"},
		{ID: 3, Pattern: "deepseek-harness/0.2.1-rc.2", Name: "DSH-RC2"},
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		ua       string
		expected string
	}{
		{"longest prefix wins", "deepseek-harness/0.2.1-rc.2 (+https://github.com/deepseek-ai/deepseek-harness)", "DSH-RC2"},
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

// TestMatchIgnoresRuleOrder 钉住「规则之间没有先后关系」：同一组规则正序与
// 倒序得到完全一致的匹配结果，这样才允许规则表不存排序字段。
func TestMatchIgnoresRuleOrder(t *testing.T) {
	t.Parallel()

	rules := ruleSet()
	reversed := []agentharness.Rule{rules[2], rules[1], rules[0]}
	uas := []string{
		"deepseek-harness/0.2.1-rc.2 (+https://github.com/deepseek-ai/deepseek-harness)",
		"deepseek-harness/0.2.1-rc.3 (+https://github.com/deepseek-ai/deepseek-harness)",
		"zcode/3.15.3",
		"OpenAI/Python 2.24.0",
	}
	for _, ua := range uas {
		t.Run(ua, func(t *testing.T) {
			t.Parallel()

			forward := agentharness.Match(ua, rules)
			backward := agentharness.Match(ua, reversed)
			if forward != backward {
				t.Errorf("Match(%q) depends on rule order: forward=%q backward=%q", ua, forward, backward)
			}
		})
	}
}

// TestMatchRuleReturnsLongestRule 预览要能说出「命中了哪条」，
// 因此返回的必须是那条最长前缀规则，而不是任意一条同名的。
func TestMatchRuleReturnsLongestRule(t *testing.T) {
	t.Parallel()

	matched := agentharness.MatchRule("deepseek-harness/0.2.1-rc.2", ruleSet())
	if matched == nil || matched.ID != 3 {
		t.Fatalf("MatchRule = %+v, want rule 3 (the longest prefix)", matched)
	}

	if got := agentharness.MatchRule("OpenAI/Python 2.24.0", ruleSet()); got != nil {
		t.Errorf("MatchRule on unknown UA = %+v, want nil", got)
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
