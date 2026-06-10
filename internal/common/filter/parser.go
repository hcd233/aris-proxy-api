// Package filter 提供 filter 表达式解析能力
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
package filter

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Filter 表达式
type Filter struct {
	Field    string
	Operator enum.Operator
	Value    string
}

// FilterCriteria 筛选条件（用于 Repository 接口）
type FilterCriteria struct {
	Filters      []Filter
	FieldConfigs map[string]FieldConfig
}

// FieldConfig 字段配置
type FieldConfig struct {
	SQLColumn string
	IsFuzzy   bool
	IsNumeric bool
	ValueMap  map[string]*string
}

// operatorInfo 操作符信息
type operatorInfo struct {
	op  enum.Operator
	len int
}

// Parse 解析 filter 表达式
// 格式: "field:value field2:!value2 field3:>100"
func Parse(expr string) ([]Filter, error) {
	if expr == "" {
		return nil, nil
	}

	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}

	parts := splitExpression(expr)
	filters := make([]Filter, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		f, err := parsePart(part)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}

	return filters, nil
}

// splitExpression 按空格分割表达式，但保留引号内的空格
func splitExpression(expr string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false

	for _, ch := range expr {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parsePart 解析单个 filter 部分
func parsePart(part string) (Filter, error) {
	// 尝试匹配操作符（按长度降序）
	operators := []operatorInfo{
		{enum.OpGTE, 3},
		{enum.OpLTE, 3},
		{enum.OpNotEqual, 2},
		{enum.OpGreater, 2},
		{enum.OpLess, 2},
		{enum.OpEqual, 1},
	}

	for _, opInfo := range operators {
		idx := strings.Index(part, string(opInfo.op))
		if idx > 0 {
			field := part[:idx]
			value := part[idx+opInfo.len:]

			value = strings.Trim(value, `"`)

			field = strings.TrimSpace(field)
			if field == "" {
				return Filter{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrEmptyFieldName, part)
			}

			return Filter{
				Field:    field,
				Operator: opInfo.op,
				Value:    value,
			}, nil
		}
	}

	return Filter{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidExpr, part)
}

// ToSQL 将 filter 转换为 SQL 条件
func ToSQL(filters []Filter, fieldConfigs map[string]FieldConfig) (sql string, args []any, err error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	var conditions []string

	for _, f := range filters {
		config, ok := fieldConfigs[f.Field]
		if !ok {
			return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnknownField, f.Field)
		}

		condition, condArgs, err := buildCondition(f, config)
		if err != nil {
			return "", nil, err
		}

		conditions = append(conditions, condition)
		args = append(args, condArgs...)
	}

	return strings.Join(conditions, constant.FilterSQLAND), args, nil
}

// buildCondition 构建单个 SQL 条件
func buildCondition(f Filter, config FieldConfig) (sql string, args []any, err error) {
	column := config.SQLColumn

	if config.ValueMap != nil {
		if mapped, ok := config.ValueMap[f.Value]; ok {
			if mapped == nil {
				switch f.Operator {
				case enum.OpEqual:
					return column + constant.FilterSQLISNULL, nil, nil
				case enum.OpNotEqual:
					return column + constant.FilterSQLISNOTNULL, nil, nil
				default:
					return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrNullValueOp, f.Operator)
				}
			}
			return buildSimpleCondition(column, f.Operator, *mapped)
		}
	}

	if config.IsFuzzy && f.Operator == enum.OpEqual {
		return column + constant.FilterSQLLIKE, []any{"%" + f.Value + "%"}, nil
	}
	if config.IsFuzzy && f.Operator == enum.OpNotEqual {
		return column + constant.FilterSQLNOTLIKE, []any{"%" + f.Value + "%"}, nil
	}

	if config.IsNumeric {
		return buildSimpleCondition(column, f.Operator, f.Value)
	}

	return buildSimpleCondition(column, f.Operator, f.Value)
}

// buildSimpleCondition 构建简单条件
func buildSimpleCondition(column string, op enum.Operator, value string) (sql string, args []any, err error) {
	switch op {
	case enum.OpEqual:
		return column + constant.FilterSQLEQ, []any{value}, nil
	case enum.OpNotEqual:
		return column + constant.FilterSQLNEQ, []any{value}, nil
	case enum.OpGreater:
		return column + constant.FilterSQLGT, []any{value}, nil
	case enum.OpLess:
		return column + constant.FilterSQLLT, []any{value}, nil
	case enum.OpGTE:
		return column + constant.FilterSQLGTE, []any{value}, nil
	case enum.OpLTE:
		return column + constant.FilterSQLLTE, []any{value}, nil
	default:
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, op)
	}
}
