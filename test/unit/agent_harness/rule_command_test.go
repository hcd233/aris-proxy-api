package agent_harness

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/agentharness/command"
	harnessport "github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/application/agentharness/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

// fakeRuleRepo 记录命令层的调用与入参。
type fakeRuleRepo struct {
	rules    []agentharness.Rule
	created  *agentharness.Rule
	updated  *agentharness.Rule
	deleted  uint
	getErr   error
	listErr  error
	updateOf func(context.Context, agentharness.Rule) error
}

func (f *fakeRuleRepo) List(context.Context) ([]agentharness.Rule, error) {
	return f.rules, f.listErr
}

func (f *fakeRuleRepo) Get(_ context.Context, id uint) (*agentharness.Rule, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for i := range f.rules {
		if f.rules[i].ID == id {
			return &f.rules[i], nil
		}
	}
	return nil, nil
}

func (f *fakeRuleRepo) Create(_ context.Context, rule agentharness.Rule) (*agentharness.Rule, error) {
	f.created = &rule
	rule.ID = 99
	return &rule, nil
}

func (f *fakeRuleRepo) Update(_ context.Context, rule agentharness.Rule) error {
	f.updated = &rule
	return nil
}

func (f *fakeRuleRepo) Delete(_ context.Context, id uint) error {
	f.deleted = id
	return nil
}

func (f *fakeRuleRepo) Fingerprint(context.Context) (string, error) { return "", nil }

func TestCreateHarnessRuleNormalizesInput(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{}
	handler := command.NewCreateHarnessRuleHandler(repo)

	view, err := handler.Handle(context.Background(), harnessport.CreateHarnessRuleCommand{
		Pattern: "  ZCode/ ",
		Name:    "ZCode",
		Sort:    20,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created == nil {
		t.Fatal("repository.Create was not called")
	}
	if repo.created.Pattern != "zcode/" {
		t.Errorf("persisted pattern = %q, want zcode/", repo.created.Pattern)
	}
	if view == nil || view.Name != "ZCode" {
		t.Errorf("view = %+v, want name ZCode", view)
	}
}

func TestCreateHarnessRuleRejectsReservedName(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{}
	handler := command.NewCreateHarnessRuleHandler(repo)

	_, err := handler.Handle(context.Background(), harnessport.CreateHarnessRuleCommand{Pattern: "x/", Name: "other"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
	if repo.created != nil {
		t.Error("repository.Create must not be called for invalid input")
	}
}

func TestCreateHarnessRuleRejectsDuplicatePattern(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{rules: []agentharness.Rule{{ID: 1, Pattern: "zcode/", Name: "ZCode", Sort: 10}}}
	handler := command.NewCreateHarnessRuleHandler(repo)

	_, err := handler.Handle(context.Background(), harnessport.CreateHarnessRuleCommand{Pattern: "ZCode/", Name: "ZCode2"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation for duplicate pattern", err)
	}
	if repo.created != nil {
		t.Error("repository.Create must not be called for duplicate pattern")
	}
}

func TestUpdateHarnessRuleRejectsMissingRule(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{}
	handler := command.NewUpdateHarnessRuleHandler(repo)

	_, err := handler.Handle(context.Background(), harnessport.UpdateHarnessRuleCommand{ID: 404, Pattern: "x/", Name: "X"})
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("err = %v, want ErrDataNotExists", err)
	}
	if repo.updated != nil {
		t.Error("repository.Update must not be called when the rule is missing")
	}
}

func TestPreviewHarnessResolvesRule(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{rules: []agentharness.Rule{{ID: 1, Pattern: "zcode/", Name: "ZCode", Sort: 10}}}
	handler := query.NewPreviewHarnessHandler(repo)

	result, err := handler.Handle(context.Background(), harnessport.PreviewHarnessQuery{UA: "ZCode/3.15.3 ai-sdk"})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if result.Harness != "ZCode" || result.Rule == nil || result.Rule.Pattern != "zcode/" {
		t.Errorf("result = %+v, want ZCode via zcode/ rule", result)
	}
}

func TestPreviewHarnessFallsBackToOther(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{}
	handler := query.NewPreviewHarnessHandler(repo)

	result, err := handler.Handle(context.Background(), harnessport.PreviewHarnessQuery{UA: "curl/8.14.1"})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if result.Harness != constant.AgentHarnessOther || result.Rule != nil {
		t.Errorf("result = %+v, want Other without a rule", result)
	}
}

func TestListHarnessRulesOrdersByView(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{rules: []agentharness.Rule{
		{ID: 1, Pattern: "zcode/", Name: "ZCode", Sort: 10},
		{ID: 2, Pattern: "pi (", Name: "Pi", Sort: 20},
	}}
	handler := query.NewListHarnessRulesHandler(repo)

	views, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(views) != 2 || views[0].Pattern != "zcode/" || views[1].Name != "Pi" {
		t.Errorf("views = %+v, want rules in repository order", views)
	}
}

func TestUpdateHarnessRuleRejectsDuplicatePattern(t *testing.T) {
	t.Parallel()

	repo := &fakeRuleRepo{rules: []agentharness.Rule{
		{ID: 1, Pattern: "zcode/", Name: "ZCode", Sort: 10},
		{ID: 2, Pattern: "pi (", Name: "Pi", Sort: 20},
	}}
	handler := command.NewUpdateHarnessRuleHandler(repo)

	_, err := handler.Handle(context.Background(), harnessport.UpdateHarnessRuleCommand{ID: 1, Pattern: "PI (", Name: "Pi2"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation for duplicate pattern", err)
	}
	if repo.updated != nil {
		t.Error("repository.Update must not be called for a duplicate pattern")
	}
}
