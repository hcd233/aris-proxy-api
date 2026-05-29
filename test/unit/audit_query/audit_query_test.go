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
