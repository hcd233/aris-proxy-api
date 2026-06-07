// Package session_keyword_index 验证 session 列表 keyword 检索依赖的索引 DDL 常量。
//
// 回归背景（feature/session-keyword-trgm-perf-2026-06-07）：
//   - GET /api/v1/session/list?keyword=xxx 在没有 trigram 索引时是 messages 表
//     顺序扫描，线上一次请求可达秒级。
//   - 修复方案：在 constant.SessionKeywordIndexSQLs 暴露三条 DDL，由
//     database.EnsureSearchIndexes 在 database migrate 阶段幂等执行：
//     1) CREATE EXTENSION pg_trgm（trigram 扩展）；
//     2) idx_messages_message_trgm：GIN trigram 索引，给 messages.message::text
//     的 ILIKE 用；
//     3) idx_sessions_message_ids_gin：GIN jsonb_path_ops 索引，给
//     jsonb_exists / @> 反查 sessions.message_ids 用。
//
// 本测试只做"常量结构 + 幂等关键字"静态断言；不连真实 DB。
package session_keyword_index

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestSessionKeywordIndexSQLs_AllPresent(t *testing.T) {
	t.Parallel()
	ddls := constant.SessionKeywordIndexSQLs
	if len(ddls) != 3 {
		t.Fatalf("SessionKeywordIndexSQLs must have exactly 3 DDLs (pg_trgm ext + 2 indexes), got %d: %q", len(ddls), ddls)
	}

	want := []string{
		"pg_trgm",
		"idx_messages_message_trgm",
		"idx_sessions_message_ids_gin",
	}
	for _, w := range want {
		found := false
		for _, ddl := range ddls {
			if strings.Contains(ddl, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SessionKeywordIndexSQLs must contain DDL referencing %q, got %q", w, ddls)
		}
	}
}

func TestSessionKeywordIndexSQLs_AreIdempotent(t *testing.T) {
	t.Parallel()
	for i, ddl := range constant.SessionKeywordIndexSQLs {
		upper := strings.ToUpper(ddl)
		// CREATE EXTENSION / CREATE INDEX 都必须带 IF NOT EXISTS 才是幂等。
		// 否则在重启时 database migrate 会因为对象已存在而失败。
		if !strings.Contains(upper, "IF NOT EXISTS") {
			t.Errorf("DDL #%d must be idempotent (IF NOT EXISTS), got %q", i, ddl)
		}
	}
}

func TestSessionKeywordIndexSQLs_MessageIndexUsesTrgmOps(t *testing.T) {
	t.Parallel()
	for _, ddl := range constant.SessionKeywordIndexSQLs {
		if !strings.Contains(ddl, "idx_messages_message_trgm") {
			continue
		}
		if !strings.Contains(ddl, "gin_trgm_ops") {
			t.Errorf("messages trgm index must use gin_trgm_ops for ILIKE, got %q", ddl)
		}
		if !strings.Contains(ddl, "message::text") {
			t.Errorf("messages trgm index must cast message column to text, got %q", ddl)
		}
		return
	}
	t.Fatal("did not find idx_messages_message_trgm DDL")
}

func TestSessionKeywordIndexSQLs_SessionIndexUsesJSONBPathOps(t *testing.T) {
	t.Parallel()
	for _, ddl := range constant.SessionKeywordIndexSQLs {
		if !strings.Contains(ddl, "idx_sessions_message_ids_gin") {
			continue
		}
		if !strings.Contains(ddl, "jsonb_path_ops") {
			t.Errorf("sessions message_ids index must use jsonb_path_ops for @/containment lookups, got %q", ddl)
		}
		if !strings.Contains(ddl, "message_ids::jsonb") {
			t.Errorf("sessions message_ids index must cast message_ids to jsonb, got %q", ddl)
		}
		return
	}
	t.Fatal("did not find idx_sessions_message_ids_gin DDL")
}
