package audit_query

import (
	"context"
	"errors"
	"testing"
	"time"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// ─── fake repository ─────────────────────────────────────

type fakeAuditRepo struct {
	listAllFunc       func(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
	listByAPIKeyIDsFn func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	listAllCalls       int
	listByAPIKeyIDsCnt int
	lastAPIKeyIDs      []uint
}

func (f *fakeAuditRepo) Save(ctx context.Context, a *aggregate.ModelCallAudit) error { return nil }

func (f *fakeAuditRepo) ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listAllCalls++
	if f.listAllFunc != nil {
		return f.listAllFunc(ctx, param, startTime, endTime)
	}
	return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
}

func (f *fakeAuditRepo) ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listByAPIKeyIDsCnt++
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listByAPIKeyIDsFn != nil {
		return f.listByAPIKeyIDsFn(ctx, apiKeyIDs, param, startTime, endTime)
	}
	return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
}

func (f *fakeAuditRepo) BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error) {
	return map[uint]*modelcall.AuditRelation{}, nil
}

func (f *fakeAuditRepo) QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	return nil, nil
}

func (f *fakeAuditRepo) QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error) {
	return nil, nil
}

type fakeAPIKeyIDLookup struct {
	lookupFunc func(ctx context.Context, userID uint) ([]uint, error)
	calls      int
}

func (f *fakeAPIKeyIDLookup) LookupIDsByUserID(ctx context.Context, userID uint) ([]uint, error) {
	f.calls++
	if f.lookupFunc != nil {
		return f.lookupFunc(ctx, userID)
	}
	return nil, nil
}

var _ modelcall.AuditRepository = (*fakeAuditRepo)(nil)

// ─── ListAllAuditLogsHandler 测试 ───────────────────────────

func TestListAllAuditLogs_DefaultsAndClamp(t *testing.T) {
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, _, _ time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.Page != 1 {
				t.Errorf("Page = %d, want 1 (default for 0)", param.Page)
			}
			if param.PageSize != 100 {
				t.Errorf("PageSize = %d, want 100 (clamped from 999)", param.PageSize)
			}
			if param.Sort != enum.SortDesc {
				t.Errorf("Sort = %q, want desc (default)", param.Sort)
			}
			if param.SortField != "created_at" {
				t.Errorf("SortField = %q, want created_at (default)", param.SortField)
			}
			return nil, &model.PageInfo{Page: 1, PageSize: 100}, nil
		},
	}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	if _, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{Page: 0, PageSize: 999}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Errorf("ListAll calls = %d, want 1", repo.listAllCalls)
	}
}

func TestListAllAuditLogs_InvalidSortField(t *testing.T) {
	repo := &fakeAuditRepo{}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	_, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{
		Page: 1, PageSize: 20, SortField: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("err want ErrValidation, got %v", err)
	}
	if repo.listAllCalls != 0 {
		t.Errorf("ListAll should NOT be called on validation error, but called %d times", repo.listAllCalls)
	}
}

func TestListAllAuditLogs_TimeRangePassthrough(t *testing.T) {
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, s, e time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if !s.Equal(start) || !e.Equal(end) {
				t.Errorf("time range mismatch: got [%v, %v], want [%v, %v]", s, e, start, end)
			}
			return nil, &model.PageInfo{}, nil
		},
	}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	if _, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{
		Page: 1, PageSize: 20, StartTime: start, EndTime: end,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// ─── ListAuditLogsByUserHandler 测试 ────────────────────────

func TestListAuditLogsByUser_LoadsUserAPIKeyIDs(t *testing.T) {
	repo := &fakeAuditRepo{
		listByAPIKeyIDsFn: func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if len(apiKeyIDs) != 2 || apiKeyIDs[0] != 10 || apiKeyIDs[1] != 20 {
				t.Errorf("apiKeyIDs = %v, want [10 20]", apiKeyIDs)
			}
			return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
		},
	}
	apiKeyLookup := &fakeAPIKeyIDLookup{
		lookupFunc: func(ctx context.Context, userID uint) ([]uint, error) {
			if userID != 7 {
				t.Errorf("userID = %d, want 7", userID)
			}
			return []uint{10, 20}, nil
		},
	}
	h := auditquery.NewListAuditLogsByUserHandler(repo, apiKeyLookup)
	_, _, err := h.Handle(context.Background(), auditquery.ListAuditLogsByUserQuery{
		UserID: 7, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if apiKeyLookup.calls != 1 {
		t.Errorf("LookupIDsByUserID calls = %d, want 1", apiKeyLookup.calls)
	}
	if repo.listByAPIKeyIDsCnt != 1 {
		t.Errorf("ListByAPIKeyIDs calls = %d, want 1", repo.listByAPIKeyIDsCnt)
	}
}

func TestListAuditLogsByUser_InvalidSortField(t *testing.T) {
	repo := &fakeAuditRepo{}
	h := auditquery.NewListAuditLogsByUserHandler(repo, &fakeAPIKeyIDLookup{})
	_, _, err := h.Handle(context.Background(), auditquery.ListAuditLogsByUserQuery{
		UserID: 1, Page: 1, PageSize: 20, SortField: "drop_table",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("err want ErrValidation, got %v", err)
	}
	if repo.listByAPIKeyIDsCnt != 0 {
		t.Errorf("repo should NOT be called on validation error, but called %d times", repo.listByAPIKeyIDsCnt)
	}
}

// ─── FillTrendSeries / FillRateSeries 测试 ────────────────────────

func TestFillTrendSeries_FillsMissingSlots(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	points := []*modelcall.ModelTrendPoint{
		{Model: "gpt-4", Time: t1, Count: 3},
		{Model: "gpt-4", Time: t3, Count: 5},
		{Model: "claude", Time: t2, Count: 1},
	}
	items := auditquery.FillTrendSeries(points)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for _, it := range items {
		if len(it.Points) != 3 {
			t.Errorf("model %s: points len = %d, want 3 (filled)", it.Model, len(it.Points))
		}
	}
	byModel := map[string]map[time.Time]int{}
	for _, it := range items {
		byModel[it.Model] = map[time.Time]int{}
		for _, p := range it.Points {
			byModel[it.Model][p.Time] = p.Count
		}
	}
	if byModel["gpt-4"][t2] != 0 {
		t.Errorf("gpt-4 missing slot at t2 should be 0, got %d", byModel["gpt-4"][t2])
	}
	if byModel["claude"][t1] != 0 || byModel["claude"][t3] != 0 {
		t.Errorf("claude missing slots should be 0, got t1=%d t3=%d", byModel["claude"][t1], byModel["claude"][t3])
	}
}

func TestFillTrendSeries_Empty(t *testing.T) {
	items := auditquery.FillTrendSeries(nil)
	if len(items) != 0 {
		t.Errorf("empty input should return empty, got %d items", len(items))
	}
}

func TestFillRateSeries_CalculatesSuccessRate(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	points := []*modelcall.RequestRatePoint{
		{Model: "gpt-4", Time: t1, Total: 10, Success: 8},
		{Model: "gpt-4", Time: t2, Total: 5, Success: 5},
	}
	items := auditquery.FillRateSeries(points)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	pts := items[0].Points
	if len(pts) != 2 {
		t.Fatalf("pts len = %d, want 2", len(pts))
	}
	// Sorted by time: t1, t2
	if pts[0].Total != 10 || pts[0].Success != 8 || pts[0].Failed != 2 || pts[0].SuccessRate != 0.8 {
		t.Errorf("pts[0] mismatch: %+v", pts[0])
	}
	if pts[1].Total != 5 || pts[1].Success != 5 || pts[1].Failed != 0 || pts[1].SuccessRate != 1.0 {
		t.Errorf("pts[1] mismatch: %+v", pts[1])
	}
}

// ─── AuditService 派发测试 ────────────────────────

func TestAuditService_DispatchesByPermission(t *testing.T) {
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, _ model.CommonParam, _, _ time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
		listByAPIKeyIDsFn: func(ctx context.Context, _ []uint, _ model.CommonParam, _, _ time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
	}
	svc := auditquery.NewAuditService(
		auditquery.NewListAllAuditLogsHandler(repo),
		auditquery.NewListAuditLogsByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewModelTrendHandler(repo),
		auditquery.NewModelTrendByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewRequestRateHandler(repo),
		auditquery.NewRequestRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
	)

	if _, _, err := svc.ListLogs(context.Background(), enum.PermissionAdmin, 1, auditquery.ListAuditLogsParams{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("admin ListLogs err: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Errorf("admin should call listAll, calls = %d", repo.listAllCalls)
	}

	if _, _, err := svc.ListLogs(context.Background(), enum.PermissionUser, 7, auditquery.ListAuditLogsParams{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("user ListLogs err: %v", err)
	}
	if repo.listByAPIKeyIDsCnt != 1 {
		t.Errorf("user should call listByAPIKeyIDs, calls = %d", repo.listByAPIKeyIDsCnt)
	}

	if _, _, err := svc.ListLogs(context.Background(), enum.Permission("nope"), 7, auditquery.ListAuditLogsParams{Page: 1, PageSize: 20}); !errors.Is(err, ierr.ErrUnauthorized) {
		t.Errorf("unknown permission should return ErrUnauthorized, got %v", err)
	}
}
