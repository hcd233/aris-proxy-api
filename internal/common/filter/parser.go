// Package filter 提供 filter 表达式解析能力
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
package filter

import (
	"fmt"
	"strings"
)

// Operator 操作符
type Operator string

const (
	OpEqual    Operator = ":"   // 等于/包含
	OpNotEqual Operator = ":!"  // 不等于/不包含
	OpGreater  Operator = ":>"  // 大于
	OpLess     Operator = ":<"  // 小于
	OpGTE      Operator = ":>=" // 大于等于
	OpLTE      Operator = ":<=" // 小于等于
)

// Filter 表达式
type Filter struct {
	Field    string
	Operator Operator
	Value    string
}

// FieldConfig 字段配置
type FieldConfig struct {
	// SQLColumn 对应的数据库列名
	SQLColumn string
	// IsFuzzy 是否模糊匹配
	IsFuzzy bool
	// IsNumeric 是否数值类型
	IsNumeric bool
	// ValueMap 特殊值映射（如 "none" -> nil）
	ValueMap map[string]*string
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
	operators := []struct {
		op  Operator
		len int
	}{
		{OpGTE, 3},
		{OpLTE, 3},
		{OpNotEqual, 2},
		{OpGreater, 2},
		{OpLess, 2},
		{OpEqual, 1},
	}

	for _, opInfo := range operators {
		idx := strings.Index(part, string(opInfo.op))
		if idx > 0 {
			field := part[:idx]
			value := part[idx+opInfo.len:]

			// 去除引号
			value = strings.Trim(value, `"`)

			return Filter{
				Field:    field,
				Operator: opInfo.op,
				Value:    value,
			}, nil
		}
	}

	return Filter{}, fmt.Errorf("invalid filter expression: %s", part)
}

// ToSQL 将 filter 转换为 SQL 条件
func ToSQL(filters []Filter, fieldConfigs map[string]FieldConfig) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	var conditions []string
	var args []any

	for _, f := range filters {
		config, ok := fieldConfigs[f.Field]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter field: %s", f.Field)
		}

		condition, condArgs, err := buildCondition(f, config)
		if err != nil {
			return "", nil, err
		}

		conditions = append(conditions, condition)
		args = append(args, condArgs...)
	}

	return strings.Join(conditions, " AND "), args, nil
}

// buildCondition 构建单个 SQL 条件
func buildCondition(f Filter, config FieldConfig) (string, []any, error) {
	column := config.SQLColumn

	// 检查特殊值映射
	if config.ValueMap != nil {
		if mapped, ok := config.ValueMap[f.Value]; ok {
			if mapped == nil {
				// NULL 值
				switch f.Operator {
				case OpEqual:
					return column + " IS NULL", nil, nil
				case OpNotEqual:
					return column + " IS NOT NULL", nil, nil
				default:
					return "", nil, fmt.Errorf("operator %s not supported for NULL value", f.Operator)
				}
			}
			// 使用映射后的值
			return buildSimpleCondition(column, f.Operator, *mapped)
		}
	}

	// 模糊匹配
	if config.IsFuzzy && f.Operator == OpEqual {
		return column + " LIKE ?", []any{"%" + f.Value + "%"}, nil
	}
	if config.IsFuzzy && f.Operator == OpNotEqual {
		return column + " NOT LIKE ?", []any{"%" + f.Value + "%"}, nil
	}

	// 数值比较
	if config.IsNumeric {
		return buildSimpleCondition(column, f.Operator, f.Value)
	}

	// 默认精确匹配
	return buildSimpleCondition(column, f.Operator, f.Value)
}

// buildSimpleCondition 构建简单条件
func buildSimpleCondition(column string, op Operator, value string) (string, []any, error) {
	switch op {
	case OpEqual:
		return column + " = ?", []any{value}, nil
	case OpNotEqual:
		return column + " != ?", []any{value}, nil
	case OpGreater:
		return column + " > ?", []any{value}, nil
	case OpLess:
		return column + " < ?", []any{value}, nil
	case OpGTE:
		return column + " >= ?", []any{value}, nil
	case OpLTE:
		return column + " <= ?", []any{value}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", op)
	}
}
