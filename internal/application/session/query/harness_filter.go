package query

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// BuildHarnessFilterCondition 把 `harness:<name>` 条件编译成 SQL。
//
// harness 不落库，条件直接对原始 UA 现算：标量子查询按 (sort, id) 取首命中规则名，
// 与 Go 侧 Match 同语义；`Other` 等价于「没有任何规则命中」（子查询为 NULL），
// 空 UA 天然落入该分支。
func BuildHarnessFilterCondition(f filter.Filter) (condition string, args []any, err error) {
	if len(f.Values) != 1 {
		return "", nil, ierr.Newf(ierr.ErrValidation, "harness filter accepts exactly one value, got %d", len(f.Values))
	}
	value := f.Values[0]
	isOther := strings.EqualFold(value, constant.AgentHarnessOther)

	switch f.Operator {
	case enum.OpEqual:
		if isOther {
			return constant.SessionHarnessResolveSQL + " IS NULL", nil, nil
		}
		return constant.SessionHarnessResolveSQL + " = ?", []any{value}, nil
	case enum.OpNotEqual:
		if isOther {
			return constant.SessionHarnessResolveSQL + " IS NOT NULL", nil, nil
		}
		return "(" + constant.SessionHarnessResolveSQL + " IS NULL OR " + constant.SessionHarnessResolveSQL + " <> ?)", []any{value}, nil
	default:
		return "", nil, ierr.Newf(ierr.ErrValidation, "unsupported harness filter operator %q", f.Operator)
	}
}
