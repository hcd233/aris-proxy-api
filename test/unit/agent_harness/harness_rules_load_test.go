package agent_harness

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type stubRuleRepository struct {
	rules []agentharness.Rule
	err   error
}

func (s *stubRuleRepository) List(context.Context) ([]agentharness.Rule, error) {
	return s.rules, s.err
}

func (s *stubRuleRepository) Get(context.Context, uint) (*agentharness.Rule, error) {
	return nil, s.err
}

func (s *stubRuleRepository) Create(context.Context, agentharness.Rule) (*agentharness.Rule, error) {
	return nil, s.err
}

func (s *stubRuleRepository) Update(context.Context, agentharness.Rule) error {
	return s.err
}

func (s *stubRuleRepository) Delete(context.Context, uint) error {
	return s.err
}

func (s *stubRuleRepository) Fingerprint(context.Context) (string, error) {
	return "", s.err
}

func TestLoadHarnessRulesReturnsRules(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepository{rules: []agentharness.Rule{{Pattern: "zcode/", Name: "ZCode"}}}

	rules := query.LoadHarnessRules(context.Background(), repo)
	if len(rules) != 1 || rules[0].Name != "ZCode" {
		t.Fatalf("LoadHarnessRules = %+v, want one ZCode rule", rules)
	}
	if got := agentharness.Match("ZCode/3.15.3", rules); got != "ZCode" {
		t.Errorf("Match with loaded rules = %q, want ZCode", got)
	}
}

func TestLoadHarnessRulesFailsOpen(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepository{err: ierr.New(ierr.ErrDBQuery, "db down")}

	rules := query.LoadHarnessRules(context.Background(), repo)
	if rules != nil {
		t.Fatalf("LoadHarnessRules = %+v, want nil on repository error", rules)
	}
	if got := agentharness.Match("ZCode/3.15.3", rules); got != constant.AgentHarnessOther {
		t.Errorf("Match with failed rules = %q, want %q", got, constant.AgentHarnessOther)
	}
}

func TestLoadHarnessRulesWithoutRepository(t *testing.T) {
	t.Parallel()

	if rules := query.LoadHarnessRules(context.Background(), nil); rules != nil {
		t.Fatalf("LoadHarnessRules(nil repo) = %+v, want nil", rules)
	}
}
