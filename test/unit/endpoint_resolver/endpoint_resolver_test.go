package endpoint_resolver

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
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

var errStubDBFailure = ierr.Wrap(ierr.ErrDBQuery, errors.New("simulated db outage"), "stub db query")

func (s *stubModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error) {
	s.callsByAlias[alias.String()]++
	b, ok := s.behaviorByAlias[alias.String()]
	if !ok {
		b = s.behaviorByAlias["default"]
	}
	switch b {
	case "hit":
		m, _ := aggregate.CreateModel(1, alias, "test-model", 1)
		return []*aggregate.Model{m}, nil
	case "miss":
		return nil, nil
	case "db_error":
		return nil, errStubDBFailure
	default:
		return nil, errors.New("unknown stub behavior")
	}
}

type stubEndpointRepo struct {
	findByIDCalled bool
}

func (s *stubEndpointRepo) FindByID(_ context.Context, id uint) (*aggregate.Endpoint, error) {
	s.findByIDCalled = true
	if id == 0 {
		return nil, nil
	}
	return aggregate.CreateEndpoint(id, "test-endpoint", "https://api.openai.com", "https://api.anthropic.com", "sk-test", true, false, true)
}

type staticModelRepo struct {
	models []*aggregate.Model
}

func (s *staticModelRepo) FindByAlias(_ context.Context, _ vo.EndpointAlias) ([]*aggregate.Model, error) {
	return s.models, nil
}

type endpointByIDRepo struct {
	endpoints map[uint]*aggregate.Endpoint
}

func (s *endpointByIDRepo) FindByID(_ context.Context, id uint) (*aggregate.Endpoint, error) {
	return s.endpoints[id], nil
}

func TestEndpointResolver_ResolveFiltersUnsupportedEndpoints(t *testing.T) {
	ctx := context.Background()
	alias := vo.EndpointAlias("test-model")
	anthropicOnly, _ := aggregate.CreateEndpoint(1, "anthropic-only", "", "https://api.anthropic.com", "sk-ant", false, false, true)
	openAIOnly, _ := aggregate.CreateEndpoint(2, "openai-only", "https://api.openai.com", "", "sk-openai", true, false, false)
	anthropicModel, _ := aggregate.CreateModel(1, alias, "claude-upstream", 1)
	openAIModel, _ := aggregate.CreateModel(2, alias, "gpt-upstream", 2)
	resolver := service.NewEndpointResolver(
		&endpointByIDRepo{endpoints: map[uint]*aggregate.Endpoint{1: anthropicOnly, 2: openAIOnly}},
		&staticModelRepo{models: []*aggregate.Model{anthropicModel, openAIModel}},
	)

	ep, m, err := resolver.Resolve(ctx, alias, func(ep *aggregate.Endpoint) bool {
		return ep.SupportOpenAIChatCompletion()
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if ep.AggregateID() != 2 {
		t.Fatalf("endpoint id = %d, want 2", ep.AggregateID())
	}
	if m.ModelName() != "gpt-upstream" {
		t.Fatalf("model name = %q, want %q", m.ModelName(), "gpt-upstream")
	}
}

func TestEndpointResolver_Resolve(t *testing.T) {
	ctx := context.Background()

	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			modelRepo := newStubModelRepo(tc.ModelBehavior)
			endpointRepo := &stubEndpointRepo{}
			resolver := service.NewEndpointResolver(endpointRepo, modelRepo)

			ep, m, err := resolver.Resolve(ctx, vo.EndpointAlias(tc.Alias), nil)

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
