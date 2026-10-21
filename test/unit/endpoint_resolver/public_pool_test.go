package endpoint_resolver

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// visibilityModelRepo 区分「自有租户」与「公开池」的解析 fake，并记录公开池被查次数。
type visibilityModelRepo struct {
	own         map[string][]*aggregate.Model
	public      map[string][]*aggregate.Model
	publicCalls int
}

func (r *visibilityModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias, userID *uint) ([]*aggregate.Model, error) {
	if userID != nil && *userID == 0 {
		return nil, nil // 本组用例不涉及存量共享池
	}
	return r.own[alias.String()], nil
}

func (r *visibilityModelRepo) FindPublicByAlias(_ context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error) {
	r.publicCalls++
	return r.public[alias.String()], nil
}

func (r *visibilityModelRepo) FindByID(context.Context, uint, *uint) (*aggregate.Model, error) {
	return nil, nil
}

func (r *visibilityModelRepo) Create(context.Context, *aggregate.Model, uint) (uint, error) {
	return 0, nil
}

func (r *visibilityModelRepo) Update(context.Context, *aggregate.Model) error { return nil }

func (r *visibilityModelRepo) Delete(context.Context, uint, *uint) error { return nil }

func (r *visibilityModelRepo) DeleteByEndpointID(context.Context, uint) error { return nil }

func (r *visibilityModelRepo) List(context.Context) ([]*aggregate.Model, error) { return nil, nil }

func (r *visibilityModelRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *visibilityModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}

func (r *visibilityModelRepo) PaginateWithFilter(context.Context, model.CommonParam, llmproxy.ModelListFilter, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *visibilityModelRepo) UpdateWithHistorySync(context.Context, *aggregate.Model, string) (llmproxy.ModelIDSyncCounts, error) {
	return llmproxy.ModelIDSyncCounts{}, nil
}

// scopeRecordingEndpointRepo 记录每次按 ID 取端点时收到的 scope，并按 scope 过滤
// （与真实仓储一致：scope 不是归属者时查不到，防止实现退回 nil scope 仍假绿）。
type scopeRecordingEndpointRepo struct {
	byID   map[uint]*aggregate.Endpoint
	scopes []uint
}

func (r *scopeRecordingEndpointRepo) FindByID(_ context.Context, id uint, scope *uint) (*aggregate.Endpoint, error) {
	if scope != nil {
		r.scopes = append(r.scopes, *scope)
	}
	ep := r.byID[id]
	if ep == nil {
		return nil, nil
	}
	if scope != nil && ep.UserID() != *scope {
		return nil, nil
	}
	return ep, nil
}

func (r *scopeRecordingEndpointRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error) {
	out := map[uint]*aggregate.Endpoint{}
	for _, id := range ids {
		if ep := r.byID[id]; ep != nil {
			out[id] = ep
		}
	}
	return out, nil
}

func (r *scopeRecordingEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}

func (r *scopeRecordingEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { return nil }

func (r *scopeRecordingEndpointRepo) Delete(context.Context, uint, *uint) error { return nil }

func (r *scopeRecordingEndpointRepo) DeleteCascade(context.Context, uint, *uint) error { return nil }

func (r *scopeRecordingEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}

func (r *scopeRecordingEndpointRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *scopeRecordingEndpointRepo) FindIDsByScope(context.Context, *uint, bool) ([]uint, error) {
	return nil, nil
}

// newVisibilityEndpoint 构造一个最小合法端点（可见性默认 private，带归属）。
func newVisibilityEndpoint(t *testing.T, id, owner uint) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(id, "ep", "https://o.example.com", "https://a.example.com", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(owner)
	return ep
}

// newVisibilityModel 构造归属 owner 的模型记录（本组用例统一使用 gpt-x 别名）。
func newVisibilityModel(t *testing.T, id, owner, endpointID uint) *aggregate.Model {
	t.Helper()
	m, err := aggregate.CreateModel(id, vo.EndpointAlias("gpt-x"), "upstream-model", endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(owner)
	return m
}

func TestResolveCandidates_OwnHitSkipsPublic(t *testing.T) {
	t.Parallel()
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 101), 2: newVisibilityEndpoint(t, 2, 201)}}
	modelRepo := &visibilityModelRepo{
		own:    map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 1, 101, 1)}},
		public: map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 2, 201, 2)}},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil)

	cands, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Endpoint.AggregateID() != 1 {
		t.Fatalf("own hit must win, got %d candidates", len(cands))
	}
	if modelRepo.publicCalls != 0 {
		t.Fatalf("public pool must not be queried on own hit, calls=%d", modelRepo.publicCalls)
	}
}

func TestResolveCandidates_FallsBackToPublic(t *testing.T) {
	t.Parallel()
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 201)}}
	modelRepo := &visibilityModelRepo{
		own:    map[string][]*aggregate.Model{},
		public: map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 1, 201, 1)}},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil)

	cands, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Endpoint.AggregateID() != 1 {
		t.Fatalf("public fallback expected, got %d candidates", len(cands))
	}
	if !slices.Equal(epRepo.scopes, []uint{201}) {
		t.Fatalf("endpoint scope = %v, want [201]", epRepo.scopes)
	}
}

func TestResolveCandidates_PublicWorksWithoutSharedPoolFallback(t *testing.T) {
	t.Parallel()
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 201)}}
	modelRepo := &visibilityModelRepo{
		own:    map[string][]*aggregate.Model{},
		public: map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 1, 201, 1)}},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil) // 共享池开关关闭

	if _, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil); err != nil {
		t.Fatalf("public pool must not depend on shared_pool_fallback: %v", err)
	}
}

func TestResolveCandidates_MultipleAdminsUseOwnScope(t *testing.T) {
	t.Parallel()
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 201), 2: newVisibilityEndpoint(t, 2, 202)}}
	modelRepo := &visibilityModelRepo{
		own: map[string][]*aggregate.Model{},
		public: map[string][]*aggregate.Model{"gpt-x": {
			newVisibilityModel(t, 1, 201, 1),
			newVisibilityModel(t, 2, 202, 2),
		}},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil)

	if _, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil); err != nil {
		t.Fatal(err)
	}
	scopes := slices.Clone(epRepo.scopes)
	slices.Sort(scopes)
	if !slices.Equal(scopes, []uint{201, 202}) {
		t.Fatalf("each public model must resolve with its own owner scope, got %v", epRepo.scopes)
	}
}

func TestResolveCandidates_DisabledOwnModelFallsBackToPublic(t *testing.T) {
	t.Parallel()
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 201)}}
	modelRepo := &visibilityModelRepo{
		// 自有记录被禁用时 FindByAlias 返回空（仓储侧已带 enabled 过滤）。
		own:    map[string][]*aggregate.Model{"gpt-x": {}},
		public: map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 1, 201, 1)}},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil)

	if _, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil); err != nil {
		t.Fatalf("disabled own model must fall back to public: %v", err)
	}
}

// TestResolveCandidates_DirtyOwnerFailsClosed 脏数据破坏 model.user_id == endpoint.user_id 时，
// 逐模型 scope 取端点会查不到 → 候选被跳过（fail-closed），不会借用他人端点。
func TestResolveCandidates_DirtyOwnerFailsClosed(t *testing.T) {
	t.Parallel()
	// 模型归属 101，但端点归属 999。
	epRepo := &scopeRecordingEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: newVisibilityEndpoint(t, 1, 999)}}
	modelRepo := &visibilityModelRepo{
		own:    map[string][]*aggregate.Model{"gpt-x": {newVisibilityModel(t, 1, 101, 1)}},
		public: map[string][]*aggregate.Model{},
	}
	resolver := service.NewEndpointResolver(epRepo, modelRepo, false, nil)

	if _, err := resolver.ResolveCandidates(t.Context(), 101, vo.EndpointAlias("gpt-x"), nil); !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("dirty owner must fail closed with ErrDataNotExists, got %v", err)
	}
}
