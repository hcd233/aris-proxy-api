// Package session_dedup SessionDAO 组查询与窗口查询的 SQL 形态护栏
//
// 两个查询都依赖 PostgreSQL 专有语法（::jsonb 强转），sqlite 无法真实执行，
// 故用 DryRun 钉住生成的 SQL 形态，行为由 e2e 覆盖
// （同 MessageDAO.FilterTerminalToolCallIDs 的先例）。
//
// 关键不变量：SessionFirstMessageIDCondition 的表达式必须与生产索引
// idx_sessions_first_msg 的索引表达式 (message_ids::jsonb->>0) 逐字一致，
// 否则插入路径的组查询退化为全表 JSON 扫描。
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newSQLShapeDB 打开 postgres DryRun 会话：只构建 SQL 不执行（不建连接）。
// 必须用 postgres dialector 而非 sqlite：sqlite 驱动注册的 "FOR" ClauseBuilder
// 不渲染 FOR UPDATE（sqlite 不支持行锁），postgres 与生产行为一致。
// ::jsonb 为 PG 专有语法，行为（真实执行）由 e2e 覆盖
// （同 MessageDAO.FilterTerminalToolCallIDs 的先例）。
func newSQLShapeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 user=guard dbname=guard port=5432 sslmode=disable"), &gorm.Config{
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open dryrun pg db: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

// assertSQLContains 断言 DryRun 生成的 SQL 包含全部期望片段
func assertSQLContains(t *testing.T, sqlText string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(sqlText, want) {
			t.Errorf("generated SQL missing %q\ngot: %s", want, sqlText)
		}
	}
}

// TestGroupForUpdateQuery_SQLShape 钉住组查询形态：
// ::jsonb 强转（索引命中前提）、FOR UPDATE（同组并发串行化）、deleted_at = 0（活跃行）。
func TestGroupForUpdateQuery_SQLShape(t *testing.T) {
	t.Parallel()
	dry := newSQLShapeDB(t)

	var out []*dbmodel.Session
	q := dao.GetSessionDAO().GroupForUpdateQuery(dry, 42).Find(&out)

	assertSQLContains(t, q.Statement.SQL.String(), []string{
		"message_ids::jsonb->>0",
		"FOR UPDATE",
		"deleted_at = 0",
	})
}

// TestCreatedSinceQuery_SQLShape 钉住窗口扫描形态：
// created_at >= 窗口条件（依赖 idx_sessions_created_at）、
// 列收窄为 id + message_ids（不含 tool_ids）。
func TestCreatedSinceQuery_SQLShape(t *testing.T) {
	t.Parallel()
	dry := newSQLShapeDB(t)

	var out []*dbmodel.Session
	q := dao.GetSessionDAO().CreatedSinceQuery(dry, time.Unix(0, 0).UTC()).Find(&out)

	sqlText := q.Statement.SQL.String()
	assertSQLContains(t, sqlText, []string{"created_at >=", "deleted_at = 0"})
	if strings.Contains(sqlText, "tool_ids") {
		t.Errorf("window scan must not select tool_ids\ngot: %s", sqlText)
	}
}

// TestSessionFirstMessageIDCondition_MatchesIndexExpression 钉住常量表达式与
// 生产索引表达式逐字一致（去掉括号与占位符后比较）。
func TestSessionFirstMessageIDCondition_MatchesIndexExpression(t *testing.T) {
	t.Parallel()
	if constant.SessionFirstMessageIDCondition != "message_ids::jsonb->>0 = ?" {
		t.Errorf("SessionFirstMessageIDCondition = %q, must be %q to match idx_sessions_first_msg",
			constant.SessionFirstMessageIDCondition, "message_ids::jsonb->>0 = ?")
	}
}
