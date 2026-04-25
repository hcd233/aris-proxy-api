// Package endpoint_resolver 验证 domain/llmproxy/service.EndpointResolver
// 的 primary→fallback 查询语义：命中返回、未命中回退、真 DB 错误不被降级。
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
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// resolverCase fixture 用例结构
type resolverCase struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	PrimaryBehavior  string `json:"primaryBehavior"`
	FallbackBehavior string `json:"fallbackBehavior"`
	Alias            string `json:"alias"`
	ExpectResolved   bool   `json:"expectResolved"`
	ExpectProvider   string `json:"expectProvider"`
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

// stubRepository 按 provider 维度配置每一路查询的行为
type stubRepository struct {
	// behaviorByProvider: provider → "hit" | "miss" | "db_error" | "unused"
	behaviorByProvider map[enum.ProviderType]string
	// callsByProvider 记录每个 provider 被访问次数，供断言"未访问"
	callsByProvider map[enum.ProviderType]int
}

func newStubRepository(primary, fallback enum.ProviderType, primaryBehavior, fallbackBehavior string) *stubRepository {
	return &stubRepository{
		behaviorByProvider: map[enum.ProviderType]string{
			primary:  primaryBehavior,
			fallback: fallbackBehavior,
		},
		callsByProvider: map[enum.ProviderType]int{},
	}
}

// errStubDBFailure 模拟仓储层包装后的 DB 错误
var errStubDBFailure = ierr.Wrap(ierr.ErrDBQuery, errors.New("simulated db outage"), "stub db query")

// FindByAliasAndProvider 实现 EndpointRepository 接口
func (s *stubRepository) FindByAliasAndProvider(_ context.Context, alias vo.EndpointAlias, provider enum.ProviderType) (*aggregate.Endpoint, error) {
	s.callsByProvider[provider]++
	switch s.behaviorByProvider[provider] {
	case "hit":
		return aggregate.NewEndpoint(1, alias, provider, vo.NewUpstreamCreds("", "", "")), nil
	case "miss":
		return nil, nil
	case "db_error":
		return nil, errStubDBFailure
	case "unused":
		// 被判定为 unused 的 provider 不应被访问；命中即视为测试失败（由调用方断言）
		return nil, errors.New("unused provider should not be queried")
	default:
		return nil, errors.New("unknown stub behavior")
	}
}

func TestEndpointResolver_Resolve(t *testing.T) {
	ctx := context.Background()
	primary := enum.ProviderOpenAI
	fallback := enum.ProviderAnthropic

	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			repo := newStubRepository(primary, fallback, tc.PrimaryBehavior, tc.FallbackBehavior)
			resolver := service.NewEndpointResolver(repo)

			ep, err := resolver.Resolve(ctx, vo.EndpointAlias(tc.Alias), primary, fallback)

			// 断言 unused provider 未被访问
			if tc.PrimaryBehavior == "unused" && repo.callsByProvider[primary] != 0 {
				t.Errorf("primary marked unused but queried %d times", repo.callsByProvider[primary])
			}
			if tc.FallbackBehavior == "unused" && repo.callsByProvider[fallback] != 0 {
				t.Errorf("fallback marked unused but queried %d times", repo.callsByProvider[fallback])
			}

			switch tc.ExpectErrKind {
			case "":
				if err != nil {
					t.Fatalf("expected success, got err: %v", err)
				}
				if !tc.ExpectResolved || ep == nil {
					t.Fatalf("expected endpoint resolved, got nil")
				}
				if string(ep.Provider()) != tc.ExpectProvider {
					t.Errorf("provider = %q, want %q", ep.Provider(), tc.ExpectProvider)
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
					t.Errorf("expected ErrDBQuery to propagate upward (not masked as ErrDataNotExists), got: %v", err)
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
