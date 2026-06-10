package audit_query

import (
	"context"
	"errors"
	"testing"
	"time"

	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// ─── fake repository ─────────────────────────────────────

type fakeAuditRepo struct {
	listAllFunc            func(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
	listByAPIKeyIDsFn      func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
	queryTokenThroughputFn func(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error)

	listDistinctUserNamesFn   func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	listDistinctModelsFn      func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	listDistinctStatusCodesFn func(ctx context.Context, startTime, endTime time.Time) ([]string, error)

	listAllCalls       int
	listByAPIKeyIDsCnt int
	lastAPIKeyIDs      []uint
}

func (f *fakeAuditRepo) Save(ctx context.Context, a *aggregate.ModelCallAudit) error { return nil }

func (f *fakeAuditRepo) ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listAllCalls++
	if f.listAllFunc != nil {
		return f.listAllFunc(ctx, param, startTime, endTime, criteria)
	}
	return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
}

func (f *fakeAuditRepo) ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listByAPIKeyIDsCnt++
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listByAPIKeyIDsFn != nil {
		return f.listByAPIKeyIDsFn(ctx, apiKeyIDs, param, startTime, endTime, criteria)
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

func (f *fakeAuditRepo) QueryTokenThroughput(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	if f.queryTokenThroughputFn != nil {
		return f.queryTokenThroughputFn(ctx, apiKeyIDs, startTime, endTime, granularity)
	}
	return nil, nil
}

func (f *fakeAuditRepo) QueryFirstTokenLatency(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.FirstTokenLatencyPoint, error) {
	return nil, nil
}

func (f *fakeAuditRepo) ListDistinctUserNames(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	if f.listDistinctUserNamesFn != nil {
		return f.listDistinctUserNamesFn(ctx, keyword, startTime, endTime)
	}
	return []string{}, nil
}

func (f *fakeAuditRepo) ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	if f.listDistinctModelsFn != nil {
		return f.listDistinctModelsFn(ctx, keyword, startTime, endTime)
	}
	return []string{}, nil
}

func (f *fakeAuditRepo) ListDistinctStatusCodes(ctx context.Context, startTime, endTime time.Time) ([]string, error) {
	if f.listDistinctStatusCodesFn != nil {
		return f.listDistinctStatusCodesFn(ctx, startTime, endTime)
	}
	return []string{}, nil
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
	t.Parallel()
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, _, _ time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
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
	t.Parallel()
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
	t.Parallel()
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, s, e time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
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
	t.Parallel()
	repo := &fakeAuditRepo{
		listByAPIKeyIDsFn: func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
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
	t.Parallel()
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

// ─── ListAuditOptionHandler 测试 ────────────────────────

func TestListAuditOption_DispatchesByField(t *testing.T) {
	t.Parallel()

	userNamesCalled, modelsCalled, statusCodesCalled := false, false, false
	repo := &fakeAuditRepo{
		listAllFunc:            nil,
		listByAPIKeyIDsFn:      nil,
		queryTokenThroughputFn: nil,
		listDistinctUserNamesFn: func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
			userNamesCalled = true
			return []string{"user1", "user2"}, nil
		},
		listDistinctModelsFn: func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
			modelsCalled = true
			return []string{"gpt-4", "claude"}, nil
		},
		listDistinctStatusCodesFn: func(ctx context.Context, startTime, endTime time.Time) ([]string, error) {
			statusCodesCalled = true
			return []string{"200", "400", "500"}, nil
		},
	}
	h := auditquery.NewListAuditOptionHandler(repo)

	t.Run("field=user", func(t *testing.T) {
		t.Parallel()
		items, err := h.Handle(context.Background(), auditquery.ListAuditOptionQuery{
			Field: constant.AuditFilterFieldUser,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !userNamesCalled {
			t.Error("expected ListDistinctUserNames to be called")
		}
		if len(items) != 2 || items[0] != "user1" {
			t.Errorf("items = %v, want [user1 user2]", items)
		}
	})

	t.Run("field=model", func(t *testing.T) {
		t.Parallel()
		items, err := h.Handle(context.Background(), auditquery.ListAuditOptionQuery{
			Field: constant.AuditFilterFieldModel,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !modelsCalled {
			t.Error("expected ListDistinctModels to be called")
		}
		if len(items) != 2 || items[0] != "gpt-4" {
			t.Errorf("items = %v, want [gpt-4 claude]", items)
		}
	})

	t.Run("field=status", func(t *testing.T) {
		t.Parallel()
		items, err := h.Handle(context.Background(), auditquery.ListAuditOptionQuery{
			Field: constant.AuditFilterFieldStatus,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !statusCodesCalled {
			t.Error("expected ListDistinctStatusCodes to be called")
		}
		if len(items) != 3 || items[0] != "200" {
			t.Errorf("items = %v, want [200 400 500]", items)
		}
	})

	t.Run("unknown field returns empty", func(t *testing.T) {
		t.Parallel()
		items, err := h.Handle(context.Background(), auditquery.ListAuditOptionQuery{
			Field: "unknown",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("items = %v, want []", items)
		}
	})
}

// ─── FillTrendSeries / FillRateSeries 测试 ────────────────────────

func TestFillTrendSeries_FillsCompleteRequestedRange(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	t4 := t1.Add(3 * time.Hour)
	points := []*modelcall.ModelTrendPoint{
		{Model: "gpt-4", Time: t1, Count: 3},
		{Model: "gpt-4", Time: t3, Count: 5},
		{Model: "claude", Time: t2, Count: 1},
	}
	items := auditquery.FillTrendSeries(points, t1, t4, enum.GranularityHour)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for _, it := range items {
		if len(it.Points) != 4 {
			t.Errorf("model %s: points len = %d, want 4 (complete requested range)", it.Model, len(it.Points))
		}
	}
	byModel := map[string]map[time.Time]int{}
	for _, it := range items {
		byModel[it.Model] = map[time.Time]int{}
		for _, p := range it.Points {
			byModel[it.Model][p.Time] = p.Count
		}
	}
	if byModel["gpt-4"][t2] != 0 || byModel["gpt-4"][t4] != 0 {
		t.Errorf("gpt-4 missing slots should be 0, got t2=%d t4=%d", byModel["gpt-4"][t2], byModel["gpt-4"][t4])
	}
	if byModel["claude"][t1] != 0 || byModel["claude"][t3] != 0 || byModel["claude"][t4] != 0 {
		t.Errorf("claude missing slots should be 0, got t1=%d t3=%d t4=%d", byModel["claude"][t1], byModel["claude"][t3], byModel["claude"][t4])
	}
}

func TestFillTrendSeries_Empty(t *testing.T) {
	t.Parallel()
	items := auditquery.FillTrendSeries(nil, time.Time{}, time.Time{}, enum.GranularityHour)
	if len(items) != 0 {
		t.Errorf("empty input should return empty, got %d items", len(items))
	}
}

func TestFillRateSeries_CalculatesSuccessRate(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	points := []*modelcall.RequestRatePoint{
		{Model: "gpt-4", Time: t1, Total: 10, Success: 8},
		{Model: "gpt-4", Time: t2, Total: 5, Success: 5},
	}
	items := auditquery.FillRateSeries(points, t1, t2, enum.GranularityHour)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	pts := items[0].Points
	if len(pts) != 2 {
		t.Fatalf("pts len = %d, want 2", len(pts))
	}
	if pts[0].Total != 10 || pts[0].Success != 8 || pts[0].Failed != 2 || pts[0].SuccessRate != 0.8 {
		t.Errorf("pts[0] mismatch: %+v", pts[0])
	}
	if pts[1].Total != 5 || pts[1].Success != 5 || pts[1].Failed != 0 || pts[1].SuccessRate != 1.0 {
		t.Errorf("pts[1] mismatch: %+v", pts[1])
	}
}

func TestFillRateSeries_MatchesDBBucketAcrossTimeZones(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 6, 1, 9, 4, 59, 0, time.UTC)
	end := time.Date(2026, 6, 1, 9, 59, 59, 0, time.UTC)
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	points := []*modelcall.RequestRatePoint{
		{Model: "gpt-4", Time: time.Date(2026, 6, 1, 17, 0, 0, 0, shanghai), Total: 10, Success: 8},
	}

	items := auditquery.FillRateSeries(points, start, end, enum.GranularityHour)

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if len(items[0].Points) != 1 {
		t.Fatalf("points len = %d, want 1", len(items[0].Points))
	}
	pt := items[0].Points[0]
	if pt.Total != 10 || pt.Success != 8 || pt.Failed != 2 || pt.SuccessRate != 0.8 {
		t.Fatalf("timezone-equivalent bucket lost aggregate data: %+v", pt)
	}
}

func TestFillTokenThroughputSeries_FillsCompleteRequestedRange(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	points := []*modelcall.TokenThroughputPoint{
		{Model: "gpt-4", Time: t1, OutputTokens: 20},
	}
	items := auditquery.FillTokenThroughputSeries(points, t1, t3, enum.GranularityHour)
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	pts := items
	if len(pts) != 3 {
		t.Fatalf("pts len = %d, want 3", len(pts))
	}
	if !pts[1].Time.Equal(t2) || pts[1].OutputTokens != 0 {
		t.Errorf("missing token bucket mismatch: %+v", pts[1])
	}
}

func TestFillTokenThroughputSeries_MatchesDBBucketAcrossTimeZones(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 6, 1, 9, 4, 59, 0, time.UTC)
	end := time.Date(2026, 6, 1, 9, 59, 59, 0, time.UTC)
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	points := []*modelcall.TokenThroughputPoint{
		{
			Model:               "gpt-4",
			Time:                time.Date(2026, 6, 1, 17, 0, 0, 0, shanghai),
			InputTokens:         11,
			OutputTokens:        22,
			CacheCreationTokens: 33,
			CacheReadTokens:     44,
		},
	}

	items := auditquery.FillTokenThroughputSeries(points, start, end, enum.GranularityHour)

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	pt := items[0]
	if pt.InputTokens != 11 || pt.OutputTokens != 22 || pt.CacheCreationTokens != 33 || pt.CacheReadTokens != 44 {
		t.Fatalf("timezone-equivalent bucket lost token aggregate data: %+v", pt)
	}
}

// ─── AuditService 派发测试 ────────────────────────

func TestAuditService_DispatchesByPermission(t *testing.T) {
	t.Parallel()
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, _ model.CommonParam, _, _ time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
		listByAPIKeyIDsFn: func(ctx context.Context, _ []uint, _ model.CommonParam, _, _ time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
	}
	svc := auditquery.NewAuditService(
		auditquery.NewListAllAuditLogsHandler(repo),
		auditquery.NewListAuditLogsByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewListAuditOptionHandler(repo),
		auditquery.NewModelTrendHandler(repo),
		auditquery.NewModelTrendByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewRequestRateHandler(repo),
		auditquery.NewRequestRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenThroughputHandler(repo),
		auditquery.NewTokenThroughputByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenRateHandler(repo),
		auditquery.NewTokenRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewModelUsageHandler(repo),
		auditquery.NewModelUsageByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewFirstTokenLatencyHandler(repo),
		auditquery.NewFirstTokenLatencyByUserHandler(repo, &fakeAPIKeyIDLookup{}),
	)

	if _, _, err := svc.ListLogs(context.Background(), enum.PermissionAdmin, 1, auditport.ListAuditLogsParams{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("admin ListLogs err: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Errorf("admin should call listAll, calls = %d", repo.listAllCalls)
	}

	if _, _, err := svc.ListLogs(context.Background(), enum.PermissionUser, 7, auditport.ListAuditLogsParams{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("user ListLogs err: %v", err)
	}
	if repo.listByAPIKeyIDsCnt != 1 {
		t.Errorf("user should call listByAPIKeyIDs, calls = %d", repo.listByAPIKeyIDsCnt)
	}

	if _, _, err := svc.ListLogs(context.Background(), enum.Permission("nope"), 7, auditport.ListAuditLogsParams{Page: 1, PageSize: 20}); !errors.Is(err, ierr.ErrUnauthorized) {
		t.Errorf("unknown permission should return ErrUnauthorized, got %v", err)
	}
}

// ─── ModelUsage 聚合测试 ────────────────────────

func TestAggregateModelUsage_SumsPerModel(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	repo := &fakeAuditRepo{
		queryTokenThroughputFn: func(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
			return []*modelcall.TokenThroughputPoint{
				{Model: "gpt-4", Time: t1, InputTokens: 100, OutputTokens: 50, CacheReadTokens: 30, CacheCreationTokens: 10},
				{Model: "gpt-4", Time: t2, InputTokens: 200, OutputTokens: 150, CacheReadTokens: 20, CacheCreationTokens: 5},
				{Model: "claude", Time: t1, InputTokens: 300, OutputTokens: 250, CacheReadTokens: 50, CacheCreationTokens: 15},
			}, nil
		},
	}
	h := auditquery.NewModelUsageHandler(repo)
	items, err := h.Handle(context.Background(), auditquery.ModelUsageQuery{
		StartTime: t1, EndTime: t2, Granularity: enum.GranularityHour,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	gpt := items[0]
	if gpt.InputTokens != 300 || gpt.OutputTokens != 200 || gpt.CacheReadTokens != 50 || gpt.CacheCreationTokens != 15 {
		t.Errorf("gpt-4 totals mismatch: %+v", gpt)
	}
	claude := items[1]
	if claude.InputTokens != 300 || claude.OutputTokens != 250 || claude.CacheReadTokens != 50 || claude.CacheCreationTokens != 15 {
		t.Errorf("claude totals mismatch: %+v", claude)
	}
}
