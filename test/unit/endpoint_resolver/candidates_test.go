package endpoint_resolver

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// cand 单个候选的构造参数：alias 关联到 name 端点，带调度参数。
type cand struct {
	alias    string
	endpoint string
	priority int
	weight   int
}

type multiModelRepo struct {
	byAlias map[string][]*aggregate.Model
}

func (r *multiModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias, _ *uint) ([]*aggregate.Model, error) {
	return r.byAlias[alias.String()], nil
}
func (r *multiModelRepo) FindByID(context.Context, uint, *uint) (*aggregate.Model, error) {
	return nil, nil
}
func (r *multiModelRepo) Create(context.Context, *aggregate.Model, uint) (uint, error) {
	return 0, nil
}
func (r *multiModelRepo) Update(context.Context, *aggregate.Model) error { return nil }
func (r *multiModelRepo) Delete(context.Context, uint, *uint) error      { return nil }
func (r *multiModelRepo) DeleteByEndpointID(context.Context, uint) error { return nil }
func (r *multiModelRepo) List(context.Context) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *multiModelRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *multiModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *multiModelRepo) PaginateWithFilter(context.Context, model.CommonParam, llmproxy.ModelListFilter, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *multiModelRepo) UpdateWithHistorySync(context.Context, *aggregate.Model, string) (llmproxy.ModelIDSyncCounts, error) {
	return llmproxy.ModelIDSyncCounts{}, nil
}

type multiEndpointRepo struct {
	byID map[uint]*aggregate.Endpoint
}

func (r *multiEndpointRepo) FindByID(_ context.Context, id uint, _ *uint) (*aggregate.Endpoint, error) {
	return r.byID[id], nil
}
func (r *multiEndpointRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error) {
	out := map[uint]*aggregate.Endpoint{}
	for _, id := range ids {
		if ep := r.byID[id]; ep != nil {
			out[id] = ep
		}
	}
	return out, nil
}
func (r *multiEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (r *multiEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { return nil }
func (r *multiEndpointRepo) Delete(context.Context, uint, *uint) error         { return nil }
func (r *multiEndpointRepo) DeleteCascade(context.Context, uint, *uint) error  { return nil }
func (r *multiEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}
func (r *multiEndpointRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *multiEndpointRepo) FindIDsByScope(context.Context, *uint) ([]uint, error) {
	return nil, nil
}

// newTestResolver 按候选声明构造 resolver：每个 cand 建一个 endpoint（ID 递增）+ 一条 model 记录。
func newTestResolver(t *testing.T, cands ...cand) service.EndpointResolver {
	t.Helper()
	epRepo := &multiEndpointRepo{byID: map[uint]*aggregate.Endpoint{}}
	modelRepo := &multiModelRepo{byAlias: map[string][]*aggregate.Model{}}
	for i, c := range cands {
		id := uint(i + 1)
		ep, err := aggregate.CreateEndpoint(id, c.endpoint, "https://o.example.com", "https://a.example.com", "k", true, true, true)
		if err != nil {
			t.Fatalf("CreateEndpoint: %v", err)
		}
		epRepo.byID[id] = ep
		m, err := aggregate.CreateModel(id, vo.EndpointAlias(c.alias), "upstream-model", id, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
		if err != nil {
			t.Fatalf("CreateModel: %v", err)
		}
		if err := m.SetScheduling(c.priority, c.weight); err != nil {
			t.Fatalf("SetScheduling: %v", err)
		}
		modelRepo.byAlias[c.alias] = append(modelRepo.byAlias[c.alias], m)
	}
	return service.NewEndpointResolver(epRepo, modelRepo, false)
}

func TestResolveCandidatesPriorityOrder(t *testing.T) {
	t.Parallel()
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-low", priority: 5, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-high1", priority: 1, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-high2", priority: 1, weight: 1},
	)
	got, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil)
	if err != nil {
		t.Fatalf("ResolveCandidates: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[2].Endpoint.Name() != "ep-low" {
		t.Fatalf("last = %s, want ep-low", got[2].Endpoint.Name())
	}
	if got[0].Model.Priority() != 1 || got[1].Model.Priority() != 1 {
		t.Fatalf("前两位应为 priority=1 档, got %d,%d", got[0].Model.Priority(), got[1].Model.Priority())
	}
}

func TestResolveCandidatesWeightedShuffle(t *testing.T) {
	t.Parallel()
	// weight 9:1，1000 次抽样中重端点应显著更多（A-Res 下期望约 90%）
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-heavy", priority: 0, weight: 9},
		cand{alias: "gpt-4", endpoint: "ep-light", priority: 0, weight: 1},
	)
	heavyFirst := 0
	for range 1000 {
		got, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil)
		if err != nil {
			t.Fatalf("ResolveCandidates: %v", err)
		}
		if got[0].Endpoint.Name() == "ep-heavy" {
			heavyFirst++
		}
	}
	if heavyFirst < 750 {
		t.Fatalf("heavy first = %d/1000, want >= 750", heavyFirst)
	}
}

func TestResolveCandidatesEmpty(t *testing.T) {
	t.Parallel()
	r := newTestResolver(t)
	if _, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil); err == nil {
		t.Fatal("无候选应返回错误")
	}
}

func TestResolveCandidatesMatcherFilters(t *testing.T) {
	t.Parallel()
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-a", priority: 0, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-b", priority: 0, weight: 1},
	)
	got, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", func(ep *aggregate.Endpoint) bool {
		return ep.Name() == "ep-b"
	})
	if err != nil {
		t.Fatalf("ResolveCandidates: %v", err)
	}
	if len(got) != 1 || got[0].Endpoint.Name() != "ep-b" {
		t.Fatalf("got = %v, want [ep-b]", got)
	}
}
