// Package trace Trace 单元测试（fake repository）
package trace

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

// FakeRepo 内存版 trace 仓储，用于单元测试
type FakeRepo struct {
	mu     sync.Mutex
	traces map[string]*trace.Trace
	byID   map[uint]*trace.Trace
	events []*trace.TraceEvent
	nextID uint
}

// NewFakeRepo 构造内存版 trace 仓储
func NewFakeRepo() *FakeRepo {
	return &FakeRepo{
		traces: map[string]*trace.Trace{},
		byID:   map[uint]*trace.Trace{},
	}
}

func (f *FakeRepo) UpsertBySessionID(_ context.Context, t *trace.Trace) (*trace.Trace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t.ID == 0 {
		f.nextID++
		t.ID = f.nextID
	}
	if t.Status == "" {
		t.Status = "active"
	}
	f.traces[t.SessionID] = t
	f.byID[t.ID] = t
	return t, nil
}

func (f *FakeRepo) FindBySessionID(_ context.Context, sid string) (*trace.Trace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.traces[sid], nil
}

func (f *FakeRepo) FindByID(_ context.Context, id uint) (*trace.Trace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byID[id], nil
}

func (f *FakeRepo) MarkDone(_ context.Context, sid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.traces[sid]; ok {
		t.Status = "done"
	}
	return nil
}

func (f *FakeRepo) InsertEvent(_ context.Context, e *trace.TraceEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	e.ID = f.nextID
	if e.TraceID == 0 {
		if t, ok := f.traces[e.SessionID]; ok {
			e.TraceID = t.ID
		}
	}
	f.events = append(f.events, e)
	return nil
}

func (f *FakeRepo) PaginateByOwners(_ context.Context, owners []string, p model.CommonParam) ([]*trace.Trace, *model.PageInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ownerSet := map[string]struct{}{}
	for _, o := range owners {
		ownerSet[o] = struct{}{}
	}
	var out []*trace.Trace
	for _, t := range f.traces {
		if len(ownerSet) == 0 {
			out = append(out, t)
			continue
		}
		if _, ok := ownerSet[t.APIKeyName]; ok {
			out = append(out, t)
		}
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	pageSize := p.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	return out, &model.PageInfo{Page: page, PageSize: pageSize, Total: int64(len(out))}, nil
}

func (f *FakeRepo) CountEvents(_ context.Context, tid uint) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var c int64
	for _, e := range f.events {
		if e.TraceID == tid {
			c++
		}
	}
	return c, nil
}

func (f *FakeRepo) ListEvents(_ context.Context, tid uint, p model.CommonParam) ([]*trace.TraceEvent, *model.PageInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*trace.TraceEvent
	for _, e := range f.events {
		if e.TraceID == tid {
			out = append(out, e)
		}
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	pageSize := p.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	return out, &model.PageInfo{Page: page, PageSize: pageSize, Total: int64(len(out))}, nil
}

// fakeAPIKeyRepo 内存版 API Key 仓储，仅实现 owner 查询
type fakeAPIKeyRepo struct {
	owners map[uint][]string
}

func newFakeAPIKeyRepo(owners map[uint][]string) *fakeAPIKeyRepo {
	return &fakeAPIKeyRepo{owners: owners}
}

func (f *fakeAPIKeyRepo) Save(_ context.Context, _ *aggregate.ProxyAPIKey) error { return nil }
func (f *fakeAPIKeyRepo) FindByID(_ context.Context, _ uint) (*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (f *fakeAPIKeyRepo) ListByUser(_ context.Context, _ uint) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (f *fakeAPIKeyRepo) ListAll(_ context.Context) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (f *fakeAPIKeyRepo) PaginateByUser(_ context.Context, _ uint, _ model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeAPIKeyRepo) PaginateAll(_ context.Context, _ model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeAPIKeyRepo) CountByUser(_ context.Context, _ uint) (int64, error) { return 0, nil }
func (f *fakeAPIKeyRepo) Delete(_ context.Context, _ uint) error               { return nil }
func (f *fakeAPIKeyRepo) LookupOwnerNamesByUserID(_ context.Context, userID uint) ([]string, error) {
	return f.owners[userID], nil
}
func (f *fakeAPIKeyRepo) LookupIDsByUserID(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
