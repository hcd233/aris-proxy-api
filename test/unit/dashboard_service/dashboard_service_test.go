// Package dashboard_service DashboardService 单元测试：权限分流、短路、上一周期、补桶、Top 聚合、脱敏。
package dashboard_service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

var (
	start = time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	end   = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
)

type repoCall struct {
	method     string
	apiKeyIDs  []uint
	start, end time.Time
	dimension  string
	location   string
}

// fakeRepo 记录调用并返回预置数据
type fakeRepo struct {
	mu        sync.Mutex
	calls     []repoCall
	stats     *modelcall.DashboardStats
	series    []*modelcall.DashboardStatsPoint
	groups    []*modelcall.DashboardGroupRow
	endpoints []*modelcall.DashboardEndpointRow
}

func (f *fakeRepo) record(c repoCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *fakeRepo) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeRepo) callsOf(method string) []repoCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repoCall
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeRepo) QueryStats(_ context.Context, ids []uint, s, e time.Time) (*modelcall.DashboardStats, error) {
	f.record(repoCall{method: "stats", apiKeyIDs: ids, start: s, end: e})
	if f.stats == nil {
		return &modelcall.DashboardStats{}, nil
	}
	return f.stats, nil
}

func (f *fakeRepo) QueryStatsSeries(_ context.Context, ids []uint, s, e time.Time, _ enum.Granularity) ([]*modelcall.DashboardStatsPoint, error) {
	f.record(repoCall{method: "series", apiKeyIDs: ids, start: s, end: e})
	return f.series, nil
}

func (f *fakeRepo) QueryModelSeries(_ context.Context, ids []uint, s, e time.Time, _ enum.Granularity) ([]*modelcall.DashboardModelPoint, error) {
	f.record(repoCall{method: "model", apiKeyIDs: ids, start: s, end: e})
	return []*modelcall.DashboardModelPoint{}, nil
}

func (f *fakeRepo) QueryTopErrors(_ context.Context, ids []uint, s, e time.Time, _ int) ([]*modelcall.DashboardErrorRow, error) {
	f.record(repoCall{method: "errors", apiKeyIDs: ids, start: s, end: e})
	return []*modelcall.DashboardErrorRow{}, nil
}

func (f *fakeRepo) QueryEndpointHealth(_ context.Context, ids []uint, s, e time.Time, _ int) ([]*modelcall.DashboardEndpointRow, error) {
	f.record(repoCall{method: "endpoint", apiKeyIDs: ids, start: s, end: e})
	return f.endpoints, nil
}

func (f *fakeRepo) QueryGroupUsage(_ context.Context, ids []uint, s, e time.Time, dim enum.DashboardDimension, _ string) ([]*modelcall.DashboardGroupRow, error) {
	f.record(repoCall{method: "group", apiKeyIDs: ids, start: s, end: e, dimension: dim})
	return f.groups, nil
}

func (f *fakeRepo) QueryHeatmap(_ context.Context, ids []uint, s, e time.Time, loc string) ([]*modelcall.DashboardHeatmapCell, error) {
	f.record(repoCall{method: "heatmap", apiKeyIDs: ids, start: s, end: e, location: loc})
	return []*modelcall.DashboardHeatmapCell{}, nil
}

type fakeKeys struct{ ids []uint }

func (f fakeKeys) LookupIDsByUserID(context.Context, uint) ([]uint, error) { return f.ids, nil }

type fakeRelations map[uint]*modelcall.AuditRelation

func (f fakeRelations) BatchGetRelations(_ context.Context, ids []uint) (map[uint]*modelcall.AuditRelation, error) {
	out := map[uint]*modelcall.AuditRelation{}
	for _, id := range ids {
		if r, ok := f[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func newService(repo *fakeRepo, keys []uint, rel fakeRelations) port.DashboardService {
	if rel == nil {
		rel = fakeRelations{}
	}
	return query.NewDashboardService(repo, fakeKeys{ids: keys}, rel, nil)
}

var (
	admin = port.DashboardScope{Permission: enum.PermissionAdmin, UserID: 1}
	demo  = port.DashboardScope{Permission: enum.PermissionDemo, UserID: 2}
	user  = port.DashboardScope{Permission: enum.PermissionUser, UserID: 9}
)

func TestDashboardScope_Dispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		scope port.DashboardScope
		want  []uint
	}{
		{"admin", admin, nil},
		{"demo", demo, nil},
		{"user", user, []uint{3, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeRepo{}
			svc := newService(repo, []uint{3, 4}, nil)
			if _, err := svc.TopErrors(t.Context(), tc.scope, start, end, 5); err != nil {
				t.Fatalf("TopErrors: %v", err)
			}
			calls := repo.callsOf("errors")
			if len(calls) != 1 {
				t.Fatalf("want 1 repo call, got %d", len(calls))
			}
			got := calls[0].apiKeyIDs
			if (tc.want == nil) != (got == nil) || len(got) != len(tc.want) {
				t.Errorf("apiKeyIDs = %v, want %v", got, tc.want)
			}
		})
	}

	svc := newService(&fakeRepo{}, nil, nil)
	_, err := svc.TopErrors(t.Context(), port.DashboardScope{Permission: "guest"}, start, end, 5)
	if !errors.Is(err, ierr.ErrUnauthorized) {
		t.Errorf("unknown permission err = %v, want ErrUnauthorized", err)
	}
}

func TestDashboardScope_UserWithoutKeysShortCircuits(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, []uint{}, nil)
	ctx := t.Context()

	ov, err := svc.Overview(ctx, user, start, end, enum.GranularityDay)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if ov.Current.Summary == nil || ov.Current.Summary.Requests != 0 {
		t.Errorf("summary = %+v, want zero", ov.Current.Summary)
	}
	if len(ov.Current.Series) != 8 || len(ov.Previous.Series) == 0 {
		t.Errorf("series must still be bucket-filled, got %d/%d", len(ov.Current.Series), len(ov.Previous.Series))
	}
	if _, err := svc.ModelSeries(ctx, user, start, end, enum.GranularityDay); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TopErrors(ctx, user, start, end, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EndpointHealth(ctx, user, start, end, 5); err != nil {
		t.Fatal(err)
	}
	top, err := svc.TopUsage(ctx, user, start, end, enum.DashboardDimensionModel, 5)
	if err != nil || top == nil || len(top.Items) != 0 {
		t.Errorf("TopUsage = (%+v, %v), want empty", top, err)
	}
	if _, err := svc.Heatmap(ctx, user, start, end, "UTC"); err != nil {
		t.Fatal(err)
	}
	if n := repo.callCount(); n != 0 {
		t.Errorf("user without keys must not hit repo, got %d calls", n)
	}
}

func TestDashboardOverview_PreviousPeriod(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, nil, nil)
	if _, err := svc.Overview(t.Context(), admin, start, end, enum.GranularityDay); err != nil {
		t.Fatalf("Overview: %v", err)
	}
	wantPrevStart := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	wantPrevEnd := start.Add(-time.Microsecond)
	var sawCurrent, sawPrevious bool
	for _, c := range repo.callsOf("stats") {
		switch {
		case c.start.Equal(start) && c.end.Equal(end):
			sawCurrent = true
		case c.start.Equal(wantPrevStart) && c.end.Equal(wantPrevEnd):
			sawPrevious = true
		}
	}
	if !sawCurrent || !sawPrevious {
		t.Errorf("stats calls = %+v; want current [%v,%v] and previous [%v,%v]", repo.callsOf("stats"), start, end, wantPrevStart, wantPrevEnd)
	}
}

func TestDashboardOverview_FillsMissingBuckets(t *testing.T) {
	t.Parallel()

	p95 := 1200.0
	repo := &fakeRepo{series: []*modelcall.DashboardStatsPoint{{
		Time:           time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC),
		DashboardStats: modelcall.DashboardStats{Requests: 7, LatencyP95Ms: &p95},
	}}}
	svc := newService(repo, nil, nil)
	ov, err := svc.Overview(t.Context(), admin, start, end, enum.GranularityDay)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	series := ov.Current.Series
	if len(series) != 8 {
		t.Fatalf("len(series) = %d, want 8", len(series))
	}
	if series[2].Requests != 7 || series[2].LatencyP95Ms == nil {
		t.Errorf("bucket 10-23 = %+v, want requests=7", series[2])
	}
	if series[0].Requests != 0 || series[0].LatencyP95Ms != nil {
		t.Errorf("missing bucket = %+v, want zero with nil latency", series[0])
	}
	if !series[0].Time.Equal(start) {
		t.Errorf("first bucket time = %v, want %v", series[0].Time, start)
	}
}

func TestDashboardOverview_EmptyRangeHasNilLatency(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{stats: &modelcall.DashboardStats{}}, nil, nil)
	ov, err := svc.Overview(t.Context(), admin, start, end, enum.GranularityDay)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if ov.Current.Summary.LatencyP95Ms != nil {
		t.Errorf("empty range latency must be nil, got %v", *ov.Current.Summary.LatencyP95Ms)
	}
}

func TestDashboardOverview_RejectsTooManyBuckets(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, nil, nil)
	_, err := svc.Overview(t.Context(), admin, end.AddDate(0, 0, -30), end, enum.GranularityMinute)
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
	if n := repo.callCount(); n != 0 {
		t.Errorf("rejected request must not hit repo, got %d calls", n)
	}
	if _, err := svc.ModelSeries(t.Context(), admin, end.AddDate(0, 0, -30), end, enum.GranularityMinute); !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("ModelSeries err = %v, want ErrValidation", err)
	}
}

func TestDashboardTop_UserDimensionAggregatesByOwner(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{groups: []*modelcall.DashboardGroupRow{
		{Key: "1", Requests: 10, InputTokens: 100, CacheReadTokens: 50, CostMicro: 3000},
		{Key: "2", Requests: 5, OutputTokens: 20, CostMicro: 2000},
		{Key: "3", Requests: 30, CostMicro: 1000},
	}}
	rel := fakeRelations{
		1: {APIKeyID: 1, APIKeyName: "k1", UserID: 9, UserName: "alice"},
		2: {APIKeyID: 2, APIKeyName: "k2", UserID: 9, UserName: "alice"},
		3: {APIKeyID: 3, APIKeyName: "k3", UserID: 8, UserName: "bob"},
	}
	svc := newService(repo, nil, rel)
	top, err := svc.TopUsage(t.Context(), admin, start, end, enum.DashboardDimensionUser, 1)
	if err != nil {
		t.Fatalf("TopUsage: %v", err)
	}
	if calls := repo.callsOf("group"); len(calls) != 1 || calls[0].dimension != enum.DashboardDimensionAPIKey {
		t.Fatalf("user dimension must query apikey groups, calls=%+v", calls)
	}
	if len(top.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(top.Items))
	}
	first := top.Items[0]
	if first.Key != "9" || first.Name != "alice" || first.Requests != 15 || first.CostMicro != 5000 {
		t.Errorf("first = %+v, want alice requests=15 cost=5000", first)
	}
	if first.Tokens != 170 || first.PromptTokens != 150 || first.CacheReadTokens != 50 {
		t.Errorf("token sums = %+v", first)
	}
	if top.OtherRequests != 30 || top.OtherCostMicro != 1000 || top.TotalCostMicro != 6000 {
		t.Errorf("other/total = %d/%d/%d, want 30/1000/6000", top.OtherRequests, top.OtherCostMicro, top.TotalCostMicro)
	}
}

func TestDashboardTop_UserDimensionForbiddenForUser(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, []uint{3}, nil)
	_, err := svc.TopUsage(t.Context(), user, start, end, enum.DashboardDimensionUser, 5)
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Errorf("err = %v, want ErrNoPermission", err)
	}
	if n := repo.callCount(); n != 0 {
		t.Errorf("forbidden request must not hit repo, got %d", n)
	}
}

func TestDashboardTop_APIKeyNamesFromRelations(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{groups: []*modelcall.DashboardGroupRow{{Key: "1", Requests: 2}, {Key: "42", Requests: 1}}}
	svc := newService(repo, nil, fakeRelations{1: {APIKeyID: 1, APIKeyName: "prod-key"}})
	top, err := svc.TopUsage(t.Context(), admin, start, end, enum.DashboardDimensionAPIKey, 5)
	if err != nil {
		t.Fatalf("TopUsage: %v", err)
	}
	if top.Items[0].Name != "prod-key" || top.Items[1].Name != "42" {
		t.Errorf("names = %q/%q, want prod-key/42", top.Items[0].Name, top.Items[1].Name)
	}
}

func TestDashboardTop_DemoMasksNames(t *testing.T) {
	t.Parallel()

	rel := fakeRelations{1: {APIKeyID: 1, APIKeyName: "secret-production-key", UserID: 9, UserName: "alice@example.com"}}
	groups := []*modelcall.DashboardGroupRow{{Key: "1", Requests: 3}}

	userTop, err := newService(&fakeRepo{groups: groups}, nil, rel).TopUsage(t.Context(), demo, start, end, enum.DashboardDimensionUser, 5)
	if err != nil {
		t.Fatalf("user TopUsage: %v", err)
	}
	if got, want := userTop.Items[0].Name, commonutil.MaskIdentity("alice@example.com"); got != want {
		t.Errorf("demo user name = %q, want %q", got, want)
	}
	keyTop, err := newService(&fakeRepo{groups: groups}, nil, rel).TopUsage(t.Context(), demo, start, end, enum.DashboardDimensionAPIKey, 5)
	if err != nil {
		t.Fatalf("apikey TopUsage: %v", err)
	}
	if got, want := keyTop.Items[0].Name, commonutil.MaskSecret("secret-production-key"); got != want {
		t.Errorf("demo key name = %q, want %q", got, want)
	}
}

func TestDashboardEndpoints_DemoMasksEndpoint(t *testing.T) {
	t.Parallel()

	rows := func() []*modelcall.DashboardEndpointRow {
		return []*modelcall.DashboardEndpointRow{{Endpoint: "anthropic-main-endpoint", Requests: 4}}
	}
	demoRows, err := newService(&fakeRepo{endpoints: rows()}, nil, nil).EndpointHealth(t.Context(), demo, start, end, 5)
	if err != nil {
		t.Fatalf("demo EndpointHealth: %v", err)
	}
	if got, want := demoRows[0].Endpoint, commonutil.MaskSecret("anthropic-main-endpoint"); got != want {
		t.Errorf("demo endpoint = %q, want %q", got, want)
	}
	adminRows, err := newService(&fakeRepo{endpoints: rows()}, nil, nil).EndpointHealth(t.Context(), admin, start, end, 5)
	if err != nil {
		t.Fatalf("admin EndpointHealth: %v", err)
	}
	if adminRows[0].Endpoint != "anthropic-main-endpoint" {
		t.Errorf("admin endpoint must stay raw, got %q", adminRows[0].Endpoint)
	}
}

func TestDashboardHeatmap_RejectsInvalidTimezone(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, nil, nil)
	for _, tz := range []string{"UTC'; drop table x", "Mars/Base", ""} {
		if _, err := svc.Heatmap(t.Context(), admin, start, end, tz); !errors.Is(err, ierr.ErrValidation) {
			t.Errorf("tz %q: err = %v, want ErrValidation", tz, err)
		}
	}
	if n := repo.callCount(); n != 0 {
		t.Errorf("invalid tz must not hit repo, got %d", n)
	}
	if _, err := svc.Heatmap(t.Context(), admin, start, end, "Asia/Shanghai"); err != nil {
		t.Fatalf("valid tz: %v", err)
	}
	if calls := repo.callsOf("heatmap"); len(calls) != 1 || calls[0].location != "Asia/Shanghai" {
		t.Errorf("valid tz must pass through, calls=%+v", calls)
	}
}
