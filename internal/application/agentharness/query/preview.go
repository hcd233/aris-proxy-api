package query

import (
	"context"
	"strings"

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

	harness := agentharness.Match(q.UA, rules)
	result := &harnessport.PreviewHarnessResult{Harness: harness}
	if harness == constant.AgentHarnessOther {
		return result, nil
	}

	normalized := strings.ToLower(strings.TrimSpace(q.UA))
	for _, rule := range rules {
		if strings.HasPrefix(normalized, rule.Pattern) {
			result.Rule = &harnessport.HarnessRuleView{
				ID:      rule.ID,
				Pattern: rule.Pattern,
				Name:    rule.Name,
				Sort:    rule.Sort,
			}
			return result, nil
		}
	}
	return result, nil
}
