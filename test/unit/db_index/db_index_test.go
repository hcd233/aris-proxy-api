// Package db_index 验证 ModelCallAudit 等模型的 GORM 索引 tag 能生成预期索引。
//
// 背景（perf/model-call-audit-indexes-2026-08-02）：
//   - ModelCallAudit 新增 idx_mca_created_at / idx_mca_apikey_created / idx_mca_model_created
//     三个索引，覆盖 admin 分页与趋势聚合查询；CreatedAt 重声明覆盖 BaseModel 只为挂索引 tag。
//   - 本测试用 sqlite 内存库执行 AutoMigrate，断言索引名与列组合正确，
//     防止后续改 tag 时静默破坏索引定义。
package db_index

import (
	"strings"
	"testing"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// indexExpect 期望的索引定义（name -> 列列表，保持顺序）。
var indexExpect = map[string][]string{
	"idx_mca_created_at":     {"created_at"},
	"idx_mca_apikey_created": {"api_key_id", "created_at"},
	"idx_mca_model_created":  {"model_id", "created_at"},
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db
}

// TestModelCallAuditAutoMigrateIndexes 验证 ModelCallAudit 迁移后生成预期复合索引。
func TestModelCallAuditAutoMigrateIndexes(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("failed to migrate model_call_audits: %v", err)
	}

	got := map[string][]string{}
	rows, err := db.Raw("SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'model_call_audits'").Rows()
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatalf("failed to scan index row: %v", err)
		}
		got[name] = parseIndexColumns(t, sql)
	}

	for idxName, wantCols := range indexExpect {
		gotCols, ok := got[idxName]
		if !ok {
			t.Errorf("index %q missing, got indexes: %v", idxName, keys(got))
			continue
		}
		if !equalStrings(gotCols, wantCols) {
			t.Errorf("index %q columns = %v, want %v", idxName, gotCols, wantCols)
		}
	}
}

// parseIndexColumns 从 sqlite_master 的 index SQL 中解析列名，如
// "CREATE INDEX idx_mca_apikey_created ON model_call_audits(api_key_id, created_at desc)"。
func parseIndexColumns(t *testing.T, sql string) []string {
	t.Helper()
	open := strings.Index(sql, "(")
	end := strings.LastIndex(sql, ")")
	if open < 0 || end <= open {
		t.Fatalf("unexpected index sql: %q", sql)
	}
	raw := sql[open+1 : end]
	parts := strings.Split(raw, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		field := strings.TrimSpace(p)
		if i := strings.Index(field, " "); i > 0 {
			field = field[:i]
		}
		cols = append(cols, strings.Trim(field, "`\"[]"))
	}
	return cols
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
