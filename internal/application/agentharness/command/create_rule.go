package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type createHarnessRuleHandler struct {
	repo agentharness.RuleRepository
}

// NewCreateHarnessRuleHandler 构造新增规则命令
func NewCreateHarnessRuleHandler(repo agentharness.RuleRepository) port.CreateHarnessRuleHandler {
	return &createHarnessRuleHandler{repo: repo}
}

// Handle 规范化 → 查重 → 落库。
//
// pattern 在 (全局) 唯一索引上冲突，因此先查重给出可读错误，
// 而不是把数据库唯一约束错误透出给前端。
func (h *createHarnessRuleHandler) Handle(ctx context.Context, cmd port.CreateHarnessRuleCommand) (*port.HarnessRuleView, error) {
	pattern, err := agentharness.NormalizePattern(cmd.Pattern)
	if err != nil {
		return nil, err
	}
	name, err := agentharness.NormalizeName(cmd.Name)
	if err != nil {
		return nil, err
	}

	existing, err := h.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, rule := range existing {
		if rule.Pattern == pattern {
			return nil, ierr.Newf(ierr.ErrValidation, "harness rule pattern %q already exists", pattern)
		}
	}

	created, err := h.repo.Create(ctx, agentharness.Rule{Pattern: pattern, Name: name, Sort: cmd.Sort})
	if err != nil {
		return nil, err
	}
	return toHarnessRuleView(created), nil
}

func toHarnessRuleView(rule *agentharness.Rule) *port.HarnessRuleView {
	if rule == nil {
		return nil
	}
	return &port.HarnessRuleView{
		ID:      rule.ID,
		Pattern: rule.Pattern,
		Name:    rule.Name,
		Sort:    rule.Sort,
	}
}
