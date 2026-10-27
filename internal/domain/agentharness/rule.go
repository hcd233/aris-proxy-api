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
// （例如 Codex 的 CLI 与 Desktop 两种 UA）。
type Rule struct {
	ID      uint
	Pattern string
	Name    string
	Sort    int
}

// Match 返回 UA 命中的 harness 名称。
//
// rules 必须已按 (Sort, ID) 升序排列，函数按该顺序取第一条前缀命中项；
// ua 为空、仅有空白或全部不命中时返回 Other。
func Match(ua string, rules []Rule) string {
	normalized := strings.ToLower(strings.TrimSpace(ua))
	if normalized == "" {
		return constant.AgentHarnessOther
	}
	for _, rule := range rules {
		if strings.HasPrefix(normalized, rule.Pattern) {
			return rule.Name
		}
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
