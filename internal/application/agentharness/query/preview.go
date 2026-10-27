package query

import (
	"context"

	harnessport "github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
)

type previewHarnessHandler struct {
	repo agentharness.RuleRepository
}

// NewPreviewHarnessHandler 构造 UA 预览查询
func NewPreviewHarnessHandler(repo agentharness.RuleRepository) harnessport.PreviewHarnessHandler {
	return &previewHarnessHandler{repo: repo}
}

// Handle 用真实规则对一条 UA 求值，返回命中规则与最终展示名。
//
// 用途：新增 harness 前先粘一条线上 UA 验证规则是否命中、命中了哪条。
func (h *previewHarnessHandler) Handle(ctx context.Context, q harnessport.PreviewHarnessQuery) (*harnessport.PreviewHarnessResult, error) {
	rules, err := h.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	matched := agentharness.MatchRule(q.UA, rules)
	result := &harnessport.PreviewHarnessResult{Harness: constant.AgentHarnessOther}
	if matched == nil {
		return result, nil
	}

	result.Harness = matched.Name
	result.Rule = &harnessport.HarnessRuleView{
		ID:      matched.ID,
		Pattern: matched.Pattern,
		Name:    matched.Name,
	}
	return result, nil
}
