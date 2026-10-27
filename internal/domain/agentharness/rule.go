// Package agentharness 提供「原始 User-Agent → agent harness 名称」的规则匹配与规则校验。
//
// harness 不是持久化字段：它由规则的 UA 前缀匹配现算得出，
// 因此规则变更后历史数据无需回填即可自动重标。
package agentharness

import (
	"strings"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Rule 一条「UA 前缀 → harness 名称」规则。
//
// Pattern 必须是调用方已规范化的小写前缀；同一 Name 可以有多条规则
// （例如 Codex 的 CLI 与 Desktop 两种 UA）。规则之间没有先后关系：
// 命中的是哪条由前缀长度决定，因此不带排序字段。
type Rule struct {
	ID      uint
	Pattern string
	Name    string
}

// MatchRule 返回 UA 命中的规则，取命中的最长前缀；未命中返回 nil。
//
// 两条不同 Pattern 若能命中同一 UA，其中一条必然是另一条的前缀，
// 于是长度严格可比、「最长」唯一确定归属——不存在并列，也就不需要排序。
// ua 为空或仅有空白时返回 nil。
func MatchRule(ua string, rules []Rule) *Rule {
	normalized := strings.ToLower(strings.TrimSpace(ua))
	if normalized == "" {
		return nil
	}
	var (
		matched      *Rule
		matchedRunes int
	)
	for i := range rules {
		rule := &rules[i]
		if !strings.HasPrefix(normalized, rule.Pattern) {
			continue
		}
		if runes := utf8.RuneCountInString(rule.Pattern); runes > matchedRunes {
			matched, matchedRunes = rule, runes
		}
	}
	return matched
}

// Match 返回 UA 命中的 harness 名称；未命中返回 Other。
func Match(ua string, rules []Rule) string {
	if rule := MatchRule(ua, rules); rule != nil {
		return rule.Name
	}
	return constant.AgentHarnessOther
}

// NormalizePattern 规范化并校验规则的 UA 前缀。
//
// 去首尾空白后转小写；拒绝空值、超长值以及 LIKE 通配符（% _ \），
// 因为前缀匹配最终的 SQL 形态是 `lower(client_ua) LIKE pattern || '%'`。
func NormalizePattern(raw string) (string, error) {
	pattern := strings.ToLower(strings.TrimSpace(raw))
	if pattern == "" {
		return "", ierr.New(ierr.ErrValidation, "harness rule pattern is empty")
	}
	if utf8.RuneCountInString(pattern) > constant.AgentHarnessMaxPatternLength {
		return "", ierr.Newf(ierr.ErrValidation, "harness rule pattern exceeds %d characters", constant.AgentHarnessMaxPatternLength)
	}
	if strings.ContainsAny(pattern, `%_\`) {
		return "", ierr.New(ierr.ErrValidation, `harness rule pattern must not contain % _ or \`)
	}
	return pattern, nil
}

// NormalizeName 规范化并校验规则的展示名。
//
// 仅去首尾空白，保留管理员输入的大小写；Other 是保留名（大小写不敏感）。
func NormalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ierr.New(ierr.ErrValidation, "harness rule name is empty")
	}
	if utf8.RuneCountInString(name) > constant.AgentHarnessMaxNameLength {
		return "", ierr.Newf(ierr.ErrValidation, "harness rule name exceeds %d characters", constant.AgentHarnessMaxNameLength)
	}
	if strings.EqualFold(name, constant.AgentHarnessOther) {
		return "", ierr.Newf(ierr.ErrValidation, "harness rule name %q is reserved", name)
	}
	return name, nil
}
