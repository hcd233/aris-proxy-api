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
//	  - 把 COUNT(*) OVER () AS total_count 折进同一条 SELECT，省掉一次独立 COUNT(*)
//	    roundtrip 与 WHERE 评估；对带 keyword 的请求尤其受益（EXISTS 子查询从两次执行
//	    降到一次）。
//	  - keyword 路径加上 pg_trgm + GIN trigram 表达式索引让 messages.message::text ILIKE
//	    走 trigram bitmap 扫描，2 字符及以上子串都能命中。
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

// TestSessionSummarySelect_UsesMaterializedCountColumns 投影 SQL 必须直读物化列，
// 不能再回去用 jsonb_array_length / ::jsonb 强转。
//
//	@author centonhuang
//	@update 2026-06-08 00:55:00
func TestSessionSummarySelect_UsesMaterializedCountColumns(t *testing.T) {
	t.Parallel()
	sel := constant.SessionSummarySelect

	if sel == "" {
		t.Fatal("SessionSummarySelect must be defined")
	}
	if strings.Contains(sel, "jsonb_array_length") {
		t.Errorf("SessionSummarySelect must not call jsonb_array_length; use materialized message_count/tool_count columns, got %q", sel)
	}
	// 注意：windowed COUNT 行为是允许的；'::jsonb' 强转才是要禁止的。
	// 这里特别检查投影里不能再出现 ::jsonb（与窗口函数中的 :: 无关，因为窗口函数没有强转）。
	if strings.Contains(sel, "::jsonb") {
		t.Errorf("SessionSummarySelect must not cast message_ids/tool_ids to jsonb in projection; use materialized columns, got %q", sel)
	}
	if !strings.Contains(sel, "message_count") {
		t.Errorf("SessionSummarySelect must select materialized message_count column, got %q", sel)
	}
	if !strings.Contains(sel, "tool_count") {
		t.Errorf("SessionSummarySelect must select materialized tool_count column, got %q", sel)
	}
}

// TestSessionSummarySelect_FoldsCountIntoWindowFunction 投影必须包含
// COUNT(*) OVER () AS total_count，把分页 SELECT 与 COUNT 折成一条语句执行。
//
//	@author centonhuang
//	@update 2026-06-08 00:55:00
func TestSessionSummarySelect_FoldsCountIntoWindowFunction(t *testing.T) {
	t.Parallel()
	sel := constant.SessionSummarySelect

	if !strings.Contains(sel, "COUNT(*) OVER ()") {
		t.Errorf("SessionSummarySelect must fold COUNT into the same SELECT via COUNT(*) OVER () to save a roundtrip + WHERE re-evaluation, got %q", sel)
	}
	if !strings.Contains(sel, "total_count") {
		t.Errorf("SessionSummarySelect must alias the windowed count as total_count to match sessionSummaryRow.TotalCount, got %q", sel)
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

// TestSessionModelHasMaterializedCountColumns 校验 GORM 模型把 message_count / tool_count
// 真的当成实体列写出来，并带 not null + default:0，让 AutoMigrate 在已有大表上做
// "metadata-only ADD COLUMN"（PG 12+），不会触发表重写或锁。
//
//	@author centonhuang
//	@update 2026-06-08 00:55:00
func TestSessionModelHasMaterializedCountColumns(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(dbmodel.Session{})

	checks := []struct {
		field  string
		column string
	}{
		{field: "MessageCount", column: "message_count"},
		{field: "ToolCount", column: "tool_count"},
	}

	for _, c := range checks {
		f, ok := rt.FieldByName(c.field)
		if !ok {
			t.Errorf("dbmodel.Session must define %s field", c.field)
			continue
		}
		if f.Type.Kind() != reflect.Int {
			t.Errorf("%s must be int (so PG ADD COLUMN with default:0 is metadata-only), got %v", c.field, f.Type)
		}
		tag := string(f.Tag)
		if !strings.Contains(tag, "column:"+c.column) {
			t.Errorf("%s tag must declare column:%s, got %q", c.field, c.column, tag)
		}
		if !strings.Contains(tag, "not null") {
			t.Errorf("%s tag must be not null (so reads never see NULL), got %q", c.field, tag)
		}
		if !strings.Contains(tag, "default:0") {
			t.Errorf("%s tag must have default:0 (so AutoMigrate ADD COLUMN succeeds on populated table), got %q", c.field, tag)
		}
	}
}
