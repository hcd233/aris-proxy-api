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

// 注：ListAuditLogsByUserHandler 还需要 *dao.ProxyAPIKeyDAO 与 *gorm.DB 依赖。
// 单元测试无法替换 *dao.ProxyAPIKeyDAO（具体类型，无法 mock），因此 ByUser handler 的
// SQL 执行路径无法在纯单元测试中覆盖。这里只覆盖：
//   - 参数清洗（SortField 非法时返回 ErrValidation 且不调任何 IO）
// SQL 路径正确性留给 plan 后的本地手工冒烟测试验证。

func TestListAuditLogsByUser_InvalidSortField(t *testing.T) {
	repo := &fakeAuditRepo{}
	// db / apiKeyDAO 在 SortField 非法时不会被调用，可以传 nil
	h := auditquery.NewListAuditLogsByUserHandler(repo, nil, nil)
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
