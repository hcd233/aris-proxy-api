// Package dashboard_handler dashboard HTTP 层映射测试（DTO 字段换算与派生值）
package dashboard_handler

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-mass/internal/application/audit/port"
	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/domain/modelcall"
	"github.com/hcd233/aris-mass/internal/dto"
	"github.com/hcd233/aris-mass/internal/handler"
)

var (
	start = time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	end   = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
)

// fakeDashboard 返回预置结果，并记录收到的 scope
type fakeDashboard struct {
	port.DashboardService
	overview *port.DashboardOverview
	top      *port.DashboardTop
	heatmap  []*modelcall.DashboardHeatmapCell
	scope    port.DashboardScope
}

func (f *fakeDashboard) Overview(_ context.Context, s port.DashboardScope, _, _ time.Time, _ enum.Granularity, _ int) (*port.DashboardOverview, error) {
	f.scope = s
	return f.overview, nil
}

func (f *fakeDashboard) TopUsage(_ context.Context, s port.DashboardScope, _, _ time.Time, _ enum.DashboardDimension, _ enum.DashboardMetric, _ int) (*port.DashboardTop, error) {
	f.scope = s
	return f.top, nil
}

func (f *fakeDashboard) Heatmap(_ context.Context, s port.DashboardScope, _, _ time.Time, _ string) ([]*modelcall.DashboardHeatmapCell, error) {
	f.scope = s
	return f.heatmap, nil
}

func adminCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := context.WithValue(t.Context(), constant.CtxKeyPermission, enum.PermissionAdmin)
	return context.WithValue(ctx, constant.CtxKeyUserID, uint(7))
}

func newHandler(f *fakeDashboard) handler.AuditHandler {
	return handler.NewAuditHandler(handler.AuditDependencies{Dashboard: f})
}

func TestDashboardHandler_OverviewMapsCostAndFailures(t *testing.T) {
	t.Parallel()

	stats := &modelcall.DashboardStats{
		Requests: 12, Success: 6, CostMicro: 1_500_000,
		Failures: []*modelcall.DashboardFailureCount{{StatusCode: 429, Count: 3}, {StatusCode: 502, Count: 2}, {StatusCode: -1, Count: 1}},
	}
	f := &fakeDashboard{overview: &port.DashboardOverview{
		Current:  port.DashboardPeriod{Summary: stats, Series: []*modelcall.DashboardStatsPoint{{Time: start, DashboardStats: modelcall.DashboardStats{Requests: 12, Success: 6}}}},
		Previous: port.DashboardPeriod{Summary: &modelcall.DashboardStats{}, Series: []*modelcall.DashboardStatsPoint{}},
		Uptime: []*modelcall.DashboardUptimePoint{{
			Time:     start,
			End:      end,
			Requests: 5,
			Success:  3,
			Failures: []*modelcall.DashboardFailureCount{{StatusCode: 503, Count: 2}},
		}},
	}}
	rsp, err := newHandler(f).HandleDashboardOverview(adminCtx(t), &dto.DashboardOverviewReq{
		StartTime:     start,
		EndTime:       end,
		Granularity:   enum.GranularityDay,
		UptimeBuckets: 32,
	})
	if err != nil {
		t.Fatalf("HandleDashboardOverview: %v", err)
	}
	data := rsp.Body.Data
	if data.Currency != enum.CurrencyUSD {
		t.Errorf("currency = %q, want USD", data.Currency)
	}
	if len(data.Uptime) != 1 {
		t.Fatalf("uptime = %+v, want 1 bucket", data.Uptime)
	}
	uptime := data.Uptime[0]
	if uptime.Requests != 5 || uptime.Success != 3 || len(uptime.Failures) != 1 ||
		uptime.Failures[0].StatusCode != 503 || uptime.Failures[0].Count != 2 {
		t.Errorf("uptime bucket = %+v, want requests=5 success=3 failures=[503×2]", uptime)
	}
	sum := data.Current.Summary
	if sum.Cost != 1.5 {
		t.Errorf("cost = %v, want 1.5", sum.Cost)
	}
	wantCodes := []int{429, 502, -1}
	if len(sum.Failures) != len(wantCodes) {
		t.Fatalf("failures = %+v, want %d codes", sum.Failures, len(wantCodes))
	}
	for i, code := range wantCodes {
		if sum.Failures[i].StatusCode != code {
			t.Errorf("failures[%d].statusCode = %d, want %d (order must be preserved)", i, sum.Failures[i].StatusCode, code)
		}
	}
	if len(data.Current.Series) != 1 || data.Current.Series[0].Failures == nil || len(data.Current.Series[0].Failures) != 0 {
		t.Errorf("series failures must be a non-nil empty slice, got %+v", data.Current.Series)
	}
	if data.Previous.Series == nil {
		t.Error("previous series must be a non-nil empty slice")
	}
	if f.scope.Permission != enum.PermissionAdmin || f.scope.UserID != 7 {
		t.Errorf("scope = %+v, want admin/7", f.scope)
	}
}

func TestDashboardHandler_TopComputesShareAndHitRate(t *testing.T) {
	t.Parallel()

	f := &fakeDashboard{top: &port.DashboardTop{
		Items: []*port.DashboardTopItem{
			{Key: "a", Name: "a", Requests: 3, PromptTokens: 100, CacheReadTokens: 40, CostMicro: 750_000},
			{Key: "b", Name: "b", Requests: 1},
		},
		OtherRequests: 2, TotalCostMicro: 1_000_000, TotalMetric: 1_000_000,
	}}
	req := &dto.DashboardTopReq{StartTime: start, EndTime: end, Limit: 2, Dimension: enum.DashboardDimensionModel, Metric: enum.DashboardMetricCost}
	rsp, err := newHandler(f).HandleDashboardTop(adminCtx(t), req)
	if err != nil {
		t.Fatalf("HandleDashboardTop: %v", err)
	}
	items := rsp.Body.Data.Items
	if items[0].CostShare != 0.75 || items[0].MetricShare != 0.75 || items[0].CacheHitRate == nil || *items[0].CacheHitRate != 0.4 {
		t.Errorf("first = %+v, want share 0.75 hit 0.4", items[0])
	}
	if items[1].CacheHitRate != nil {
		t.Errorf("zero prompt tokens must give nil hit rate, got %v", *items[1].CacheHitRate)
	}
	if rsp.Body.Data.OtherRequests != 2 {
		t.Errorf("otherRequests = %d, want 2", rsp.Body.Data.OtherRequests)
	}

	f.top = &port.DashboardTop{Items: []*port.DashboardTopItem{{Key: "a", Requests: 1}}}
	rsp, err = newHandler(f).HandleDashboardTop(adminCtx(t), req)
	if err != nil {
		t.Fatalf("HandleDashboardTop(zero cost): %v", err)
	}
	if rsp.Body.Data.Items[0].CostShare != 0 || rsp.Body.Data.Items[0].MetricShare != 0 {
		t.Errorf("zero total must give share 0, got %v/%v", rsp.Body.Data.Items[0].CostShare, rsp.Body.Data.Items[0].MetricShare)
	}
}

func TestDashboardHandler_TopMetricShareFollowsRequests(t *testing.T) {
	t.Parallel()

	f := &fakeDashboard{top: &port.DashboardTop{
		Items:          []*port.DashboardTopItem{{Key: "a", Name: "a", Requests: 30, CostMicro: 1_000_000}},
		TotalCostMicro: 4_000_000, TotalMetric: 40,
	}}
	req := &dto.DashboardTopReq{StartTime: start, EndTime: end, Limit: 5, Dimension: enum.DashboardDimensionModel, Metric: enum.DashboardMetricRequests}
	rsp, err := newHandler(f).HandleDashboardTop(adminCtx(t), req)
	if err != nil {
		t.Fatalf("HandleDashboardTop: %v", err)
	}
	item := rsp.Body.Data.Items[0]
	if item.MetricShare != 0.75 || item.CostShare != 0.25 {
		t.Errorf("metric share = %v (want 0.75 by requests), cost share = %v (want 0.25)", item.MetricShare, item.CostShare)
	}
}

func TestDashboardHandler_HeatmapWeekdayZeroBased(t *testing.T) {
	t.Parallel()

	f := &fakeDashboard{heatmap: []*modelcall.DashboardHeatmapCell{{Weekday: 1, Hour: 9, Count: 3}, {Weekday: 7, Hour: 23, Count: 1}}}
	rsp, err := newHandler(f).HandleDashboardHeatmap(adminCtx(t), &dto.DashboardHeatmapReq{StartTime: start, EndTime: end, TZ: "UTC"})
	if err != nil {
		t.Fatalf("HandleDashboardHeatmap: %v", err)
	}
	cells := rsp.Body.Data
	if cells[0].Weekday != 0 || cells[1].Weekday != 6 || cells[1].Hour != 23 {
		t.Errorf("cells = %+v, want weekday 0 and 6", cells)
	}
}
