// Package session_option_list 验证 session 筛选选项接口的字段分发与视角隔离逻辑。
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
	listDistinctModelsCalled  bool
	listDistinctScoresCalled  bool
	listMessageCountStatsCall int
	lastSessionIDs            []uint
	// lastOwnerNames 最近一次 ListDistinct* 调用收到的 owner 范围（nil=admin/demo 全量）
	lastOwnerNames []string
}

func (r *fakeSessionReadRepo) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *fakeSessionReadRepo) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *fakeSessionReadRepo) ListSessionsByIDs(ctx context.Context, ids []uint, param model.CommonParam) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
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

func (r *fakeSessionReadRepo) ListDistinctScores(ctx context.Context, ownerNames []string, startTime, endTime time.Time, sessionIDs []uint) ([]int, error) {
	r.listDistinctScoresCalled = true
	r.lastSessionIDs = sessionIDs
	r.lastOwnerNames = ownerNames
	return []int{1, 3, 5}, nil
}

func (r *fakeSessionReadRepo) ListDistinctModels(ctx context.Context, ownerNames []string, keyword string, startTime, endTime time.Time, sessionIDs []uint) ([]string, error) {
	r.listDistinctModelsCalled = true
	r.lastSessionIDs = sessionIDs
	r.lastOwnerNames = ownerNames
	return []string{"gpt-4o", "claude-3-5-sonnet"}, nil
}

func (r *fakeSessionReadRepo) ListMessageCountStats(ctx context.Context, ownerNames []string, startTime, endTime time.Time, sessionIDs []uint) (maxCount int, bucketCounts map[int]int64, err error) {
	r.listMessageCountStatsCall++
	r.lastSessionIDs = sessionIDs
	r.lastOwnerNames = ownerNames
	return 82, map[int]int64{0: 5, 1: 3, 2: 7}, nil
}

func (r *fakeSessionReadRepo) ListSessionsForExport(ctx context.Context, f session.ExportFilter) ([]*session.ExportSessionRow, error) {
	return nil, nil
}

func (r *fakeSessionReadRepo) PreviewExport(ctx context.Context, f session.ExportFilter) (*session.ExportPreview, error) {
	return nil, nil
}

type fakeOwnerNameLookup struct {
	lookupFunc func(ctx context.Context, userID uint) ([]string, error)
	calls      int
}

func (f *fakeOwnerNameLookup) LookupOwnerNamesByUserID(ctx context.Context, userID uint) ([]string, error) {
	f.calls++
	if f.lookupFunc != nil {
		return f.lookupFunc(ctx, userID)
	}
	return nil, nil
}

func TestListSessionOptionHandler_FieldModel(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo, &fakeOwnerNameLookup{})
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.SessionFilterFieldModel, IsAdmin: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listDistinctModelsCalled {
		t.Error("expected ListDistinctModels to be called")
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if repo.lastOwnerNames != nil {
		t.Errorf("admin scope must pass nil ownerNames, got %v", repo.lastOwnerNames)
	}
}

func TestListSessionOptionHandler_PassesSessionIDs(t *testing.T) {
	t.Parallel()

	sessionIDs := []uint{1, 3, 5}
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo, &fakeOwnerNameLookup{})
	if _, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{
		Field:      constant.SessionFilterFieldMessageCount,
		SessionIDs: sessionIDs,
	}); err != nil {
		t.Fatalf("list message count options: %v", err)
	}
	if len(repo.lastSessionIDs) != len(sessionIDs) {
		t.Fatalf("session ids len = %d, want %d", len(repo.lastSessionIDs), len(sessionIDs))
	}
	for i := range sessionIDs {
		if repo.lastSessionIDs[i] != sessionIDs[i] {
			t.Fatalf("session ids[%d] = %d, want %d", i, repo.lastSessionIDs[i], sessionIDs[i])
		}
	}
	if repo.lastOwnerNames != nil {
		t.Errorf("demo whitelist scope must pass nil ownerNames, got %v", repo.lastOwnerNames)
	}
}

func TestListSessionOptionHandler_FieldScore(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo, &fakeOwnerNameLookup{})
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.FieldScore, IsAdmin: true})
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
	handler := query.NewListSessionOptionHandler(repo, &fakeOwnerNameLookup{})
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: "unknown", IsAdmin: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty items, got %d", len(items))
	}
}

func TestListSessionOptionHandler_FieldMessageCount(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo, &fakeOwnerNameLookup{})
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.SessionFilterFieldMessageCount, IsAdmin: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listMessageCountStatsCall != 1 {
		t.Errorf("expected ListMessageCountStats to be called once, got %d", repo.listMessageCountStatsCall)
	}
	want := []string{"0-10", "11-50", "51-82"}
	if len(items) != len(want) {
		t.Fatalf("expected %d items, got %d: %v", len(want), len(items), items)
	}
	for i, w := range want {
		if items[i] != w {
			t.Errorf("items[%d] mismatch: want %q, got %q", i, w, items[i])
		}
	}
}

// TestListSessionOptionHandler_UserScopeRestricted user 视角（非 admin 且非
// demo 白名单）的选项必须按名下 key owner 过滤（2026-08-26 越权修复回归——
// 此前 user 视角返回全平台维度选项）。
func TestListSessionOptionHandler_UserScopeRestricted(t *testing.T) {
	t.Parallel()

	repo := &fakeSessionReadRepo{}
	lookup := &fakeOwnerNameLookup{
		lookupFunc: func(ctx context.Context, userID uint) ([]string, error) {
			if userID != 42 {
				t.Errorf("lookup userID = %d, want 42", userID)
			}
			return []string{"key-a", "key-b"}, nil
		},
	}
	handler := query.NewListSessionOptionHandler(repo, lookup)

	if _, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{
		Field:   constant.SessionFilterFieldModel,
		UserID:  42,
		IsAdmin: false,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookup.calls != 1 {
		t.Errorf("lookup calls = %d, want 1", lookup.calls)
	}
	if len(repo.lastOwnerNames) != 2 || repo.lastOwnerNames[0] != "key-a" || repo.lastOwnerNames[1] != "key-b" {
		t.Errorf("lastOwnerNames = %v, want [key-a key-b]", repo.lastOwnerNames)
	}
}

// TestListSessionOptionHandler_UserWithoutKeysReturnsEmpty 名下无 key 的 user
// 视角必须返回空选项且不触达仓储（不得退化为全平台维度）。
func TestListSessionOptionHandler_UserWithoutKeysReturnsEmpty(t *testing.T) {
	t.Parallel()

	repo := &fakeSessionReadRepo{}
	lookup := &fakeOwnerNameLookup{
		lookupFunc: func(ctx context.Context, userID uint) ([]string, error) {
			return []string{}, nil
		},
	}
	handler := query.NewListSessionOptionHandler(repo, lookup)

	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{
		Field:   constant.SessionFilterFieldModel,
		UserID:  42,
		IsAdmin: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("items = %v, want empty", items)
	}
	if repo.listDistinctModelsCalled {
		t.Error("repo must not be reached when user has no keys")
	}
}
