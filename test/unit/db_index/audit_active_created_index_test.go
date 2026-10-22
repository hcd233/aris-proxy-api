// Package db_index 守护 GORM tag 无法表达的 PG 专有索引 DDL。
//
// 审计列表的 COUNT(*) 在 82K 行 model_call_audits 上是 Seq Scan（2026-10-23 实测 111ms）；
// 需要 (created_at DESC) WHERE deleted_at = 0 的部分索引才能走 index-only scan。
// 部分索引无法用 GORM tag 表达，只能通过启动期原生 DDL 建立。
package db_index

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestAuditActiveCreatedIndexDDL 锁定审计部分索引的形态：
// 幂等 DDL、created_at DESC 排序、deleted_at = 0 部分谓词。
func TestAuditActiveCreatedIndexDDL(t *testing.T) {
	t.Parallel()

	ddl := constant.AuditActiveCreatedIndexDDL
	wants := []string{
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS",
		"idx_mca_active_created",
		"model_call_audits",
		"created_at DESC",
		"WHERE deleted_at = 0",
	}
	for _, want := range wants {
		if !strings.Contains(ddl, want) {
			t.Errorf("AuditActiveCreatedIndexDDL missing %q, got: %s", want, ddl)
		}
	}
}
