// Package session_keyword_filter 验证 session 列表 keyword 过滤 SQL 片段使用 JSONB 操作符，
// 而不是把 JSON 列塞进原生数组操作符 ANY(...)
//
// 回归背景（bugfix/session-keyword-jsonb-2026-06-07）：
//   - sessions.message_ids 在 gorm 中通过 serializer:json 存储为 JSON 文本，
//     PostgreSQL 里既不是 int[] 也不是 jsonb（需要 ::jsonb 强转后才能用 JSONB 函数）。
//   - 旧实现直接用 messages.id = ANY(sessions.message_ids)，
//     触发 SQLSTATE 42809 "op ANY/ALL (array) requires array on right side"，
//     表现为 GET /api/v1/session/list?keyword=xxx 全部 500（traceID ed2ade34-...）。
//   - 修复方案：抽 SessionKeywordFilterSQL 常量，使用 JSONB 顶层元素包含操作符 ?，
//     与既有的 SessionSummarySelect 中 message_ids::jsonb 用法保持一致。
package session_keyword_filter

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestSessionKeywordFilterSQL_UsesJSONBOperator(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if fragment == "" {
		t.Fatal("SessionKeywordFilterSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "::jsonb") {
		t.Errorf("SessionKeywordFilterSQL must cast message_ids to jsonb, got %q", fragment)
	}
	if !strings.Contains(fragment, "jsonb_exists(") {
		t.Errorf("SessionKeywordFilterSQL must use jsonb_exists() (avoiding bare '?' which collides with gorm placeholders), got %q", fragment)
	}
	if !strings.Contains(fragment, "messages.id::text") {
		t.Errorf("SessionKeywordFilterSQL must reference messages.id::text, got %q", fragment)
	}
	if !strings.Contains(fragment, "ILIKE ?") {
		t.Errorf("SessionKeywordFilterSQL must keep ILIKE ? parameter placeholder, got %q", fragment)
	}
}

func TestSessionKeywordFilterSQL_DoesNotUseNativeArrayANY(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if strings.Contains(fragment, "ANY(sessions.message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use native array ANY(sessions.message_ids), got %q", fragment)
	}
	if strings.Contains(fragment, "ANY(message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use ANY against raw message_ids column, got %q", fragment)
	}
	// Bare '?' outside of the ILIKE placeholder would collide with gorm's ?-placeholder syntax
	// and get treated as a parameter slot (which is what caused the SQLSTATE 42601 regression
	// on the first deploy — see trace ec09dbee-1a34-401f-a2a0-d67678067da4).
	placeholderCount := strings.Count(fragment, "?")
	if placeholderCount != 1 {
		t.Errorf("SessionKeywordFilterSQL must contain exactly one ? placeholder (for ILIKE), got %d in %q", placeholderCount, fragment)
	}
}
