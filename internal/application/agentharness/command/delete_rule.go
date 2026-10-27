package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type deleteHarnessRuleHandler struct {
	repo agentharness.RuleRepository
}

// NewDeleteHarnessRuleHandler 构造删除规则命令
func NewDeleteHarnessRuleHandler(repo agentharness.RuleRepository) port.DeleteHarnessRuleHandler {
	return &deleteHarnessRuleHandler{repo: repo}
}

// Handle 校验存在性后硬删除；删除后历史数据在下一次查询即回落为 Other。
func (h *deleteHarnessRuleHandler) Handle(ctx context.Context, cmd port.DeleteHarnessRuleCommand) error {
	existing, err := h.repo.Get(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ierr.Newf(ierr.ErrDataNotExists, "harness rule %d not found", cmd.ID)
	}
	return h.repo.Delete(ctx, cmd.ID)
}
