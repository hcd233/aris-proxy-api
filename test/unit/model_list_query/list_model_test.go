// Package model_list_query 验证平铺模型列表查询的嵌套回填、username 解析与 demo 脱敏。
package model_list_query

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	uservo "github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// testTime 固定时间戳，避免断言受当前时钟影响
func testTime() time.Time { return time.Unix(1756000000, 0) }

// mustUser 构造用户聚合。
//
// 注意：identity 聚合只暴露 RestoreUser（9 参数），没有 CreateUser——
// 参数顺序为 id, name, email, avatar, permission, lastLogin, createdAt,
// githubBindID, googleBindID。
func mustUser(id uint, name string) *useragg.User {
	return useragg.RestoreUser(
		id,
		uservo.UserName(name),
		uservo.Email(name+"@example.com"),
		uservo.Avatar("https://cdn.example.com/"+name+".png"),
		enum.PermissionUser,
		testTime(), testTime(), "", "",
	)
}

// ── fakes：嵌入接口后仅覆写用到的方法 ──

type fakeModelRepo struct {
	llmproxy.ModelRepository
	models    []*llmagg.Model
	gotFilter llmproxy.ModelListFilter
	gotScope  *uint
}

func (f *fakeModelRepo) PaginateWithFilter(_ context.Context, param model.CommonParam, filter llmproxy.ModelListFilter, scope *uint) ([]*llmagg.Model, *model.PageInfo, error) {
	f.gotFilter = filter
	f.gotScope = scope
	return f.models, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: int64(len(f.models))}, nil
}

type fakeEndpointRepo struct {
	llmproxy.EndpointRepository
	endpoints map[uint]*llmagg.Endpoint
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

type fakeUserRepo struct {
	identity.UserRepository
	users  map[uint]*useragg.User
	byName map[string]*useragg.User
	gotIDs []uint
}

func (f *fakeUserRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*useragg.User, error) {
	f.gotIDs = ids
	out := make(map[uint]*useragg.User, len(ids))
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

func (f *fakeUserRepo) FindByName(_ context.Context, name string) (*useragg.User, error) {
	return f.byName[name], nil
}

// ── helpers ──

var textCaps = []enum.InputModality{enum.InputModalityText}

func mustModel(t *testing.T, id, epID, userID uint, alias, upstream string) *llmagg.Model {
	t.Helper()
	m, err := llmagg.CreateModel(id, llmvo.EndpointAlias(alias), upstream, epID, true, 1000, 100, textCaps)
	if err != nil {
		t.Fatalf("create model aggregate: %v", err)
	}
	m.SetUserID(userID)
	return m
}

func mustEndpoint(t *testing.T, id uint, name string) *llmagg.Endpoint {
	t.Helper()
	ep, err := llmagg.CreateEndpoint(id, name, "https://o.example.com", "https://a.example.com", "sk-secret", true, false, false)
	if err != nil {
		t.Fatalf("create endpoint aggregate: %v", err)
	}
	return ep
}

func baseParam() model.CommonParam {
	return model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}}
}

func TestListModel_NestedBackfill(t *testing.T) {
	t.Parallel()
	mRepo := &fakeModelRepo{models: []*llmagg.Model{
		mustModel(t, 11, 1, 101, "alias-a", "up-a"),
		mustModel(t, 12, 999, 0, "alias-orphan", "up-b"), // 端点 999 不存在 + userID=0
	}}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{1: mustEndpoint(t, 1, "ep-one")}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{101: mustUser(101, "centonhuang")}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, page, err := h.Handle(t.Context(), port.ListModelQuery{
		CommonParam: baseParam(),
		ScopeUserID: lo.ToPtr(uint(101)),
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("total: want 2, got %d", page.Total)
	}
	if views[0].Endpoint == nil || views[0].Endpoint.Name != "ep-one" {
		t.Errorf("row 0 endpoint should be backfilled, got %+v", views[0].Endpoint)
	}
	if views[0].User == nil || views[0].User.Name != "centonhuang" {
		t.Errorf("row 0 user should be backfilled, got %+v", views[0].User)
	}
	// 端点缺失 → nil（不得造出空壳对象）
	if views[1].Endpoint != nil {
		t.Errorf("row 1 endpoint should be nil, got %+v", views[1].Endpoint)
	}
	if views[1].User != nil {
		t.Errorf("row 1 user should be nil for userID=0, got %+v", views[1].User)
	}
	// userID=0 不得进入用户批查
	if lo.Contains(uRepo.gotIDs, uint(0)) {
		t.Errorf("userID 0 must be filtered out, got %v", uRepo.gotIDs)
	}
}

func TestListModel_DemoMasksUpstreamModel(t *testing.T) {
	t.Parallel()
	mRepo := &fakeModelRepo{models: []*llmagg.Model{mustModel(t, 11, 1, 101, "alias-a", "secret-upstream-name")}}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{1: mustEndpoint(t, 1, "ep-one")}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, _, err := h.Handle(t.Context(), port.ListModelQuery{
		CommonParam: baseParam(),
		IsDemo:      true,
		ScopeUserID: lo.ToPtr(uint(101)),
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if views[0].UpstreamModel == "secret-upstream-name" {
		t.Error("demo must mask upstreamModel")
	}
	if views[0].UpstreamModel == "" {
		t.Error("demo masked upstreamModel should not be empty")
	}
}

func TestListModel_UsernameResolvesToScope(t *testing.T) {
	t.Parallel()
	target := mustUser(202, "someone")
	// 预置数据：一旦 scope 退化成 nil（admin 全量视角），这些行就会被返回，
	// 从而让"用户不存在时应短路返回空"的断言真正有区分力。
	mRepo := &fakeModelRepo{models: []*llmagg.Model{
		mustModel(t, 21, 1, 202, "someone-model", "up-s"),
		mustModel(t, 22, 2, 303, "other-model", "up-o"),
	}}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}, byName: map[string]*useragg.User{"someone": target}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, _, err := h.Handle(t.Context(), port.ListModelQuery{
		CommonParam: baseParam(),
		ScopeUserID: nil, // admin
		Username:    "someone",
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if mRepo.gotScope == nil || *mRepo.gotScope != 202 {
		t.Errorf("username should resolve to scope 202, got %v", mRepo.gotScope)
	}
	// 仓储层是 fake 不做真实过滤，故此处只断言 scope 解析正确
	if len(views) == 0 {
		t.Error("expected fake rows to be projected")
	}

	// 用户不存在 → 空结果，且不得让 scope 保持 nil 退化为全量可见。
	// 若退化，这里会拿到上面预置的 2 行而非空。
	h2 := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, page, err := h2.Handle(t.Context(), port.ListModelQuery{
		CommonParam: baseParam(),
		Username:    "ghost",
	})
	if err != nil {
		t.Fatalf("handle ghost: %v", err)
	}
	if len(views) != 0 || page.Total != 0 {
		t.Errorf("unknown username must yield empty result, got %d rows total=%d", len(views), page.Total)
	}
}

func TestListModel_FilterPassthrough(t *testing.T) {
	t.Parallel()
	mRepo := &fakeModelRepo{}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	if _, _, err := h.Handle(t.Context(), port.ListModelQuery{
		CommonParam: baseParam(),
		ScopeUserID: lo.ToPtr(uint(101)),
		Status:      "disabled",
		EndpointID:  7,
		Capability:  enum.InputModalityImage,
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	want := llmproxy.ModelListFilter{Status: "disabled", EndpointID: 7, Capability: enum.InputModalityImage}
	if mRepo.gotFilter != want {
		t.Errorf("filter passthrough: want %+v, got %+v", want, mRepo.gotFilter)
	}
}
