package query

import (
	"context"

	harnessport "github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type listHarnessRulesHandler struct {
	repo agentharness.RuleRepository
}

// NewListHarnessRulesHandler 构造规则列表查询
func NewListHarnessRulesHandler(repo agentharness.RuleRepository) harnessport.ListHarnessRulesHandler {
	return &listHarnessRulesHandler{repo: repo}
}

// Handle 返回全部规则，顺序即匹配优先级。
func (h *listHarnessRulesHandler) Handle(ctx context.Context) ([]*harnessport.HarnessRuleView, error) {
	rules, err := h.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]*harnessport.HarnessRuleView, 0, len(rules))
	for _, rule := range rules {
		views = append(views, &harnessport.HarnessRuleView{
			ID:      rule.ID,
			Pattern: rule.Pattern,
			Name:    rule.Name,
			Sort:    rule.Sort,
		})
	}
	return views, nil
}
