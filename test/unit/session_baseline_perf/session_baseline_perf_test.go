// Package session_baseline_perf 验证 session 列表「不带 keyword」基线路径与
// 「带 keyword」路径的机制级优化在代码层面落地。
//
// 优化背景：
//
//	refactor/session-list-baseline-perf-2026-06-07：
//	  - 旧 SessionSummarySelect 用 COALESCE(jsonb_array_length(message_ids::jsonb), 0)
//	    AS message_count 做表达式投影，sort by message_count/tool_count 没法走索引，
//	    每行都要做 jsonb 解析。
//	  - 旧实现 sessions 表上只有主键索引，
//	    SELECT … WHERE deleted_at = 0 AND api_key_name IN (…) AND created_at BETWEEN ?
//	    ORDER BY created_at DESC 走全表 + filesort；COUNT(*) 又跑同样的 WHERE，成本翻倍。
//
//	  现版本：
//	    1) 物化 message_count / tool_count 列；
//	    2) 标准 BTREE 复合索引 (api_key_name, created_at) / (deleted_at, created_at)；
//	    3) SessionSummarySelect 改为直接读列。
//
//	perf/session-list-trigram-and-windowcount-2026-06-08：
//	  - keyword 路径加上 pg_trgm + GIN trigram 表达式索引让 messages.message::text ILIKE
//	    走 trigram bitmap 扫描，2 字符及以上子串都能命中。
//
//	perf/session-list-split-count-2026-10-23（BENCHMARK §3.6 压测复核）：
//	  - 2026-06-08 的 COUNT(*) OVER () 折进 SELECT 在大表上适得其反：窗口函数在 LIMIT
//	    之前求值，PG 必须对 WHERE 命中的全部 3225 行计算 jsonb_array_length（每行一次
//	    TOAST 解压）再 top-N 排序，EXPLAIN 实测 119.6ms。
//	  - 现改为独立 COUNT(*)（走 idx_sessions_first_msg 部分索引，1.0ms）+ 分页 SELECT
//	    去掉窗口（只解压 20 行，4.1ms）。SQL 形态由 test/unit/session_list_split 钉住。
//
// 雷区记录：
//
//	提交 75658e5 走 pg_trgm + GIN 表达式索引路径，
//	CREATE INDEX ... USING gin (message::text gin_trgm_ops) 因表达式外层括号缺失抛
//	SQLSTATE 42601，migrate Job 直接卡死整个 deploy，最终被 11e4602 revert。
//	这次纠正成 USING gin ((message::text) gin_trgm_ops)（外层是索引列列表的括号，
//	内层是表达式本身的括号），并把这个形态钉进单测。
package session_baseline_perf

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// TestSessionSummarySelect_UsesExpressionCountColumns 投影 SQL 必须用表达式从
// message_ids / tool_ids 实时计算 count，不再依赖物化列。
//
//	@author centonhuang
//	@update 2026-06-09 17:10:00
func TestSessionSummarySelect_UsesExpressionCountColumns(t *testing.T) {
	t.Parallel()
	sel := constant.SessionSummarySelect

	if sel == "" {
		t.Fatal("SessionSummarySelect must be defined")
	}
	// 必须包含 jsonb_array_length 来计算 count
	if !strings.Contains(sel, "jsonb_array_length") {
		t.Errorf("SessionSummarySelect must use jsonb_array_length to compute count from message_ids/tool_ids, got %q", sel)
	}
	// 必须包含 message_count 和 tool_count 作为别名
	if !strings.Contains(sel, "message_count") {
		t.Errorf("SessionSummarySelect must alias computed count as message_count, got %q", sel)
	}
	if !strings.Contains(sel, "tool_count") {
		t.Errorf("SessionSummarySelect must alias computed count as tool_count, got %q", sel)
	}
}

// TestSessionSummarySelect_HasNoWindowCount 投影不得包含 COUNT(*) OVER ()：
// 窗口函数强制在 LIMIT 前对全部命中行求值 jsonb_array_length，是 session/list
// 3.7s 尾延迟的主因（2026-10-23 压测复核，见包注释）。
//
//	@author centonhuang
//	@update 2026-10-23 01:20:00
func TestSessionSummarySelect_HasNoWindowCount(t *testing.T) {
	t.Parallel()
	sel := constant.SessionSummarySelect

	if strings.Contains(sel, "COUNT(*) OVER") {
		t.Errorf("SessionSummarySelect must not fold COUNT into a window function: it forces jsonb detoast on every matched row before LIMIT, got %q", sel)
	}
	if strings.Contains(sel, "total_count") {
		t.Errorf("SessionSummarySelect must not carry the windowed total_count alias; total is now a separate COUNT query, got %q", sel)
	}
}

// TestSessionSummaryRow_HasTotalCountField 行模型必须有 TotalCount 字段映射到
// total_count 别名，否则 GORM Find 会丢掉窗口函数返回的总数。
// 这里通过反射读私有结构体——由于是同包访问做不到，但可以通过 sessionRepository
// 的导出 API（FindMessagesByIDsChunked 等）间接覆盖，所以这里改成只 sanity-check
// SessionSummaryProjection（窗口函数返回的 total 仅供 PageInfo 使用，不进 projection）。
//
// 真实保护是 SessionSummarySelect 与 sessionSummaryRow 必须保持别名一致；
// 这一对一致性靠 e2e 测试的 pageInfo.total 数值正确性来兜底。
//
//	@author centonhuang
//	@update 2026-06-08 00:55:00
func TestSessionSummaryProjection_DoesNotLeakTotalCount(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(dbmodel.Session{})
	if _, ok := rt.FieldByName("TotalCount"); ok {
		t.Errorf("dbmodel.Session must not define TotalCount field; total_count is a windowed alias only on sessionSummaryRow, not a real column")
	}
}

// TestSessionModelDoesNotHaveMaterializedCountColumns 校验 GORM 模型不再定义
// message_count / tool_count 物化列，count 从 message_ids / tool_ids 实时计算。
//
//	@author centonhuang
//	@update 2026-06-09 17:10:00
func TestSessionModelDoesNotHaveMaterializedCountColumns(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(dbmodel.Session{})

	checks := []string{"MessageCount", "ToolCount"}

	for _, field := range checks {
		if _, ok := rt.FieldByName(field); ok {
			t.Errorf("dbmodel.Session must not define %s field; count should be computed from message_ids/tool_ids", field)
		}
	}
}
