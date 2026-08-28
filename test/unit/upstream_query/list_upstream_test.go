// Package upstream_query 验证 upstream 分组列表查询的分组分页、keyword 整组聚合与用户回填。
package upstream_query

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/application/upstream/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	uservo "github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// ==================== fakes ====================

type fakeEndpointRepo struct {
	endpoints map[uint]*llmagg.Endpoint // 以 ID 索引；ID 列表即 FindIDsByScope 结果
}

func (f *fakeEndpointRepo) FindIDsByScope(_ context.Context, scopeUserID uint) ([]uint, error) {
	ids := make([]uint, 0, len(f.endpoints))
	for id, ep := range f.endpoints {
		if scopeUserID == 0 || ep.UserID() == scopeUserID {
			ids = append(ids, id)
		}
	}
	// 与 SQL 实现 ORDER BY id 对齐，保证分页确定性
	slices.Sort(ids)
	return ids, nil
}

func (f *fakeEndpointRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*llmagg.Endpoint, error) {
	out := make(map[uint]*llmagg.Endpoint, len(ids))
	for _, id := range ids {
		if ep, ok := f.endpoints[id]; ok {
			out[id] = ep
		}
	}
	return out, nil
}

func (f *fakeEndpointRepo) FindByID(context.Context, uint, uint) (*llmagg.Endpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) Create(context.Context, *llmagg.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (f *fakeEndpointRepo) Update(context.Context, *llmagg.Endpoint) error   { return nil }
func (f *fakeEndpointRepo) Delete(context.Context, uint, uint) error         { return nil }
func (f *fakeEndpointRepo) DeleteCascade(context.Context, uint, uint) error  { return nil }
func (f *fakeEndpointRepo) List(context.Context) ([]*llmagg.Endpoint, error) { return nil, nil }
func (f *fakeEndpointRepo) Paginate(context.Context, model.CommonParam, uint) ([]*llmagg.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)

type fakeModelRepo struct {
	models []*llmagg.Model
}

func (f *fakeModelRepo) PaginateWithFilter(context.Context, model.CommonParam, llmproxy.ModelListFilter, *uint) ([]*llmagg.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *fakeModelRepo) ListByEndpointIDs(_ context.Context, endpointIDs []uint) ([]*llmagg.Model, error) {
	set := make(map[uint]struct{}, len(endpointIDs))
	for _, id := range endpointIDs {
		set[id] = struct{}{}
	}
	out := make([]*llmagg.Model, 0)
	for _, m := range f.models {
		if _, ok := set[m.EndpointID()]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeModelRepo) FindByAlias(context.Context, llmvo.EndpointAlias, uint) ([]*llmagg.Model, error) {
	return nil, nil
}
func (f *fakeModelRepo) FindByID(context.Context, uint, uint) (*llmagg.Model, error) { return nil, nil }
func (f *fakeModelRepo) Create(context.Context, *llmagg.Model, uint) (uint, error)   { return 0, nil }
func (f *fakeModelRepo) Update(context.Context, *llmagg.Model) error                 { return nil }
func (f *fakeModelRepo) Delete(context.Context, uint, uint) error                    { return nil }
func (f *fakeModelRepo) DeleteByEndpointID(context.Context, uint) error              { return nil }
func (f *fakeModelRepo) List(context.Context) ([]*llmagg.Model, error)               { return nil, nil }
func (f *fakeModelRepo) Paginate(context.Context, model.CommonParam, uint) ([]*llmagg.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

var _ llmproxy.ModelRepository = (*fakeModelRepo)(nil)

type fakeUserRepo struct {
	users map[uint]*useragg.User
}

func (f *fakeUserRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*useragg.User, error) {
	out := make(map[uint]*useragg.User, len(ids))
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

func (f *fakeUserRepo) FindByName(_ context.Context, name string) (*useragg.User, error) {
	for _, u := range f.users {
		if string(u.Name()) == name {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeUserRepo) FindByID(context.Context, uint) (*useragg.User, error) { return nil, nil }
func (f *fakeUserRepo) Create(context.Context, *useragg.User) (uint, error)   { return 0, nil }
func (f *fakeUserRepo) Update(context.Context, *useragg.User) error           { return nil }
func (f *fakeUserRepo) Save(context.Context, *useragg.User) error             { return nil }
func (f *fakeUserRepo) FindByGithubBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByGoogleBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByPermission(context.Context, enum.Permission) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) ReplaceDemoUser(context.Context, uint) (uint, error) { return 0, nil }
func (f *fakeUserRepo) TouchLastLogin(context.Context, uint) error          { return nil }
func (f *fakeUserRepo) ListUsers(context.Context, model.CommonParam, enum.Permission) ([]*useragg.User, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeUserRepo) DeleteCascade(context.Context, uint) error { return nil }

var _ identity.UserRepository = (*fakeUserRepo)(nil)

// ==================== fixtures ====================

const (
	uAlice uint = 11
	uBob   uint = 12

	epA uint = 101
	epB uint = 102
	epC uint = 103
)

func mustEndpoint(t *testing.T, id, userID uint, name string) *llmagg.Endpoint {
	t.Helper()
	ep, err := llmagg.CreateEndpoint(id, name, "https://o.example.com", "https://a.example.com", "sk-secret", true, false, false)
	if err != nil {
		t.Fatalf("CreateEndpoint(%d): %v", id, err)
	}
	ep.SetUserID(userID)
	ep.SetTimestamps(testTime(), testTime())
	return ep
}

func mustModel(t *testing.T, id, userID, endpointID uint, alias string) *llmagg.Model {
	t.Helper()
	m, err := llmagg.CreateModel(id, llmvo.EndpointAlias(alias), "upstream-"+alias, endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel(%d): %v", id, err)
	}
	m.SetUserID(userID)
	m.SetTimestamps(testTime(), testTime())
	return m
}

func testTime() time.Time { return time.Unix(1756000000, 0) }

func newHandler(t *testing.T, eps map[uint]*llmagg.Endpoint, models []*llmagg.Model, users map[uint]*useragg.User) port.ListUpstreamHandler {
	t.Helper()
	return query.NewListUpstreamHandler(&fakeEndpointRepo{endpoints: eps}, &fakeModelRepo{models: models}, &fakeUserRepo{users: users})
}

func alice(t *testing.T) *useragg.User {
	t.Helper()
	return useragg.RestoreUser(uAlice, uservo.UserName("alice"), uservo.Email("alice@example.com"), uservo.Avatar("https://cdn.example.com/a.png"), enum.PermissionUser, testTime(), testTime(), "", "")
}

// 三端点四模型：epA(alice) x2 模型、epB(alice) x1、epC(bob) x1。
func standardFixture(t *testing.T) port.ListUpstreamHandler {
	t.Helper()
	al := alice(t)
	bob := useragg.RestoreUser(uBob, uservo.UserName("bob"), uservo.Email("bob@example.com"), "", enum.PermissionUser, testTime(), testTime(), "", "")
	eps := map[uint]*llmagg.Endpoint{
		epA: mustEndpoint(t, epA, uAlice, "ep-a"),
		epB: mustEndpoint(t, epB, uAlice, "ep-b"),
		epC: mustEndpoint(t, epC, uBob, "ep-c"),
	}
	models := []*llmagg.Model{
		mustModel(t, 201, uAlice, epA, "sonnet"),
		mustModel(t, 202, uAlice, epA, "gpt5"),
		mustModel(t, 203, uAlice, epB, "haiku"),
		mustModel(t, 204, uBob, epC, "mini"),
	}
	_ = al
	return newHandler(t, eps, models, map[uint]*useragg.User{uAlice: al, uBob: bob})
}

// ==================== tests ====================

// TestListUpstream_GroupPaginationAndBucketing pageSize=2 时第一页返回前两组，total=组数，modelTotal=全量模型数。
func TestListUpstream_GroupPaginationAndBucketing(t *testing.T) {
	t.Parallel()

	h := standardFixture(t)

	groups, modelTotal, pageInfo, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 2}},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups on page 1, got %d", len(groups))
	}
	if pageInfo == nil || pageInfo.Total != 3 {
		t.Fatalf("expected total=3 endpoints, got %+v", pageInfo)
	}
	if modelTotal != 4 {
		t.Fatalf("expected modelTotal=4, got %d", modelTotal)
	}
	first := groups[0]
	if first.Endpoint.ID != epA || first.ModelCount != 2 || len(first.Models) != 2 {
		t.Fatalf("unexpected first group: id=%d count=%d len=%d", first.Endpoint.ID, first.ModelCount, len(first.Models))
	}
	if first.Truncated {
		t.Fatalf("first group unexpectedly truncated")
	}
}

// TestListUpstream_KeywordAggregatesWholeGroup keyword 命中组内某模型时整组返回，total 只计命中组。
func TestListUpstream_KeywordAggregatesWholeGroup(t *testing.T) {
	t.Parallel()

	h := standardFixture(t)

	groups, modelTotal, pageInfo, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{
			PageParam:  model.PageParam{Page: 1, PageSize: 10},
			QueryParam: model.QueryParam{Query: "sonnet"},
		},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(groups) != 1 || groups[0].Endpoint.ID != epA {
		t.Fatalf("expected only ep-a group, got %d groups", len(groups))
	}
	if len(groups[0].Models) != 2 {
		t.Fatalf("expected whole group (2 models), got %d", len(groups[0].Models))
	}
	if modelTotal != 2 || pageInfo.Total != 1 {
		t.Fatalf("expected modelTotal=2 total=1, got modelTotal=%d total=%d", modelTotal, pageInfo.Total)
	}
}

// TestListUpstream_TruncationAt200 组内模型超过 200 截断并标记 Truncated。
func TestListUpstream_TruncationAt200(t *testing.T) {
	t.Parallel()

	models := make([]*llmagg.Model, 0, 205)
	for i := range 205 {
		models = append(models, mustModel(t, uint(300+i), uAlice, epA, "m"+strconv.Itoa(i)))
	}
	h := newHandler(t, map[uint]*llmagg.Endpoint{epA: mustEndpoint(t, epA, uAlice, "ep-a")}, models, nil)

	groups, modelTotal, _, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(groups) != 1 || !groups[0].Truncated {
		t.Fatalf("expected truncated group, got %+v", groups)
	}
	if groups[0].ModelCount != 200 || len(groups[0].Models) != 200 {
		t.Fatalf("expected 200 models after truncation, got count=%d len=%d", groups[0].ModelCount, len(groups[0].Models))
	}
	if modelTotal != 205 {
		t.Fatalf("modelTotal should not be affected by truncation, got %d", modelTotal)
	}
}

// TestListUpstream_NestedUserFilledAndMissing 归属用户存在回填嵌套 user；不存在整体为 nil。
func TestListUpstream_NestedUserFilledAndMissing(t *testing.T) {
	t.Parallel()

	al := alice(t)
	// epA 归属 alice（users 里存在）；epB 归属一个不在 users 集合的用户（模拟软删）。
	eps := map[uint]*llmagg.Endpoint{
		epA: mustEndpoint(t, epA, uAlice, "ep-a"),
		epB: mustEndpoint(t, epB, 99, "ep-b-ghost"),
	}
	models := []*llmagg.Model{mustModel(t, 201, uAlice, epA, "sonnet")}
	h := newHandler(t, eps, models, map[uint]*useragg.User{uAlice: al})

	groups, _, _, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	byEp := make(map[uint]*port.UpstreamGroupView, len(groups))
	for _, g := range groups {
		byEp[g.Endpoint.ID] = g
	}
	if byEp[epA].Endpoint.User == nil || byEp[epA].Endpoint.User.Name != "alice" ||
		byEp[epA].Endpoint.User.Avatar != "https://cdn.example.com/a.png" {
		t.Fatalf("ep-a user not filled: %+v", byEp[epA].Endpoint.User)
	}
	if byEp[epB].Endpoint.User != nil {
		t.Fatalf("ep-b ghost owner should be nil, got %+v", byEp[epB].Endpoint.User)
	}
}

// TestListUpstream_ScopeIsolation ScopeUserID 限定结果范围。
func TestListUpstream_ScopeIsolation(t *testing.T) {
	t.Parallel()

	h := standardFixture(t)

	groups, modelTotal, pageInfo, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
		ScopeUserID: uBob,
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(groups) != 1 || groups[0].Endpoint.ID != epC {
		t.Fatalf("expected only ep-c for bob, got %d groups", len(groups))
	}
	if modelTotal != 1 || pageInfo.Total != 1 {
		t.Fatalf("expected modelTotal=1 total=1, got modelTotal=%d total=%d", modelTotal, pageInfo.Total)
	}
}

// TestListUpstream_UsernameResolveToEmpty admin 视角传不存在的 username 返回空结果而非错误。
func TestListUpstream_UsernameResolveToEmpty(t *testing.T) {
	t.Parallel()

	h := standardFixture(t)

	groups, modelTotal, pageInfo, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
		Username:    "nobody",
	})
	if err != nil {
		t.Fatalf("Handle should not fail for unknown username: %v", err)
	}
	if len(groups) != 0 || modelTotal != 0 {
		t.Fatalf("expected empty result, got %d groups modelTotal=%d", len(groups), modelTotal)
	}
	if pageInfo == nil {
		t.Fatalf("pageInfo should not be nil even for empty result")
	}
}
