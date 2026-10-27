package agent_harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func newAuditDryRunDB(t *testing.T) *gorm.DB {
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

func TestQueryHarnessTrendSQL(t *testing.T) {
	t.Parallel()

	db := newAuditDryRunDB(t)
	sqls := make([]string, 0, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_harness_trend", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	repo := repository.NewAuditRepository(db, nil)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	if _, err := repo.QueryHarnessTrend(context.Background(), nil, start, end, enum.GranularityDay, "7:1"); err != nil {
		t.Fatalf("QueryHarnessTrend returned error: %v", err)
	}

	if len(sqls) != 1 {
		t.Fatalf("QueryHarnessTrend produced %d queries, want 1: %v", len(sqls), sqls)
	}
	sqlText := sqls[0]
	if !strings.Contains(sqlText, "agent_harness_rules") {
		t.Errorf("trend query must resolve harness through the rules table: %s", sqlText)
	}
	if !strings.Contains(sqlText, "LIKE r.pattern || '%'") {
		t.Errorf("trend query must prefix-match user_agent against rule patterns: %s", sqlText)
	}
	if !strings.Contains(sqlText, "COALESCE") || !strings.Contains(sqlText, "'Other'") {
		t.Errorf("trend query must coalesce unmapped rows to Other: %s", sqlText)
	}
	if !strings.Contains(strings.ToLower(sqlText), "group by") {
		t.Errorf("trend query must group by harness bucket: %s", sqlText)
	}
	if strings.Contains(sqlText, "upstream_status_code") {
		t.Errorf("harness trend counts all calls (including failures), must not filter by upstream status: %s", sqlText)
	}
}

func TestQueryHarnessTrendShortCircuitsEmptyScope(t *testing.T) {
	t.Parallel()

	db := newAuditDryRunDB(t)
	sqls := make([]string, 0, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_harness_trend_empty", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	repo := repository.NewAuditRepository(db, nil)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	points, err := repo.QueryHarnessTrend(context.Background(), []uint{}, start, end, enum.GranularityDay, "7:1")
	if err != nil {
		t.Fatalf("QueryHarnessTrend returned error: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("empty scope must return no points, got %d", len(points))
	}
	if len(sqls) != 0 {
		t.Errorf("empty scope must not query the database, got: %v", sqls)
	}
}
