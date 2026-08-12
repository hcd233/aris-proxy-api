// Package filter 提供 filter 表达式解析能力
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
package filter

import (
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// Filter 表达式
type Filter struct {
	Field    string
	Operator enum.Operator
	Values   []string
}

// FilterCriteria 筛选条件（用于 Repository 接口）
type FilterCriteria struct {
	Filters      []Filter
	FieldConfigs map[string]FieldConfig
}

// FieldConfig 字段配置
type FieldConfig struct {
	SQLColumn    string
	SQLExpr      string // 优先于 SQLColumn 的 SQL 表达式片段（如 jsonb_array_length(message_ids::jsonb)）
	IsFuzzy      bool
	IsNumeric    bool
	IsJSONBArray bool
	IsRange      bool // 值格式 "min-max"，生成区间（BETWEEN）条件
	ValueMap     map[string]*string
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

	parts := lo.Filter(splitExpression(expr), func(part string, _ int) bool {
		return strings.TrimSpace(part) != ""
	})
	return util.MapErr(parts, func(part string, _ int) (Filter, error) {
		return parsePart(part)
	})
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
			field := strings.TrimSpace(part[:idx])
			rawValue := part[idx+opInfo.len:]

			if field == "" {
				return Filter{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrEmptyFieldName, part)
			}

			values := splitValues(rawValue)
			if len(values) == 0 {
				return Filter{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidExpr, part)
			}

			return Filter{
				Field:    field,
				Operator: opInfo.op,
				Values:   values,
			}, nil
		}
	}

	return Filter{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidExpr, part)
}

// splitValues 解析 value 段：
//   - 整体引号包裹 → 单值字面量（不按 | 拆）
//   - 否则按 | 拆分，trim 空白，剔除空项
func splitValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return []string{raw[1 : len(raw)-1]}
	}
	values := lo.FilterMap(strings.Split(raw, "|"), func(p string, _ int) (string, bool) {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		return p, p != ""
	})
	return values
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

// buildCondition 构建单个 SQL 条件（单值或多值）
func buildCondition(f Filter, config FieldConfig) (sql string, args []any, err error) {
	column := config.SQLColumn
	if config.SQLExpr != "" {
		column = config.SQLExpr
	}

	if !isMultiValueAllowed(f.Operator) && len(f.Values) > 1 {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrMultiValueWithComparison, f.Operator)
	}

	if config.IsRange {
		return buildRangeCondition(column, f)
	}

	if config.IsJSONBArray {
		return buildJSONBArrayCondition(column, f)
	}

	if config.ValueMap != nil {
		return buildValueMapCondition(column, f, config)
	}

	if config.IsFuzzy {
		return buildFuzzyCondition(column, f)
	}

	if config.IsNumeric {
		return buildNumericCondition(column, f)
	}

	return buildPlainCondition(column, f)
}

// isMultiValueAllowed 判定操作符是否支持多值
func isMultiValueAllowed(op enum.Operator) bool {
	return op == enum.OpEqual || op == enum.OpNotEqual
}

// rangeValue 解析后的区间值
type rangeValue struct {
	min int
	max int
}

// parseRangeValue 解析 "min-max" 格式的区间值，min/max 均为非负整数且 min <= max
func parseRangeValue(v string) (rangeValue, error) {
	parts := strings.SplitN(v, "-", 2)
	if len(parts) != 2 {
		return rangeValue{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidRange, v)
	}
	min, err := strconv.Atoi(parts[0])
	if err != nil || min < 0 {
		return rangeValue{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidRange, v)
	}
	max, err := strconv.Atoi(parts[1])
	if err != nil || max < min {
		return rangeValue{}, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrInvalidRange, v)
	}
	return rangeValue{min: min, max: max}, nil
}

// buildRangeCondition 构建区间条件（值格式 "min-max"，用于 SQLExpr 计算列）
// 单值：expr >= ? AND expr <= ?；多值 equal：OR 连接；多值 not equal：AND 连接 NOT
func buildRangeCondition(expr string, f Filter) (sql string, args []any, err error) {
	if !isMultiValueAllowed(f.Operator) {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}

	ranges := make([]rangeValue, 0, len(f.Values))
	for _, v := range f.Values {
		r, parseErr := parseRangeValue(v)
		if parseErr != nil {
			return "", nil, parseErr
		}
		ranges = append(ranges, r)
	}

	frag := func(r rangeValue) (string, []any) {
		return "(" + expr + constant.FilterSQLGTE + " AND " + expr + constant.FilterSQLLTE + ")",
			[]any{r.min, r.max}
	}

	switch f.Operator {
	case enum.OpEqual:
		if len(ranges) == 1 {
			part, partArgs := frag(ranges[0])
			return part, partArgs, nil
		}
		parts := make([]string, len(ranges))
		args = make([]any, 0, len(ranges)*2)
		for i, r := range ranges {
			part, partArgs := frag(r)
			parts[i] = part
			args = append(args, partArgs...)
		}
		return "(" + strings.Join(parts, constant.FilterSQLOR) + ")", args, nil
	case enum.OpNotEqual:
		if len(ranges) == 1 {
			part, partArgs := frag(ranges[0])
			return "NOT " + part, partArgs, nil
		}
		parts := make([]string, len(ranges))
		args = make([]any, 0, len(ranges)*2)
		for i, r := range ranges {
			part, partArgs := frag(r)
			parts[i] = "NOT " + part
			args = append(args, partArgs...)
		}
		return "(" + strings.Join(parts, constant.FilterSQLAND) + ")", args, nil
	default:
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}
}

// buildJSONBArrayCondition 构建 JSONB 数组精确包含条件
// 单值：column::jsonb @> jsonb_build_array(?::text)
// 多值 equal：OR 连接多个 @> 条件
// 多值 not equal：AND 连接多个 NOT @> 条件
func buildJSONBArrayCondition(column string, f Filter) (sql string, args []any, err error) {
	if !isMultiValueAllowed(f.Operator) {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}

	jsonbColumn := column + "::jsonb"
	frag := jsonbColumn + " @> jsonb_build_array(?::text)"

	switch f.Operator {
	case enum.OpEqual:
		if len(f.Values) == 1 {
			return frag, []any{f.Values[0]}, nil
		}
		parts := make([]string, len(f.Values))
		args = make([]any, len(f.Values))
		for i, v := range f.Values {
			parts[i] = frag
			args[i] = v
		}
		return "(" + strings.Join(parts, constant.FilterSQLOR) + ")", args, nil
	case enum.OpNotEqual:
		if len(f.Values) == 1 {
			return "NOT " + frag, []any{f.Values[0]}, nil
		}
		parts := make([]string, len(f.Values))
		args = make([]any, len(f.Values))
		for i, v := range f.Values {
			parts[i] = "NOT " + frag
			args[i] = v
		}
		return "(" + strings.Join(parts, constant.FilterSQLAND) + ")", args, nil
	default:
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}
}

// buildFuzzyCondition LIKE / NOT LIKE 单值或 OR/AND 多值
func buildFuzzyCondition(column string, f Filter) (sql string, args []any, err error) {
	if len(f.Values) == 1 {
		switch f.Operator {
		case enum.OpEqual:
			return column + constant.FilterSQLLIKE, []any{"%" + f.Values[0] + "%"}, nil
		case enum.OpNotEqual:
			return column + constant.FilterSQLNOTLIKE, []any{"%" + f.Values[0] + "%"}, nil
		}
	}
	frag := constant.FilterSQLLIKE
	joiner := constant.FilterSQLOR
	if f.Operator == enum.OpNotEqual {
		frag = constant.FilterSQLNOTLIKE
		joiner = constant.FilterSQLAND
	}
	parts := lo.RepeatBy(len(f.Values), func(_ int) string { return column + frag })
	args = lo.Map(f.Values, func(v string, _ int) any { return "%" + v + "%" })
	return "(" + strings.Join(parts, joiner) + ")", args, nil
}

// buildNumericCondition = / != 单值或 IN/NOT IN 多值
func buildNumericCondition(column string, f Filter) (sql string, args []any, err error) {
	if len(f.Values) == 1 {
		return buildSimpleCondition(column, f.Operator, f.Values[0])
	}
	switch f.Operator {
	case enum.OpEqual:
		return column + constant.FilterSQLIN, []any{f.Values}, nil
	case enum.OpNotEqual:
		return column + constant.FilterSQLNOTIN, []any{f.Values}, nil
	}
	return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
}

// buildPlainCondition 无配置标志的字段（既非 fuzzy 也非 numeric 也无 ValueMap）
func buildPlainCondition(column string, f Filter) (sql string, args []any, err error) {
	if len(f.Values) == 1 {
		return buildSimpleCondition(column, f.Operator, f.Values[0])
	}
	return buildNumericCondition(column, f)
}

// resolveValueMapValues 解析 ValueMap 映射，返回 resolved 列表
type valueMapResolved struct {
	isNull bool
	value  string
}

func resolveValueMapValues(values []string, valueMap map[string]*string) []valueMapResolved {
	return lo.Map(values, func(raw string, _ int) valueMapResolved {
		mapped, ok := valueMap[raw]
		if !ok {
			return valueMapResolved{value: raw}
		}
		if mapped == nil {
			return valueMapResolved{isNull: true}
		}
		return valueMapResolved{value: *mapped}
	})
}

// buildValueMapCondition 处理含 ValueMap 的字段，支持单值与多值（含 NULL 项混合）
func buildValueMapCondition(column string, f Filter, config FieldConfig) (sql string, args []any, err error) {
	resolveds := resolveValueMapValues(f.Values, config.ValueMap)

	// 单值快速路径，与原行为完全等价
	if len(resolveds) == 1 {
		return buildSingleValueMapCondition(column, resolveds[0], f, config)
	}

	// 多值：NULL 与非 NULL 混合
	return buildMultiValueMapCondition(column, resolveds, f, config)
}

// buildSingleValueMapCondition 单值 ValueMap 条件
func buildSingleValueMapCondition(column string, r valueMapResolved, f Filter, config FieldConfig) (sql string, args []any, err error) {
	if r.isNull {
		switch f.Operator {
		case enum.OpEqual:
			return column + constant.FilterSQLISNULL, nil, nil
		case enum.OpNotEqual:
			return column + constant.FilterSQLISNOTNULL, nil, nil
		default:
			return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrNullValueOp, f.Operator)
		}
	}
	// 非 NULL 单值，走 fuzzy / numeric / plain 各自分支
	single := Filter{Field: f.Field, Operator: f.Operator, Values: []string{r.value}}
	switch {
	case config.IsFuzzy:
		return buildFuzzyCondition(column, single)
	case config.IsNumeric:
		return buildNumericCondition(column, single)
	default:
		return buildSimpleCondition(column, f.Operator, r.value)
	}
}

// buildMultiValueMapCondition 多值 ValueMap 条件（NULL 与非 NULL 混合）
func buildMultiValueMapCondition(column string, resolveds []valueMapResolved, f Filter, config FieldConfig) (sql string, args []any, err error) {
	parts := make([]string, 0, len(resolveds))
	args = make([]any, 0, len(resolveds))
	joiner := constant.FilterSQLOR
	if f.Operator == enum.OpNotEqual {
		joiner = constant.FilterSQLAND
	}
	for _, r := range resolveds {
		if r.isNull {
			if f.Operator == enum.OpEqual {
				parts = append(parts, column+constant.FilterSQLISNULL)
			} else {
				parts = append(parts, column+constant.FilterSQLISNOTNULL)
			}
			continue
		}
		switch {
		case config.IsFuzzy:
			if f.Operator == enum.OpEqual {
				parts = append(parts, column+constant.FilterSQLLIKE)
			} else {
				parts = append(parts, column+constant.FilterSQLNOTLIKE)
			}
			args = append(args, "%"+r.value+"%")
		default:
			if f.Operator == enum.OpEqual {
				parts = append(parts, column+constant.FilterSQLEQ)
			} else {
				parts = append(parts, column+constant.FilterSQLNEQ)
			}
			args = append(args, r.value)
		}
	}
	return "(" + strings.Join(parts, joiner) + ")", args, nil
}

// buildSimpleCondition 构建单值简单条件（保持原逻辑、原行为）
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
