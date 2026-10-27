package agent_harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
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

// TestQueryHarnessTrendCacheKeyIncludesRulesFingerprint 钉住 D5：
// 规则指纹参与读缓存键——指纹变了，必须落到不同的缓存键上（否则改规则后图会陈旧 5 分钟）。
func TestQueryHarnessTrendCacheKeyIncludesRulesFingerprint(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db := newAuditDryRunDB(t)
	repo := repository.NewAuditRepository(db, cache.NewReadCache(rdb))

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	if _, err := repo.QueryHarnessTrend(context.Background(), []uint{7}, start, end, enum.GranularityDay, "7:100"); err != nil {
		t.Fatalf("first query: %v", err)
	}
	if _, err := repo.QueryHarnessTrend(context.Background(), []uint{7}, start, end, enum.GranularityDay, "7:200"); err != nil {
		t.Fatalf("second query: %v", err)
	}

	keys := mr.Keys()
	if len(keys) < 2 {
		t.Fatalf("expected one cache key per rules fingerprint, got %d keys: %v", len(keys), keys)
	}
}

// TestFillHarnessTrendSeries 钉住前端依赖的形状：Other 恒在最后、时间轴零值补齐。
func TestFillHarnessTrendSeries(t *testing.T) {
	t.Parallel()

	day1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)
	points := []*modelcall.HarnessTrendPoint{
		{Harness: "ZCode", Time: day1, Count: 2},
		{Harness: "Other", Time: day2, Count: 1},
		{Harness: "DSH", Time: day2, Count: 3},
	}

	items := auditport.FillHarnessTrendSeries(points, day1, day2, enum.GranularityDay)
	if len(items) != 3 {
		t.Fatalf("series = %d, want 3", len(items))
	}
	if items[len(items)-1].Harness != "Other" {
		t.Errorf("Other must be the last series, got %q", items[len(items)-1].Harness)
	}
	for _, item := range items {
		if len(item.Points) != 2 {
			t.Fatalf("%s has %d buckets, want 2 (zero-filled)", item.Harness, len(item.Points))
		}
	}
	var zcode *dto.HarnessTrendItem
	for _, item := range items {
		if item.Harness == "ZCode" {
			zcode = item
		}
	}
	if zcode == nil {
		t.Fatal("ZCode series missing")
	}
	if zcode.Points[0].Count != 2 || zcode.Points[1].Count != 0 {
		t.Errorf("ZCode points = %+v, want 2 then 0", zcode.Points)
	}
}
