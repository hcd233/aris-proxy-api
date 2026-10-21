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

// TestSessionKeywordFilterSQL_UsesAnyArrayShape 断言 2026-10-15 形态：
// 第二段过滤必须以 = ANY(?::bigint[]) 接收应用层传入的常量 id 数组。
//
// 历史教训：EXISTS(SRF + IN 子查询) 会被 planner 建成 correlated SubPlan，
// messages 的 ILIKE 扫描对外层每行重复执行（生产实测 10s~60s+）；
// IN 子查询形态（2026-08-02）的去相关成败依赖 trigram 统计信息，统计漂移后退化。
// 常量数组形态从结构上不可能产生 correlated SubPlan。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestSessionKeywordFilterSQL_UsesAnyArrayShape(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if fragment == "" {
		t.Fatal("SessionKeywordFilterSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "jsonb_array_elements_text(") {
		t.Errorf("SessionKeywordFilterSQL must expand sessions.questions via jsonb_array_elements_text(), got %q", fragment)
	}
	if !strings.Contains(fragment, "= ANY(?::bigint[])") {
		t.Errorf("SessionKeywordFilterSQL must match ids via = ANY(?::bigint[]) constant array to rule out correlated SubPlan, got %q", fragment)
	}
	if strings.Contains(fragment, "IN (SELECT id FROM messages") {
		t.Errorf("SessionKeywordFilterSQL must not use IN (SELECT id FROM messages ...) — statistics drift turns it into a correlated SubPlan with per-row ILIKE scans; pass ids as a constant array instead, got %q", fragment)
	}
	if strings.Contains(fragment, "JOIN messages ON messages.id =") {
		t.Errorf("SessionKeywordFilterSQL must not JOIN messages (trigram bitmap trap on high-frequency keywords), got %q", fragment)
	}
}

// TestSessionKeywordFilterSQL_AvoidsLandmines 维护历史 bugfix 的护栏：
//
//   - 不能用 ANY(sessions.message_ids)：message_ids 在 PG 里是 jsonb 文本不是原生数组（fix #58）
//
//   - 整段 SQL 必须只有 1 个 '?'（fix #59：裸 '?' 操作符与 gorm 占位符撞车，SQLSTATE 42601）
//
//   - 不能出现 jsonb_exists 强相关历史形态（refactor 2026-06-07）
//
//     @author centonhuang
//     @update 2026-10-15 16:00:00
func TestSessionKeywordFilterSQL_AvoidsLandmines(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if strings.Contains(fragment, "ANY(sessions.message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use native array ANY(sessions.message_ids), got %q", fragment)
	}
	if strings.Contains(fragment, "ANY(message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use ANY against raw message_ids column, got %q", fragment)
	}

	// 占位符只允许 1 个（id 数组占位）。多于 1 个意味着引入了裸 '?' 操作符，
	// gorm 会把它当成参数槽，回到 fix #59 的报错路径（SQLSTATE 42601）。
	placeholderCount := strings.Count(fragment, "?")
	if placeholderCount != 1 {
		t.Errorf("SessionKeywordFilterSQL must contain exactly one ? placeholder (for the id array), got %d in %q", placeholderCount, fragment)
	}

	// jsonb_exists 是历史实现，强相关到 sessions，导致 planner 退化为每条 session
	// 在 messages 全表跑一次 ILIKE。
	if strings.Contains(fragment, "jsonb_exists(") {
		t.Errorf("SessionKeywordFilterSQL must not use historical jsonb_exists() correlated form, got %q", fragment)
	}
}

// TestSessionKeywordMatchIDsSQL_ScopesToUserContent 断言第一段只搜用户消息正文：
// 部分索引 idx_messages_user_content_trgm 以 role='user' 为谓词，查询必须携带相同
// 条件才能命中索引；搜 ->>'content' 而非 message 全文避免 role/tool 元数据误报。
// 多模态 content 是 parts 数组，粗筛命中的是 JSON 文本，必须再按 part.text 精筛，
// 否则搜 "text"/"type" 会命中所有多模态消息。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestSessionKeywordMatchIDsSQL_ScopesToUserContent(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordMatchIDsSQL

	if fragment == "" {
		t.Fatal("SessionKeywordMatchIDsSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "message::jsonb->>'role' = 'user'") {
		t.Errorf("SessionKeywordMatchIDsSQL must scope to user messages to hit the partial index, got %q", fragment)
	}
	if !strings.Contains(fragment, "message::jsonb->>'content' ILIKE ?") {
		t.Errorf("SessionKeywordMatchIDsSQL must match message content via ILIKE ? (indexable coarse filter), got %q", fragment)
	}
	if strings.Contains(fragment, "message::text ILIKE") {
		t.Errorf("SessionKeywordMatchIDsSQL must not match raw message JSON text (role/tool metadata false positives), got %q", fragment)
	}
	if !strings.Contains(fragment, "jsonb_array_elements(") || !strings.Contains(fragment, "part->>'text' ILIKE ?") {
		t.Errorf("SessionKeywordMatchIDsSQL must refine multimodal parts to their text field, got %q", fragment)
	}
	// 占位符：粗筛 + 精筛各一个，调用方传两次同一 pattern
	if strings.Count(fragment, "?") != 2 {
		t.Errorf("SessionKeywordMatchIDsSQL must contain exactly two ? placeholders (coarse + text refine), got %q", fragment)
	}
}
