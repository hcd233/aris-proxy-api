package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// LoadHarnessRules 读取 harness 规则；读取失败时返回 nil 并只记 warn。
//
// harness 是展示与筛选的附属数据：规则表抖动不该让会话列表/详情整体失败，
// Match(ua, nil) 会回落为 Other，界面降级但可用。
func LoadHarnessRules(ctx context.Context, rules agentharness.RuleRepository) []agentharness.Rule {
	if rules == nil {
		return nil
	}
	list, err := rules.List(ctx)
	if err != nil {
		logger.WithCtx(ctx).Warn("[SessionQuery] Load harness rules failed, fallback to Other", zap.Error(err))
		return nil
	}
	return list
}
