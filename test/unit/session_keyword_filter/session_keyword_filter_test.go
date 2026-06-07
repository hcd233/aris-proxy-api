// Package session_keyword_filter 验证 session 列表 keyword 过滤 SQL 片段的关键形态。
//
// 历史背景：
//
//   - fix #58：sessions.message_ids 在 gorm 中通过 serializer:json 存储为 JSON 文本，
//     PostgreSQL 里既不是 int[] 也不是 jsonb（需要 ::jsonb 强转后才能用 JSONB 函数）。
//     旧实现直接用 messages.id = ANY(sessions.message_ids) 触发 SQLSTATE 42809。
//
//   - fix #59：把 sessions.message_ids::jsonb ? messages.id::text 写成裸 '?' 顶层包含
//     操作符会与 gorm 占位符撞车（SQLSTATE 42601），改用 jsonb_exists() 函数形式规避。
//
//   - refactor/session-list-keyword-perf-2026-06-07：旧实现用 EXISTS + jsonb_exists 强相关
//     子查询，planner 只能为每条候选 session 在 messages 全表上重跑 ILIKE 顺序扫描，
//     线上 keyword 检索接近 O(sessions × messages) + 还要再跑一次 COUNT(*)。
//     现在把方向反过来：jsonb_array_elements_text 把 sessions.message_ids 展开成 K 个 ID，
//     按 PK 回查 messages，仅对这 K 行做 ILIKE。复杂度变成 O(sessions × K)，K << M。
//
// 本文件用单元测试把"新形态"和"老雷区"都钉死，避免重构时被回退。
package session_keyword_filter

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestSessionKeywordFilterSQL_UsesPKJoinShape 断言新形态：
// 必须以 jsonb_array_elements_text 展开 message_ids 并按 messages.id 主键回查，
// 这样 planner 才能避开"为每条 session 在 messages 全表跑 ILIKE 顺序扫描"。
//
//	@author centonhuang
//	@update 2026-06-07 21:20:00
func TestSessionKeywordFilterSQL_UsesPKJoinShape(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if fragment == "" {
		t.Fatal("SessionKeywordFilterSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "::jsonb") {
		t.Errorf("SessionKeywordFilterSQL must cast message_ids to jsonb, got %q", fragment)
	}
	if !strings.Contains(fragment, "jsonb_array_elements_text(") {
		t.Errorf("SessionKeywordFilterSQL must expand sessions.message_ids via jsonb_array_elements_text() so messages can be PK-joined instead of running a correlated ILIKE per session, got %q", fragment)
	}
	if !strings.Contains(fragment, "messages.id =") {
		t.Errorf("SessionKeywordFilterSQL must PK-join messages on messages.id (so the planner uses the primary key), got %q", fragment)
	}
	if !strings.Contains(fragment, "ILIKE ?") {
		t.Errorf("SessionKeywordFilterSQL must keep ILIKE ? parameter placeholder, got %q", fragment)
	}
}

// TestSessionKeywordFilterSQL_AvoidsLandmines 维护历史 bugfix 的护栏：
//
//   - 不能用 ANY(sessions.message_ids)：message_ids 在 PG 里是 jsonb 文本不是原生数组（fix #58）
//
//   - 整段 SQL 必须只有 1 个 '?'（gorm 占位符），不能再出现裸 '?' jsonb 顶层操作符（fix #59）
//
//   - 不能让 ILIKE 与 sessions.message_ids 形成强相关（refactor 2026-06-07）
//
//     @author centonhuang
//     @update 2026-06-07 21:20:00
func TestSessionKeywordFilterSQL_AvoidsLandmines(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if strings.Contains(fragment, "ANY(sessions.message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use native array ANY(sessions.message_ids), got %q", fragment)
	}
	if strings.Contains(fragment, "ANY(message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use ANY against raw message_ids column, got %q", fragment)
	}

	// 占位符只允许 1 个（ILIKE ?）。多于 1 个意味着引入了裸 '?' 操作符，
	// gorm 会把它当成参数槽，回到 fix #59 的报错路径（SQLSTATE 42601）。
	placeholderCount := strings.Count(fragment, "?")
	if placeholderCount != 1 {
		t.Errorf("SessionKeywordFilterSQL must contain exactly one ? placeholder (for ILIKE), got %d in %q", placeholderCount, fragment)
	}

	// jsonb_exists 是历史实现，强相关到 sessions，导致 planner 退化为每条 session
	// 在 messages 全表跑一次 ILIKE。新实现走 jsonb_array_elements_text + PK join。
	if strings.Contains(fragment, "jsonb_exists(") {
		t.Errorf("SessionKeywordFilterSQL must not couple ILIKE to a per-session jsonb_exists subquery; expand message_ids via jsonb_array_elements_text and PK-join messages instead, got %q", fragment)
	}
}
