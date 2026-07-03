// Package session_option_list 验证 session 筛选选项接口的字段分发逻辑。
package session_option_list

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

type fakeSessionReadRepo struct {
	listDistinctModelsCalled bool
	listDistinctScoresCalled bool
}

func (r *fakeSessionReadRepo) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *fakeSessionReadRepo) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *fakeSessionReadRepo) GetSessionDetail(ctx context.Context, id uint) (*session.SessionDetailProjection, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) GetSessionMeta(ctx context.Context, id uint) (*session.SessionMetaProjection, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) FindMessagesByIDs(ctx context.Context, ids []uint) ([]*session.MessageDetailProjection, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) FindToolsByIDs(ctx context.Context, ids []uint) ([]*session.ToolDetailProjection, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) ListDistinctScores(ctx context.Context, startTime, endTime time.Time) ([]int, error) {
	r.listDistinctScoresCalled = true
	return []int{1, 3, 5}, nil
}

func (r *fakeSessionReadRepo) ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	r.listDistinctModelsCalled = true
	return []string{"gpt-4o", "claude-3-5-sonnet"}, nil
}

func (r *fakeSessionReadRepo) ListSessionsForExport(ctx context.Context, f session.ExportFilter) ([]*session.ExportSessionRow, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) PreviewExport(ctx context.Context, f session.ExportFilter) (*session.ExportPreview, error) {
	return nil, nil
}

func TestListSessionOptionHandler_FieldModel(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.SessionFilterFieldModel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listDistinctModelsCalled {
		t.Error("expected ListDistinctModels to be called")
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestListSessionOptionHandler_FieldScore(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.FieldScore})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listDistinctScoresCalled {
		t.Error("expected ListDistinctScores to be called")
	}
	if len(items) != 4 {
		t.Errorf("expected 4 items (unscored + 3 scores), got %d", len(items))
	}
}

func TestListSessionOptionHandler_UnknownField(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty items, got %d", len(items))
	}
}
