package dashboard_repo

import (
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-mass/internal/common/enum"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/infrastructure/repository"
	"github.com/hcd233/aris-mass/test/unit/sqlitetest"
)

func TestQueryStatsSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)

	if _, err := repo.QueryStats(t.Context(), nil, rangeStart, rangeEnd); err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("QueryStats produced %d queries, want 1: %v", len(sqls), sqls)
	}
	assertSQLContains(t, sqls[0],
		"count(*) filter (where upstream_status_code = 200)",
		"percentile_cont(0.95) within group (order by first_token_latency_ms)",
		"cost_micro is not null",
		"stream_duration_ms > 0",
		"deleted_at = 0",
	)
	if strings.Contains(strings.ToLower(sqls[0]), "api_key_id in") {
		t.Errorf("admin scope must not filter api_key_id: %s", sqls[0])
	}
}

func TestQueryStatsSeriesSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)

	if _, err := repo.QueryStatsSeries(t.Context(), []uint{7}, rangeStart, rangeEnd, enum.GranularityHour); err != nil {
		t.Fatalf("QueryStatsSeries: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("QueryStatsSeries produced %d queries, want 1: %v", len(sqls), sqls)
	}
	assertSQLContains(t, sqls[0], "date_trunc('hour'", "group by", "api_key_id in", "percentile_cont(0.5)")
}

// 定宽粒度用 date_bin 以 Unix 纪元为原点切桶（与 Go 侧 BuildBuckets 的 Time.Truncate 对齐）
func TestQueryStatsSeriesSQL_FixedStepGranularity(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		enum.GranularityFiveMinutes:   "date_bin(interval '5 minutes', created_at, timestamptz '1970-01-01 00:00:00+00')",
		enum.GranularityTenMinutes:    "date_bin(interval '10 minutes', created_at, timestamptz '1970-01-01 00:00:00+00')",
		enum.GranularityThirtyMinutes: "date_bin(interval '30 minutes', created_at, timestamptz '1970-01-01 00:00:00+00')",
		enum.GranularityFourHours:     "date_bin(interval '4 hours', created_at, timestamptz '1970-01-01 00:00:00+00')",
	}
	for g, want := range cases {
		t.Run(g, func(t *testing.T) {
			t.Parallel()
			db, captured := newDryRunDB(t)
			repo := repository.NewDashboardRepository(db, nil)
			if _, err := repo.QueryStatsSeries(t.Context(), []uint{7}, rangeStart, rangeEnd, g); err != nil {
				t.Fatalf("QueryStatsSeries: %v", err)
			}
			sqls := captured.all()
			if len(sqls) != 1 {
				t.Fatalf("QueryStatsSeries produced %d queries, want 1: %v", len(sqls), sqls)
			}
			assertSQLContains(t, sqls[0], want, "group by")
		})
	}
}

func TestQueryUptimeBucketsSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)

	points, err := repo.QueryUptimeBuckets(t.Context(), []uint{7}, rangeStart, rangeEnd, 32)
	if err != nil {
		t.Fatalf("QueryUptimeBuckets: %v", err)
	}
	if len(points) != 32 {
		t.Fatalf("len(points) = %d, want 32", len(points))
	}
	if !points[0].Time.Equal(rangeStart) || !points[31].End.Equal(rangeEnd) {
		t.Errorf("uptime range = [%v,%v], want [%v,%v]", points[0].Time, points[31].End, rangeStart, rangeEnd)
	}
	sqls := captured.all()
	if len(sqls) != 2 {
		t.Fatalf("QueryUptimeBuckets produced %d queries, want 2 (buckets + failure codes): %v", len(sqls), sqls)
	}
	assertSQLContains(t, sqls[0], "date_bin(interval '18901 seconds'", "group by", "api_key_id in")
	for _, s := range sqls {
		if strings.Contains(strings.ToLower(s), "percentile_cont") {
			t.Errorf("uptime query must not compute percentiles: %s", s)
		}
	}
	assertSQLContains(t, sqls[1], "date_bin(interval '18901 seconds'", "upstream_status_code <> 200",
		"upstream_status_code as status_code", "group by time, status_code", "api_key_id in")
	// date_bin 的原点必须是区间起点，且作为第一个绑定参数（SELECT 先于 WHERE 绑定）
	vars := captured.lastVars()
	if len(vars) == 0 {
		t.Fatal("no bind vars captured")
	}
	origin, ok := vars[0].(time.Time)
	if !ok || !origin.Equal(rangeStart) {
		t.Errorf("first bind var = %#v, want bucket origin %v", vars[0], rangeStart)
	}
}

func TestQueryUptimeBuckets_EmptyScopeFillsBuckets(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)

	points, err := repo.QueryUptimeBuckets(t.Context(), []uint{}, rangeStart, rangeEnd, 32)
	if err != nil || len(points) != 32 {
		t.Fatalf("QueryUptimeBuckets(empty) = (%d, %v), want (32, nil)", len(points), err)
	}
	if got := len(captured.all()); got != 0 {
		t.Fatalf("empty scope must not hit db, got %d queries", got)
	}
	for _, p := range points {
		if p.Requests != 0 || p.Success != 0 {
			t.Errorf("empty scope bucket = %+v, want zero", p)
		}
	}
}

func TestDashboardStats_EmptyScopeShortCircuits(t *testing.T) {
	t.Parallel()

	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seed := []*dbmodel.ModelCallAudit{
		{APIKeyID: 10, ModelID: "gpt-4", UpstreamStatusCode: 200, CreatedAt: rangeStart.Add(time.Hour)},
		{APIKeyID: 20, ModelID: "claude", UpstreamStatusCode: 200, CreatedAt: rangeStart.Add(time.Hour)},
	}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := repository.NewDashboardRepository(db, nil)

	stats, err := repo.QueryStats(t.Context(), []uint{}, rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("QueryStats(empty) err: %v", err)
	}
	if stats == nil || stats.Requests != 0 || stats.LatencyP95Ms != nil {
		t.Errorf("QueryStats(empty) = %+v, want zero value", stats)
	}
	points, err := repo.QueryStatsSeries(t.Context(), []uint{}, rangeStart, rangeEnd, enum.GranularityHour)
	if err != nil || len(points) != 0 {
		t.Errorf("QueryStatsSeries(empty) = (%d, %v), want (0, nil)", len(points), err)
	}
}

func TestDashboardStats_CacheHit(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, newReadCache(t))

	for range 2 {
		if _, err := repo.QueryStats(t.Context(), nil, rangeStart, rangeEnd); err != nil {
			t.Fatalf("QueryStats: %v", err)
		}
	}
	if got := len(captured.all()); got != 1 {
		t.Fatalf("same params must be served by cache, got %d db queries", got)
	}
	if _, err := repo.QueryStats(t.Context(), nil, rangeStart, rangeEnd.Add(2*time.Minute)); err != nil {
		t.Fatalf("shifted QueryStats: %v", err)
	}
	if got := len(captured.all()); got != 2 {
		t.Fatalf("different window must miss cache, got %d db queries", got)
	}
}
