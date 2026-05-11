package audit_query

import (
	"context"
	"testing"
	"time"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

type mockAuditRepository struct {
	listFunc func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

func (m *mockAuditRepository) Save(ctx context.Context, audit *aggregate.ModelCallAudit) error {
	return nil
}

func (m *mockAuditRepository) ListByAPIKeyID(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, apiKeyID, param, startTime, endTime)
	}
	return nil, &model.PageInfo{}, nil
}

var _ modelcall.AuditRepository = (*mockAuditRepository)(nil)

func TestListAuditLogsHandler_DefaultValues(t *testing.T) {
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.Page != 1 {
				t.Errorf("Page = %d, want 1 (default)", param.Page)
			}
			if param.PageSize != 20 {
				t.Errorf("PageSize = %d, want 20 (default)", param.PageSize)
			}
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, pageInfo, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID: 1,
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if pageInfo.Page != 1 {
		t.Errorf("Page = %d, want 1", pageInfo.Page)
	}
}

func TestListAuditLogsHandler_PageSizeClamp(t *testing.T) {
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.PageSize != 100 {
				t.Errorf("expected PageSize clamped to 100, got %d", param.PageSize)
			}
			return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID: 1,
		PageSize: 200,
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestListAuditLogsHandler_PageSizeMinimum(t *testing.T) {
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.PageSize != 20 {
				t.Errorf("expected PageSize default 20 for invalid value, got %d", param.PageSize)
			}
			return nil, &model.PageInfo{Page: 1, PageSize: 20}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID: 1,
		PageSize: 0,
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestListAuditLogsHandler_InvalidSortField(t *testing.T) {
	repo := &mockAuditRepository{}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID:  1,
		SortField: "nonexistent_field",
	})
	if err == nil {
		t.Fatal("expected error for invalid sort field, got nil")
	}
}

func TestListAuditLogsHandler_ValidSortField(t *testing.T) {
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.SortField != "input_tokens" {
				t.Errorf("SortField = %q, want input_tokens", param.SortField)
			}
			return nil, &model.PageInfo{}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID:  1,
		SortField: "input_tokens",
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestListAuditLogsHandler_DefaultSortDesc(t *testing.T) {
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.Sort != enum.SortDesc {
				t.Errorf("Sort = %q, want desc", param.Sort)
			}
			if param.SortField != "created_at" {
				t.Errorf("SortField = %q, want created_at", param.SortField)
			}
			return nil, &model.PageInfo{}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID: 1,
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestListAuditLogsHandler_RepoError(t *testing.T) {
	repoErr := ierr.New(ierr.ErrDBQuery, "db error")
	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, nil, repoErr
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID: 1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAuditLogsHandler_TimeRangePassthrough(t *testing.T) {
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	repo := &mockAuditRepository{
		listFunc: func(ctx context.Context, apiKeyID uint, param model.CommonParam, s time.Time, e time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if !s.Equal(start) {
				t.Errorf("startTime = %v, want %v", s, start)
			}
			if !e.Equal(end) {
				t.Errorf("endTime = %v, want %v", e, end)
			}
			return nil, &model.PageInfo{}, nil
		},
	}
	handler := auditquery.NewListAuditLogsHandler(repo)

	_, _, err := handler.Handle(context.Background(), auditquery.ListAuditLogsQuery{
		APIKeyID:  1,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}
