// Package dashboard_handler dashboard HTTP 层映射测试（DTO 字段换算与派生值）
package dashboard_handler

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
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

func (f *fakeDashboard) Overview(_ context.Context, s port.DashboardScope, _, _ time.Time, _ enum.Granularity) (*port.DashboardOverview, error) {
	f.scope = s
	return f.overview, nil
}

func (f *fakeDashboard) TopUsage(_ context.Context, s port.DashboardScope, _, _ time.Time, _ enum.DashboardDimension, _ int) (*port.DashboardTop, error) {
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

	stats := &modelcall.DashboardStats{Requests: 10, Success: 6, RateLimitFailures: 2, ServerFailures: 1, CostMicro: 1_500_000}
	f := &fakeDashboard{overview: &port.DashboardOverview{
		Current:  port.DashboardPeriod{Summary: stats, Series: []*modelcall.DashboardStatsPoint{{Time: start, DashboardStats: *stats}}},
		Previous: port.DashboardPeriod{Summary: &modelcall.DashboardStats{}, Series: []*modelcall.DashboardStatsPoint{}},
	}}
	rsp, err := newHandler(f).HandleDashboardOverview(adminCtx(t), &dto.DashboardRangeReq{StartTime: start, EndTime: end, Granularity: enum.GranularityDay})
	if err != nil {
		t.Fatalf("HandleDashboardOverview: %v", err)
	}
	data := rsp.Body.Data
	if data.Currency != enum.CurrencyUSD {
		t.Errorf("currency = %q, want USD", data.Currency)
	}
	sum := data.Current.Summary
	if sum.Cost != 1.5 {
		t.Errorf("cost = %v, want 1.5", sum.Cost)
	}
	if sum.Failures.RateLimit != 2 || sum.Failures.Server != 1 || sum.Failures.Connection != 0 || sum.Failures.Other != 1 {
		t.Errorf("failures = %+v, want rateLimit=2 server=1 connection=0 other=1", sum.Failures)
	}
	if len(data.Current.Series) != 1 || data.Current.Series[0].Failures.Other != 1 {
		t.Errorf("series must carry the same mapping, got %+v", data.Current.Series)
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
		OtherRequests: 2, TotalCostMicro: 1_000_000,
	}}
	req := &dto.DashboardTopReq{StartTime: start, EndTime: end, Limit: 2, Dimension: enum.DashboardDimensionModel}
	rsp, err := newHandler(f).HandleDashboardTop(adminCtx(t), req)
	if err != nil {
		t.Fatalf("HandleDashboardTop: %v", err)
	}
	items := rsp.Body.Data.Items
	if items[0].CostShare != 0.75 || items[0].CacheHitRate == nil || *items[0].CacheHitRate != 0.4 {
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
	if rsp.Body.Data.Items[0].CostShare != 0 {
		t.Errorf("zero total cost must give share 0, got %v", rsp.Body.Data.Items[0].CostShare)
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
