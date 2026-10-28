package audit_query

import (
	"context"
	"errors"
	"strings"
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
	"github.com/samber/lo"
)

// ─── fake repository ─────────────────────────────────────

type fakeAuditRepo struct {
	listAllFunc       func(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
	listByAPIKeyIDsFn func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	listDistinctUserNamesFn   func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	listDistinctModelsFn      func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	listDistinctStatusCodesFn func(ctx context.Context, startTime, endTime time.Time) ([]string, error)
	listDistinctUserAgentsFn  func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	batchGetRelationsFn       func(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error)

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
	if f.batchGetRelationsFn != nil {
		return f.batchGetRelationsFn(ctx, apiKeyIDs)
	}
	return map[uint]*modelcall.AuditRelation{}, nil
}

func (f *fakeAuditRepo) ListDistinctUserNames(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error) {
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listDistinctUserNamesFn != nil {
		return f.listDistinctUserNamesFn(ctx, keyword, startTime, endTime)
	}
	return []string{}, nil
}

func (f *fakeAuditRepo) ListDistinctModels(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error) {
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listDistinctModelsFn != nil {
		return f.listDistinctModelsFn(ctx, keyword, startTime, endTime)
	}
	return []string{}, nil
}

func (f *fakeAuditRepo) ListDistinctStatusCodes(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]string, error) {
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listDistinctStatusCodesFn != nil {
		return f.listDistinctStatusCodesFn(ctx, startTime, endTime)
	}
	return []string{}, nil
}

func (f *fakeAuditRepo) ListDistinctUserAgents(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error) {
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listDistinctUserAgentsFn != nil {
		return f.listDistinctUserAgentsFn(ctx, keyword, startTime, endTime)
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
			if param.PageSize != 500 {
				t.Errorf("PageSize = %d, want 500 (clamped from 999)", param.PageSize)
			}
			if param.Sort != enum.SortAsc {
				t.Errorf("Sort = %q, want asc (default)", param.Sort)
			}
			if param.SortField != "id" {
				t.Errorf("SortField = %q, want id (default)", param.SortField)
			}
			return nil, &model.PageInfo{Page: 1, PageSize: 500}, nil
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

func TestListAllAuditLogs_UAFilterParsing(t *testing.T) {
	t.Parallel()
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, _, _ time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if criteria == nil || len(criteria.Filters) != 1 {
				t.Fatalf("criteria.Filters = %v, want exactly 1 filter", criteria)
			}
			f := criteria.Filters[0]
			if f.Field != constant.AuditFilterFieldUA {
				t.Errorf("filter field = %q, want %q", f.Field, constant.AuditFilterFieldUA)
			}
			// 引号包裹的含空格 UA 值应按单值保留，| 连接的多值正确拆分
			if len(f.Values) != 2 || f.Values[0] != "Mozilla/5.0 (Macintosh)" || f.Values[1] != "curl/8.0" {
				t.Errorf("filter values = %v, want [Mozilla/5.0 (Macintosh) curl/8.0]", f.Values)
			}
			// 验证 ua 字段配置映射到 user_agent 列并生成 fuzzy LIKE SQL
			sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
			if err != nil {
				t.Fatalf("ToSQL err: %v", err)
			}
			if !strings.Contains(sql, "user_agent LIKE ?") || !strings.Contains(sql, " OR ") {
				t.Errorf("sql = %q, want user_agent LIKE ? joined by OR", sql)
			}
			if len(args) != 2 || args[0] != "%Mozilla/5.0 (Macintosh)%" || args[1] != "%curl/8.0%" {
				t.Errorf("args = %v, want %%...%% wrapped LIKE values", args)
			}
			return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
		},
	}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	if _, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{
		Page: 1, PageSize: 20,
		Filter: `ua:"Mozilla/5.0 (Macintosh)"|curl/8.0`,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

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

	userNamesCalled, modelsCalled, statusCodesCalled, userAgentsCalled := false, false, false, false
	repo := &fakeAuditRepo{
		listAllFunc:       nil,
		listByAPIKeyIDsFn: nil,
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
		listDistinctUserAgentsFn: func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
			userAgentsCalled = true
			return []string{"Mozilla/5.0", "curl/8.0"}, nil
		},
	}
	h := auditquery.NewListAuditOptionHandler(repo, &fakeAPIKeyIDLookup{})

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

	t.Run("field=ua", func(t *testing.T) {
		t.Parallel()
		items, err := h.Handle(context.Background(), auditquery.ListAuditOptionQuery{
			Field: constant.AuditFilterFieldUA,
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !userAgentsCalled {
			t.Error("expected ListDistinctUserAgents to be called")
		}
		if len(items) != 2 || items[0] != "Mozilla/5.0" {
			t.Errorf("items = %v, want [Mozilla/5.0 curl/8.0]", items)
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

func TestAuditService_DemoOptionsUseFullQuery(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{
		listDistinctUserAgentsFn: func(_ context.Context, _ string, _ time.Time, _ time.Time) ([]string, error) {
			return []string{"full-agent"}, nil
		},
	}
	svc := auditquery.NewAuditService(
		nil,
		nil,
		auditquery.NewListAuditOptionHandler(repo, &fakeAPIKeyIDLookup{}),
	)

	items, err := svc.ListAuditOption(context.Background(), enum.PermissionDemo, 0, constant.AuditFilterFieldUA, "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list demo audit options: %v", err)
	}
	if len(items) != 1 || items[0] != "full-agent" {
		t.Fatalf("items = %v, want [full-agent]", items)
	}
}

// ─── FillTrendSeries / FillRateSeries 测试 ────────────────────────

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
		auditquery.NewListAuditOptionHandler(repo, &fakeAPIKeyIDLookup{}),
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

// ─── ByUser 指标查询：无 API Key 用户不得越权 ───────────────────

// TestListAuditOption_UserScopeRestricted user 视角的筛选选项必须限制在名下
// key 范围内：UserID 非零时先 lookup 名下 key 并传递给仓储（2026-08-26 越权
// 修复回归——此前 user 视角返回全平台维度选项）。
func TestListAuditOption_UserScopeRestricted(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{
		listDistinctUserNamesFn: func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
			return []string{"user1", "user2"}, nil
		},
	}
	lookup := &fakeAPIKeyIDLookup{
		lookupFunc: func(ctx context.Context, userID uint) ([]uint, error) {
			if userID != 42 {
				t.Errorf("lookup userID = %d, want 42", userID)
			}
			return []uint{7, 8}, nil
		},
	}
	h := auditquery.NewListAuditOptionHandler(repo, lookup)

	items, err := h.Handle(t.Context(), auditquery.ListAuditOptionQuery{
		Field:  constant.AuditFilterFieldUser,
		UserID: lo.ToPtr(uint(42)),
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %v, want 2 entries", items)
	}
	if len(repo.lastAPIKeyIDs) != 2 || repo.lastAPIKeyIDs[0] != 7 || repo.lastAPIKeyIDs[1] != 8 {
		t.Errorf("lastAPIKeyIDs = %v, want [7 8]", repo.lastAPIKeyIDs)
	}
	if lookup.calls != 1 {
		t.Errorf("lookup calls = %d, want 1", lookup.calls)
	}
}

// TestListAuditOption_UserWithoutKeysReturnsEmpty 名下无 key 的 user 视角
// 必须返回空选项且不触达仓储（不得退化为全平台维度）。
func TestListAuditOption_UserWithoutKeysReturnsEmpty(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	lookup := &fakeAPIKeyIDLookup{
		lookupFunc: func(ctx context.Context, userID uint) ([]uint, error) {
			return []uint{}, nil
		},
	}
	h := auditquery.NewListAuditOptionHandler(repo, lookup)

	items, err := h.Handle(t.Context(), auditquery.ListAuditOptionQuery{
		Field:  constant.AuditFilterFieldModel,
		UserID: lo.ToPtr(uint(42)),
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("items = %v, want empty", items)
	}
	if repo.lastAPIKeyIDs != nil {
		t.Errorf("repo must not be reached, lastAPIKeyIDs = %v", repo.lastAPIKeyIDs)
	}
}

// TestListAuditOption_AdminKeepsFullScope admin/demo 视角（UserID=nil）不触发
// lookup，key 范围为 nil（全量）。
func TestListAuditOption_AdminKeepsFullScope(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	lookup := &fakeAPIKeyIDLookup{}
	h := auditquery.NewListAuditOptionHandler(repo, lookup)

	if _, err := h.Handle(t.Context(), auditquery.ListAuditOptionQuery{
		Field: constant.AuditFilterFieldModel,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if lookup.calls != 0 {
		t.Errorf("lookup calls = %d, want 0 (admin must not lookup)", lookup.calls)
	}
	if repo.lastAPIKeyIDs != nil {
		t.Errorf("lastAPIKeyIDs = %v, want nil (full scope)", repo.lastAPIKeyIDs)
	}
}

// TestListAuditOption_ZeroUserReturnsEmpty user 视角 userID=0（认证缺失）必须
// 显式短路返回空选项，不得退化为 admin 全量视角，也不得触达 lookup/仓储。
func TestListAuditOption_ZeroUserReturnsEmpty(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	lookup := &fakeAPIKeyIDLookup{}
	h := auditquery.NewListAuditOptionHandler(repo, lookup)

	items, err := h.Handle(t.Context(), auditquery.ListAuditOptionQuery{
		Field:  constant.AuditFilterFieldModel,
		UserID: lo.ToPtr(uint(0)),
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("items = %v, want empty for userID=0", items)
	}
	if lookup.calls != 0 {
		t.Errorf("lookup calls = %d, want 0 (short-circuit before lookup)", lookup.calls)
	}
	if repo.lastAPIKeyIDs != nil {
		t.Errorf("lastAPIKeyIDs = %v, want nil (must not fall back to full scope)", repo.lastAPIKeyIDs)
	}
}
