// Package session_baseline_perf 验证 session 列表「不带 keyword」基线路径的机制级优化
// 在代码层面落地，避免后续重构时被无意回退到老的 jsonb_array_length / 无索引形态。
//
// 优化背景（refactor/session-list-baseline-perf-2026-06-07）：
//
//   - 旧 SessionSummarySelect 用 COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count
//     做表达式投影，sort by message_count/tool_count 没法走索引，每行都要做 jsonb 解析。
//
//   - 旧实现 sessions 表上只有主键索引，
//     SELECT … WHERE deleted_at = 0 AND api_key_name IN (…) AND created_at BETWEEN ?
//     ORDER BY created_at DESC 走全表 + filesort；COUNT(*) 又跑同样的 WHERE，成本翻倍。
//
//   - 现版本：
//     1) 物化 message_count / tool_count 列，写入路径同步维护，存量 PostMigrate 回填；
//     2) 标准 BTREE 复合索引 (api_key_name, created_at) / (deleted_at, created_at)；
//     3) SessionSummarySelect 改为直接读列，丢掉 jsonb_array_length 与 ::jsonb 强转。
//
// 雷区记录：
//   - 提交 75658e5 走 pg_trgm + GIN 表达式索引路径，
//     CREATE INDEX ... USING gin (message::text gin_trgm_ops) 因括号缺失抛 SQLSTATE 42601，
//     migrate Job 直接卡死整个 deploy，最终被 11e4602 revert。
//     这里把所有相关护栏写进单测，禁止再引入 CREATE EXTENSION / 表达式索引。
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
//	@update 2026-06-07 21:50:00
func TestSessionSummarySelect_UsesMaterializedCountColumns(t *testing.T) {
	t.Parallel()
	sel := constant.SessionSummarySelect

	if sel == "" {
		t.Fatal("SessionSummarySelect must be defined")
	}
	if strings.Contains(sel, "jsonb_array_length") {
		t.Errorf("SessionSummarySelect must not call jsonb_array_length; use materialized message_count/tool_count columns, got %q", sel)
	}
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

// TestSessionModelHasMaterializedCountColumns 校验 GORM 模型把 message_count / tool_count
// 真的当成实体列写出来，并带 not null + default:0，让 AutoMigrate 在已有大表上做
// "metadata-only ADD COLUMN"（PG 12+），不会触发表重写或锁。
//
//	@author centonhuang
//	@update 2026-06-07 21:50:00
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

// TestSessionPerfPostMigrateSQLs_BannedPatterns 把曾经把整个 deploy 拖下水的几种 SQL
// 形态全部钉死，避免再回到 75658e5 的脚下。
//
//	@author centonhuang
//	@update 2026-06-07 21:50:00
func TestSessionPerfPostMigrateSQLs_BannedPatterns(t *testing.T) {
	t.Parallel()
	sqls := constant.SessionPerfPostMigrateSQLs
	if len(sqls) == 0 {
		t.Fatal("SessionPerfPostMigrateSQLs must define at least the BTREE composite indexes")
	}

	for i, q := range sqls {
		// 禁止 CREATE EXTENSION：生产 DB 角色没有 superuser 权限，会直接 502 整个 migrate Job
		// （这是 75658e5 的第一类潜在雷，靠当时 superuser 才没爆，但仍是不可接受的依赖）。
		if strings.Contains(strings.ToUpper(q), "CREATE EXTENSION") {
			t.Errorf("step %d uses CREATE EXTENSION; needs superuser perm, fragile across environments: %q", i, q)
		}

		// 禁止 pg_trgm / gin_trgm_ops / jsonb_path_ops 等扩展 / 非 BTREE 索引方法：
		// 这是 baseline 路径，不需要全文检索；keyword 路径已通过 SessionKeywordFilterSQL 做了
		// PK 回查改写，也用不上这些扩展。
		lower := strings.ToLower(q)
		if strings.Contains(lower, "pg_trgm") || strings.Contains(lower, "gin_trgm_ops") || strings.Contains(lower, "jsonb_path_ops") {
			t.Errorf("step %d references trigram / jsonb_path_ops; baseline path needs only BTREE composite index: %q", i, q)
		}

		trimmed := strings.TrimSpace(strings.ToUpper(q))
		if strings.HasPrefix(trimmed, "CREATE INDEX") {
			// CREATE INDEX 必须 IF NOT EXISTS（幂等可重入，第二次 migrate Job 不会因为索引已存在而 panic）
			if !strings.Contains(trimmed, "IF NOT EXISTS") {
				t.Errorf("step %d CREATE INDEX must use IF NOT EXISTS for idempotence: %q", i, q)
			}
			// 禁止表达式索引里裸用 '::' 强转 —— 这是 75658e5 的真实事故根因
			// （CREATE INDEX ... USING gin (col::text gin_trgm_ops) 抛 SQLSTATE 42601）。
			if strings.Contains(q, "::") {
				t.Errorf("step %d CREATE INDEX contains '::' cast; expression indexes need extra parens, this is what blew up 75658e5: %q", i, q)
			}
			// CREATE INDEX 必须是 BTREE（默认）：明确禁止 USING GIN / USING GIST 等需要扩展或表达式的方法。
			if strings.Contains(trimmed, "USING ") && !strings.Contains(trimmed, "USING BTREE") {
				t.Errorf("step %d CREATE INDEX uses non-default index method; baseline path must be plain BTREE: %q", i, q)
			}
		}
	}
}

// TestSessionPerfPostMigrateSQLs_HasBaselineIndexes 至少要包含两条复合索引，
// 分别覆盖用户路径（api_key_name + created_at）与 admin 路径（deleted_at + created_at），
// 命中 list 接口的 ORDER BY created_at DESC + WHERE 过滤。
//
//	@author centonhuang
//	@update 2026-06-07 21:50:00
func TestSessionPerfPostMigrateSQLs_HasBaselineIndexes(t *testing.T) {
	t.Parallel()
	sqls := constant.SessionPerfPostMigrateSQLs

	hasAPIKeyCreated := false
	hasDeletedAtCreated := false
	for _, q := range sqls {
		lower := strings.ToLower(q)
		// 用户路径：(api_key_name, created_at)
		if strings.Contains(lower, "create index") && strings.Contains(lower, "api_key_name") && strings.Contains(lower, "created_at") {
			hasAPIKeyCreated = true
		}
		// admin 路径：(deleted_at, created_at)
		if strings.Contains(lower, "create index") && strings.Contains(lower, "deleted_at") && strings.Contains(lower, "created_at") {
			hasDeletedAtCreated = true
		}
	}

	if !hasAPIKeyCreated {
		t.Errorf("SessionPerfPostMigrateSQLs missing composite index on (api_key_name, created_at) for user-paginate path")
	}
	if !hasDeletedAtCreated {
		t.Errorf("SessionPerfPostMigrateSQLs missing composite index on (deleted_at, created_at) for admin path / soft-delete filter")
	}
}

// TestSessionPerfPostMigrateSQLs_BackfillIsIdempotent 校验 message_count/tool_count
// 回填 UPDATE 用 WHERE 限定到未回填行，第二次 migrate Job 不会再扫一次全表。
//
//	@author centonhuang
//	@update 2026-06-07 21:50:00
func TestSessionPerfPostMigrateSQLs_BackfillIsIdempotent(t *testing.T) {
	t.Parallel()
	sqls := constant.SessionPerfPostMigrateSQLs

	hasBackfill := false
	for _, q := range sqls {
		lower := strings.ToLower(q)
		if !(strings.HasPrefix(strings.TrimSpace(lower), "update sessions")) {
			continue
		}
		hasBackfill = true
		// 必须 SET 两个列
		if !strings.Contains(lower, "message_count") || !strings.Contains(lower, "tool_count") {
			t.Errorf("backfill UPDATE must set both message_count and tool_count: %q", q)
		}
		// 必须有 WHERE 把已回填行过滤掉，否则第二次 migrate 会重新扫全表
		if !strings.Contains(lower, "where") {
			t.Errorf("backfill UPDATE must have WHERE clause to be idempotent (avoid re-scanning all rows on repeated migrate): %q", q)
		}
		if !strings.Contains(lower, "message_count = 0") || !strings.Contains(lower, "tool_count = 0") {
			t.Errorf("backfill UPDATE WHERE must short-circuit on already-backfilled rows (message_count = 0 AND tool_count = 0): %q", q)
		}
	}

	if !hasBackfill {
		t.Errorf("SessionPerfPostMigrateSQLs must include a one-shot backfill UPDATE for message_count/tool_count")
	}
}
