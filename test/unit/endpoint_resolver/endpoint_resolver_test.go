package endpoint_resolver

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type resolverCase struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	ModelBehavior    string `json:"modelBehavior"`
	EndpointBehavior string `json:"endpointBehavior"`
	Alias            string `json:"alias"`
	ExpectResolved   bool   `json:"expectResolved"`
	ExpectErrKind    string `json:"expectErrKind"`
}

func loadCases(t *testing.T) []resolverCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []resolverCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

type stubModelRepo struct {
	behaviorByAlias map[string]string
	callsByAlias    map[string]int
}

func newStubModelRepo(behavior string) *stubModelRepo {
	return &stubModelRepo{
		behaviorByAlias: map[string]string{"default": behavior},
		callsByAlias:    map[string]int{},
	}
}

var errStubDBFailure = ierr.New(ierr.ErrDBQuery, "simulated db outage")

func (s *stubModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias, _ uint) ([]*aggregate.Model, error) {
	s.callsByAlias[alias.String()]++
	b, ok := s.behaviorByAlias[alias.String()]
	if !ok {
		b = s.behaviorByAlias["default"]
	}
	switch b {
	case "hit":
		m, _ := aggregate.CreateModel(1, alias, "test-model", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
		return []*aggregate.Model{m}, nil
	case "miss":
		return nil, nil
	case "db_error":
		return nil, errStubDBFailure
	default:
		return nil, ierr.New(ierr.ErrInternal, "unknown stub behavior")
	}
}

func (s *stubModelRepo) FindByID(_ context.Context, _, _ uint) (*aggregate.Model, error) {
	return nil, nil
}

func (s *stubModelRepo) Create(_ context.Context, _ *aggregate.Model, _ uint) (uint, error) {
	return 0, nil
}

func (s *stubModelRepo) Update(_ context.Context, _ *aggregate.Model) error {
	return nil
}

func (s *stubModelRepo) Delete(_ context.Context, _, _ uint) error {
	return nil
}

func (s *stubModelRepo) DeleteByEndpointID(_ context.Context, _ uint) error {
	return nil
}

func (s *stubModelRepo) List(_ context.Context) ([]*aggregate.Model, error) {
	return nil, nil
}

func (s *stubModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}

func (s *stubModelRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

type stubEndpointRepo struct {
	findByIDCalled bool
}

func (s *stubEndpointRepo) FindByID(_ context.Context, id, _ uint) (*aggregate.Endpoint, error) {
	s.findByIDCalled = true
	if id == 0 {
		return nil, nil
	}
	return aggregate.CreateEndpoint(id, "test-endpoint", "https://api.openai.com", "https://api.anthropic.com", "sk-test", true, false, true)
}

func (s *stubEndpointRepo) BatchFindByIDs(_ context.Context, _ []uint) (map[uint]*aggregate.Endpoint, error) {
	return map[uint]*aggregate.Endpoint{}, nil
}

func (s *stubEndpointRepo) Create(_ context.Context, _ *aggregate.Endpoint, _ uint) (uint, error) {
	return 0, nil
}

func (s *stubEndpointRepo) Update(_ context.Context, _ *aggregate.Endpoint) error {
	return nil
}

func (s *stubEndpointRepo) Delete(_ context.Context, _, _ uint) error {
	return nil
}

func (s *stubEndpointRepo) DeleteCascade(_ context.Context, _, _ uint) error {
	return nil
}

func (s *stubEndpointRepo) List(_ context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}

func (s *stubEndpointRepo) FindIDsByScope(context.Context, uint) ([]uint, error) {
	return nil, nil
}

func (s *stubEndpointRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

type staticModelRepo struct {
	models []*aggregate.Model
}

func (s *staticModelRepo) FindByAlias(_ context.Context, _ vo.EndpointAlias, userID uint) ([]*aggregate.Model, error) {
	return s.models, nil
}

func (s *staticModelRepo) FindByID(_ context.Context, _, _ uint) (*aggregate.Model, error) {
	return nil, nil
}

func (s *staticModelRepo) Create(_ context.Context, _ *aggregate.Model, _ uint) (uint, error) {
	return 0, nil
}

func (s *staticModelRepo) Update(_ context.Context, _ *aggregate.Model) error {
	return nil
}

func (s *staticModelRepo) Delete(_ context.Context, _, _ uint) error {
	return nil
}

func (s *staticModelRepo) DeleteByEndpointID(_ context.Context, _ uint) error {
	return nil
}

func (s *staticModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}

func (s *staticModelRepo) List(_ context.Context) ([]*aggregate.Model, error) {
	return nil, nil
}

func (s *staticModelRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

type endpointByIDRepo struct {
	endpoints map[uint]*aggregate.Endpoint
}

func (s *endpointByIDRepo) FindIDsByScope(context.Context, uint) ([]uint, error) {
	return nil, nil
}

func (s *endpointByIDRepo) FindByID(_ context.Context, id, _ uint) (*aggregate.Endpoint, error) {
	return s.endpoints[id], nil
}

func (s *endpointByIDRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error) {
	out := make(map[uint]*aggregate.Endpoint, len(ids))
	for _, id := range ids {
		if ep, ok := s.endpoints[id]; ok {
			out[id] = ep
		}
	}
	return out, nil
}

func (s *endpointByIDRepo) Create(_ context.Context, _ *aggregate.Endpoint, _ uint) (uint, error) {
	return 0, nil
}

func (s *endpointByIDRepo) Update(_ context.Context, _ *aggregate.Endpoint) error {
	return nil
}

func (s *endpointByIDRepo) Delete(_ context.Context, _, _ uint) error {
	return nil
}

func (s *endpointByIDRepo) DeleteCascade(_ context.Context, _, _ uint) error {
	return nil
}

func (s *endpointByIDRepo) List(_ context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}

func (s *endpointByIDRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func TestEndpointResolver_ResolveSkipsDisabledModels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	alias := vo.EndpointAlias("test-model")
	ep, _ := aggregate.CreateEndpoint(1, "test-endpoint", "https://api.openai.com", "", "sk-test", true, false, false)
	disabledModel, _ := aggregate.CreateModel(1, alias, "disabled-upstream", 1, false, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	enabledModel, _ := aggregate.CreateModel(2, alias, "enabled-upstream", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	resolver := service.NewEndpointResolver(
		&endpointByIDRepo{endpoints: map[uint]*aggregate.Endpoint{1: ep}},
		&staticModelRepo{models: []*aggregate.Model{disabledModel, enabledModel}},
	)

	_, m, err := resolver.Resolve(ctx, 0, alias, func(ep *aggregate.Endpoint) bool {
		return ep.SupportOpenAIChatCompletion()
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if m.UpstreamModel() != "enabled-upstream" {
		t.Fatalf("upstream model = %q, want %q", m.UpstreamModel(), "enabled-upstream")
	}
	if m.Enabled() {
		t.Log("resolved to enabled model as expected")
	}
}

func TestEndpointResolver_ResolveFiltersUnsupportedEndpoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	alias := vo.EndpointAlias("test-model")
	anthropicOnly, _ := aggregate.CreateEndpoint(1, "anthropic-only", "", "https://api.anthropic.com", "sk-ant", false, false, true)
	openAIOnly, _ := aggregate.CreateEndpoint(2, "openai-only", "https://api.openai.com", "", "sk-openai", true, false, false)
	anthropicModel, _ := aggregate.CreateModel(1, alias, "claude-upstream", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	openAIModel, _ := aggregate.CreateModel(2, alias, "gpt-upstream", 2, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	resolver := service.NewEndpointResolver(
		&endpointByIDRepo{endpoints: map[uint]*aggregate.Endpoint{1: anthropicOnly, 2: openAIOnly}},
		&staticModelRepo{models: []*aggregate.Model{anthropicModel, openAIModel}},
	)

	ep, m, err := resolver.Resolve(ctx, 0, alias, func(ep *aggregate.Endpoint) bool {
		return ep.SupportOpenAIChatCompletion()
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if ep.AggregateID() != 2 {
		t.Fatalf("endpoint id = %d, want 2", ep.AggregateID())
	}
	if m.UpstreamModel() != "gpt-upstream" {
		t.Fatalf("upstream model = %q, want %q", m.UpstreamModel(), "gpt-upstream")
	}
}

func TestEndpointResolver_Resolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range loadCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			modelRepo := newStubModelRepo(tc.ModelBehavior)
			endpointRepo := &stubEndpointRepo{}
			resolver := service.NewEndpointResolver(endpointRepo, modelRepo)

			ep, m, err := resolver.Resolve(ctx, 0, vo.EndpointAlias(tc.Alias), nil)

			switch tc.ExpectErrKind {
			case "":
				if err != nil {
					t.Fatalf("expected success, got err: %v", err)
				}
				if !tc.ExpectResolved {
					if ep != nil || m != nil {
						t.Fatal("expected nil endpoint and model, got non-nil")
					}
				} else {
					if ep == nil || m == nil {
						t.Fatal("expected endpoint and model resolved, got nil")
					}
					if m.Alias().String() != tc.Alias {
						t.Errorf("alias = %q, want %q", m.Alias().String(), tc.Alias)
					}
				}
			case "validation":
				if !errors.Is(err, ierr.ErrValidation) {
					t.Errorf("expected ErrValidation, got: %v", err)
				}
			case "not_exists":
				if !errors.Is(err, ierr.ErrDataNotExists) {
					t.Errorf("expected ErrDataNotExists, got: %v", err)
				}
			case "db_query":
				if !errors.Is(err, ierr.ErrDBQuery) {
					t.Errorf("expected ErrDBQuery to propagate upward, got: %v", err)
				}
				if errors.Is(err, ierr.ErrDataNotExists) {
					t.Errorf("DB error must not be masked as ErrDataNotExists, got: %v", err)
				}
			default:
				t.Fatalf("unknown expectErrKind: %q", tc.ExpectErrKind)
			}
		})
	}
}

// ownedModelRepo 仅对指定 (userID, alias) 返回命中，验证 Resolve 的用户隔离语义。
type ownedModelRepo struct {
	ownerUserID uint
	alias       string
}

func (r *ownedModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}

func (r *ownedModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias, userID uint) ([]*aggregate.Model, error) {
	if userID == r.ownerUserID && alias.String() == r.alias {
		m, _ := aggregate.CreateModel(1, alias, "upstream-x", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
		return []*aggregate.Model{m}, nil
	}
	return nil, nil
}
func (r *ownedModelRepo) FindByID(context.Context, uint, uint) (*aggregate.Model, error) {
	return nil, nil
}
func (r *ownedModelRepo) Create(context.Context, *aggregate.Model, uint) (uint, error) { return 0, nil }
func (r *ownedModelRepo) Update(context.Context, *aggregate.Model) error               { return nil }
func (r *ownedModelRepo) Delete(context.Context, uint, uint) error                     { return nil }
func (r *ownedModelRepo) DeleteByEndpointID(context.Context, uint) error               { return nil }
func (r *ownedModelRepo) List(context.Context) ([]*aggregate.Model, error)             { return nil, nil }
func (r *ownedModelRepo) Paginate(context.Context, model.CommonParam, uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

// TestEndpointResolver_UserIsolation 非归属用户的别名解析必须返回 ErrDataNotExists。
func TestEndpointResolver_UserIsolation(t *testing.T) {
	t.Parallel()

	resolver := service.NewEndpointResolver(
		&stubEndpointRepo{},
		&ownedModelRepo{ownerUserID: 101, alias: "gpt-x"},
	)

	// 归属用户解析成功
	_, _, err := resolver.Resolve(context.Background(), 101, vo.EndpointAlias("gpt-x"), nil)
	if err != nil {
		t.Fatalf("owner resolve should succeed: %v", err)
	}

	// 非归属用户 → 数据不存在（不泄露他人配置存在性）
	_, _, err = resolver.Resolve(context.Background(), 202, vo.EndpointAlias("gpt-x"), nil)
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("non-owner resolve should be ErrDataNotExists, got %v", err)
	}
}
