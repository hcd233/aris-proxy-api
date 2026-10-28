package dashboard_repo

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/test/unit/sqlitetest"
)

func TestQueryModelSeriesSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)
	if _, err := repo.QueryModelSeries(t.Context(), nil, rangeStart, rangeEnd, enum.GranularityDay); err != nil {
		t.Fatalf("QueryModelSeries: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("want 1 query, got %v", sqls)
	}
	assertSQLContains(t, sqls[0], "group by model_id, time", "filter (where upstream_status_code = 200", "date_trunc('day'")
}

func TestQueryTopErrorsSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)
	if _, err := repo.QueryTopErrors(t.Context(), nil, rangeStart, rangeEnd, 5); err != nil {
		t.Fatalf("QueryTopErrors: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("want 1 query, got %v", sqls)
	}
	assertSQLContains(t, sqls[0], "upstream_status_code <> 200", "left(error_message, 200)", "order by count desc")
	if !slices.Contains(captured.lastVars(), any(5)) {
		t.Errorf("limit must be a bound parameter, vars=%v", captured.lastVars())
	}
}

func TestQueryEndpointHealthSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)
	if _, err := repo.QueryEndpointHealth(t.Context(), nil, rangeStart, rangeEnd, 5); err != nil {
		t.Fatalf("QueryEndpointHealth: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("want 1 query, got %v", sqls)
	}
	assertSQLContains(t, sqls[0], "group by endpoint", "order by requests desc", "percentile_cont(0.95)")
}

func TestQueryGroupUsageSQL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		dimension enum.DashboardDimension
		fragments []string
	}{
		{"model", enum.DashboardDimensionModel, []string{"model_id as group_key", "group by group_key"}},
		{"apikey", enum.DashboardDimensionAPIKey, []string{"cast(api_key_id as text)"}},
		{"harness", enum.DashboardDimensionHarness, []string{"agent_harness_rules", "'other'"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, captured := newDryRunDB(t)
			repo := repository.NewDashboardRepository(db, nil)
			if _, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, tc.dimension, "1:1"); err != nil {
				t.Fatalf("QueryGroupUsage: %v", err)
			}
			sqls := captured.all()
			if len(sqls) != 1 {
				t.Fatalf("want 1 query, got %v", sqls)
			}
			assertSQLContains(t, sqls[0], tc.fragments...)
		})
	}
}

func TestQueryGroupUsage_RejectsUnsupportedDimension(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)
	for _, dim := range []enum.DashboardDimension{enum.DashboardDimensionUser, "id; drop table x"} {
		_, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, dim, "")
		if !errors.Is(err, ierr.ErrValidation) {
			t.Errorf("dimension %q: err = %v, want ErrValidation", dim, err)
		}
	}
	if got := captured.all(); len(got) != 0 {
		t.Errorf("unsupported dimension must not reach SQL, got %v", got)
	}
}

func TestQueryHeatmapSQL(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, nil)
	if _, err := repo.QueryHeatmap(t.Context(), nil, rangeStart, rangeEnd, "Asia/Shanghai"); err != nil {
		t.Fatalf("QueryHeatmap: %v", err)
	}
	sqls := captured.all()
	if len(sqls) != 1 {
		t.Fatalf("want 1 query, got %v", sqls)
	}
	assertSQLContains(t, sqls[0],
		"extract(isodow from created_at at time zone $",
		"extract(hour from created_at at time zone $",
	)
	if strings.Contains(sqls[0], "Asia/Shanghai") {
		t.Errorf("timezone must be bound, not inlined: %s", sqls[0])
	}
	tzCount := 0
	for _, v := range captured.lastVars() {
		if v == "Asia/Shanghai" {
			tzCount++
		}
	}
	if tzCount != 2 {
		t.Errorf("timezone must be bound twice, vars=%v", captured.lastVars())
	}
}

func TestQueryGroupUsageCacheKeyIncludesFingerprint(t *testing.T) {
	t.Parallel()

	db, captured := newDryRunDB(t)
	repo := repository.NewDashboardRepository(db, newReadCache(t))
	for _, fp := range []string{"1:1", "1:1", "2:2"} {
		if _, err := repo.QueryGroupUsage(t.Context(), nil, rangeStart, rangeEnd, enum.DashboardDimensionHarness, fp); err != nil {
			t.Fatalf("QueryGroupUsage: %v", err)
		}
	}
	if got := len(captured.all()); got != 2 {
		t.Errorf("fingerprint must partition cache keys: want 2 db queries, got %d", got)
	}
}

func TestDashboardBreakdown_EmptyScopeShortCircuits(t *testing.T) {
	t.Parallel()

	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.ModelCallAudit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&dbmodel.ModelCallAudit{APIKeyID: 10, ModelID: "gpt-4", UpstreamStatusCode: 500, CreatedAt: rangeStart}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := repository.NewDashboardRepository(db, nil)
	ctx := t.Context()
	empty := []uint{}

	checks := []struct {
		name string
		run  func() (int, error)
	}{
		{"model series", func() (int, error) {
			r, err := repo.QueryModelSeries(ctx, empty, rangeStart, rangeEnd, enum.GranularityDay)
			return len(r), err
		}},
		{"top errors", func() (int, error) {
			r, err := repo.QueryTopErrors(ctx, empty, rangeStart, rangeEnd, 5)
			return len(r), err
		}},
		{"endpoint health", func() (int, error) {
			r, err := repo.QueryEndpointHealth(ctx, empty, rangeStart, rangeEnd, 5)
			return len(r), err
		}},
		{"group usage", func() (int, error) {
			r, err := repo.QueryGroupUsage(ctx, empty, rangeStart, rangeEnd, enum.DashboardDimensionHarness, "")
			return len(r), err
		}},
		{"heatmap", func() (int, error) {
			r, err := repo.QueryHeatmap(ctx, empty, rangeStart, rangeEnd, "UTC")
			return len(r), err
		}},
	}
	for _, c := range checks {
		if n, err := c.run(); err != nil || n != 0 {
			t.Errorf("%s(empty) = (%d, %v), want (0, nil)", c.name, n, err)
		}
	}
}
