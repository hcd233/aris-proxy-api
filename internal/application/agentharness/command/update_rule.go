package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type updateHarnessRuleHandler struct {
	repo agentharness.RuleRepository
}

// NewUpdateHarnessRuleHandler 构造修改规则命令
func NewUpdateHarnessRuleHandler(repo agentharness.RuleRepository) port.UpdateHarnessRuleHandler {
	return &updateHarnessRuleHandler{repo: repo}
}

// Handle 校验存在性 → 规范化 → 覆盖。
func (h *updateHarnessRuleHandler) Handle(ctx context.Context, cmd port.UpdateHarnessRuleCommand) (*port.HarnessRuleView, error) {
	existing, err := h.repo.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ierr.Newf(ierr.ErrDataNotExists, "harness rule %d not found", cmd.ID)
	}

	pattern, err := agentharness.NormalizePattern(cmd.Pattern)
	if err != nil {
		return nil, err
	}
	name, err := agentharness.NormalizeName(cmd.Name)
	if err != nil {
		return nil, err
	}

	updated := agentharness.Rule{ID: cmd.ID, Pattern: pattern, Name: name, Sort: cmd.Sort}
	if err := h.repo.Update(ctx, updated); err != nil {
		return nil, err
	}
	return toHarnessRuleView(&updated), nil
}
