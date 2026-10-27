package port

import (
	"context"
)

// HarnessRuleView 规则视图
type HarnessRuleView struct {
	ID      uint
	Pattern string
	Name    string
}

// CreateHarnessRuleCommand 新增规则
type CreateHarnessRuleCommand struct {
	Pattern string
	Name    string
}

// UpdateHarnessRuleCommand 修改规则
type UpdateHarnessRuleCommand struct {
	ID      uint
	Pattern string
	Name    string
}

// DeleteHarnessRuleCommand 删除规则
type DeleteHarnessRuleCommand struct {
	ID uint
}

// PreviewHarnessQuery UA 预览
type PreviewHarnessQuery struct {
	UA string
}

// PreviewHarnessResult 预览结果
type PreviewHarnessResult struct {
	Harness string
	Rule    *HarnessRuleView
}

// ListHarnessRulesHandler 规则列表
type ListHarnessRulesHandler interface {
	Handle(ctx context.Context) ([]*HarnessRuleView, error)
}

// CreateHarnessRuleHandler 新增规则
type CreateHarnessRuleHandler interface {
	Handle(ctx context.Context, cmd CreateHarnessRuleCommand) (*HarnessRuleView, error)
}

// UpdateHarnessRuleHandler 修改规则
type UpdateHarnessRuleHandler interface {
	Handle(ctx context.Context, cmd UpdateHarnessRuleCommand) (*HarnessRuleView, error)
}

// DeleteHarnessRuleHandler 删除规则
type DeleteHarnessRuleHandler interface {
	Handle(ctx context.Context, cmd DeleteHarnessRuleCommand) error
}

// PreviewHarnessHandler UA 预览
type PreviewHarnessHandler interface {
	Handle(ctx context.Context, q PreviewHarnessQuery) (*PreviewHarnessResult, error)
}
