# Audit / Sessions 过滤器 Pill 多选化 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Audit 页 3 个 Select（User/Model/Status）和 Sessions 页 1 个 Select（Score）替换为支持多选的 Pill 控件，并扩展后端 filter 解析器以支持同字段多值 OR。

**Architecture:** 后端 `internal/common/filter/parser.go` 扩展为接受 `field:val1|val2|val3` 形式的多值表达式（同字段 OR、跨字段 AND，引号保护字面值），`Filter.Value string` 改为 `Values []string`；前端新增 `MultiSelectPill` 通用组件，audit / sessions 两页分别接入并把 `filterX: string` 改为 `filterX: string[]`。搜索框、TimeRangePicker、行操作、分页一律不动。

**Tech Stack:** Go 1.25.1 / GORM / PostgreSQL（后端）+ Next.js 16 / React 19 / Tailwind v4 / @base-ui/react Popover（前端）。

**Spec:** `docs/superpowers/specs/2026-06-10-filter-pill-multiselect-design.md`

---

## File Structure

**Backend:**
- Modify: `internal/common/filter/parser.go` — Filter 结构 / Parse / ToSQL 多值分支
- Modify: `internal/common/constant/filter.go` — 新增 IN / NOTIN / OR / 错误信息常量
- Modify: `test/unit/filter/parser_test.go` — 现有用例迁移 + 新多值用例

**Frontend:**
- Create: `web/src/components/ui/multi-select-pill.tsx` — 通用多选 Pill 组件
- Modify: `web/src/app/(dashboard)/audit/page.tsx` — 接入 3 个 Pill
- Modify: `web/src/app/(dashboard)/sessions/page.tsx` — 接入 1 个 Pill + 清理 `OptionItem` 失效 import

---

## Task 0: 创建 worktree 与开发分支

**Files:**
- 工作区切换到 `.worktrees/feature-filter-pill-multiselect-2026-06-10`

- [ ] **Step 1：创建 worktree 并切换分支**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
git worktree add -b feature/filter-pill-multiselect-2026-06-10 \
  .worktrees/feature-filter-pill-multiselect-2026-06-10 master
```

预期输出包含 `Preparing worktree (new branch …)`、`HEAD is now at …`。

- [ ] **Step 2：cd 进入 worktree，确认 spec 文件存在**

```bash
cd .worktrees/feature-filter-pill-multiselect-2026-06-10
ls docs/superpowers/specs/2026-06-10-filter-pill-multiselect-design.md
```

预期：列出该文件路径。后续所有 task 都在这个 worktree 内执行。

---

## Task 1: 后端 Filter 结构 + Parse 多值（TDD）

**Files:**
- Modify: `internal/common/filter/parser.go`
- Test: `test/unit/filter/parser_test.go`

### 设计要点

`Filter.Value string` → `Values []string`，长度始终 ≥ 1。Parse 在切出 `value` 后：
- 若 `value` 整体被双引号包裹：strip 引号，作为单值
- 否则按 `|` 拆，逐项 trim，剔除空项，至少保留 1 项

### 步骤

- [ ] **Step 1：先扩展现有 `TestParse` 用例的期望（结构变更）**

打开 `test/unit/filter/parser_test.go`，把现有用例里所有 `Value: "x"` 替换为 `Values: []string{"x"}`，然后追加多值用例：

```go
// 现有用例改写示例
{name: "single filter", expr: "user:john",
    want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"john"}}}},

// 新增用例
{name: "multi value pipe", expr: "user:alice|bob",
    want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}}},
{name: "multi value with not equal", expr: "user:!alice|bob",
    want: []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Values: []string{"alice", "bob"}}}},
{name: "multi value with comparison should still parse",
    expr: "score:>3|5",
    want: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3", "5"}}}},
{name: "quoted value preserves pipe", expr: `user:"alice|bob"`,
    want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice|bob"}}}},
{name: "trims empty parts", expr: "user:alice||bob|",
    want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}}},
{name: "all empty parts errors out", expr: "user:|", wantErr: true},
{name: "score none-or-value mixed", expr: "score:none|3",
    want: []filter.Filter{{Field: "score", Operator: enum.OpEqual, Values: []string{"none", "3"}}}},
```

并把 `TestParse` 内 `for i, f := range got { if f != tt.want[i] { ... } }` 的相等判断替换为字段级断言（slice 不能用 `!=` 比较）：

```go
for i, f := range got {
    w := tt.want[i]
    if f.Field != w.Field || f.Operator != w.Operator || len(f.Values) != len(w.Values) {
        t.Errorf("Parse()[%d] = %+v, want %+v", i, f, w)
        continue
    }
    for j := range f.Values {
        if f.Values[j] != w.Values[j] {
            t.Errorf("Parse()[%d].Values[%d] = %q, want %q", i, j, f.Values[j], w.Values[j])
        }
    }
}
```

- [ ] **Step 2：跑测试，确认全部失败（结构尚未改）**

```bash
go test -count=1 -run TestParse ./test/unit/filter/...
```

预期：编译失败 `unknown field Values in struct literal of type filter.Filter`。

- [ ] **Step 3：修改 `internal/common/filter/parser.go` 的 `Filter` 结构**

把：
```go
type Filter struct {
	Field    string
	Operator enum.Operator
	Value    string
}
```
改为：
```go
type Filter struct {
	Field    string
	Operator enum.Operator
	Values   []string
}
```

- [ ] **Step 4：改写 `parsePart` 支持多值与引号保护**

把 `parsePart` 整个函数替换为（注意 `parsePart` 同文件内、保持相对位置）：

```go
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

			values, err := splitValues(rawValue)
			if err != nil {
				return Filter{}, err
			}
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
func splitValues(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return []string{raw[1 : len(raw)-1]}, nil
	}
	parts := strings.Split(raw, "|")
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 单段内若仍有引号，统一去掉
		p = strings.Trim(p, `"`)
		if p == "" {
			continue
		}
		values = append(values, p)
	}
	return values, nil
}
```

- [ ] **Step 5：跑测试，确认 `TestParse` 全绿**

```bash
go test -count=1 -run TestParse ./test/unit/filter/...
```

预期：`PASS`。`TestToSQL` 这步会因为 `Value`→`Values` 编译失败，下个 task 解决。

- [ ] **Step 6：Commit**

```bash
git add internal/common/filter/parser.go test/unit/filter/parser_test.go
git commit -m "refactor(filter): parse multi-value expression with pipe separator"
```

---

## Task 2: 后端 ToSQL 多值分支 + 常量（TDD）

**Files:**
- Modify: `internal/common/constant/filter.go`
- Modify: `internal/common/filter/parser.go`
- Test: `test/unit/filter/parser_test.go`

### 设计要点

ToSQL 矩阵：

| 配置 / 操作符 | len==1 | len>1 |
|---|---|---|
| Fuzzy + `:` | `col LIKE ?` | `(col LIKE ? OR col LIKE ? ...)` |
| Fuzzy + `:!` | `col NOT LIKE ?` | `(col NOT LIKE ? AND col NOT LIKE ? ...)` |
| Numeric + `:` | `col = ?` | `col IN (?)` (gorm 自动展开 `[]string`) |
| Numeric + `:!` | `col != ?` | `col NOT IN (?)` |
| ValueMap NULL + `:` | `col IS NULL` | `(col IS NULL OR col = ? ...)` 混合 |
| ValueMap NULL + `:!` | `col IS NOT NULL` | `(col IS NOT NULL AND col != ? ...)` 混合 |
| 比较操作 (>, <, >=, <=) | 同前 | **拒绝**：`ErrBadRequest` |

跨字段：依然 `AND` 拼接，不变。

### 步骤

- [ ] **Step 1：扩展 `TestToSQL` 用例**

在 `test/unit/filter/parser_test.go` 的 `TestToSQL` 函数 `tests` 切片末尾追加（现有用例的 `Value: "x"` → `Values: []string{"x"}`，已在 Task 1 改了）：

```go
// 多值用例
{
    name: "fuzzy multi value OR LIKE",
    filters: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}},
    wantSQL:  "(user_name LIKE ? OR user_name LIKE ?)",
    wantArgs: []any{"%alice%", "%bob%"},
},
{
    name: "fuzzy multi value AND NOT LIKE",
    filters: []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Values: []string{"alice", "bob"}}},
    wantSQL:  "(user_name NOT LIKE ? AND user_name NOT LIKE ?)",
    wantArgs: []any{"%alice%", "%bob%"},
},
{
    name: "numeric multi value IN",
    filters: []filter.Filter{{Field: "status", Operator: enum.OpEqual, Values: []string{"200", "500"}}},
    wantSQL:  "upstream_status_code IN (?)",
    wantArgs: []any{[]string{"200", "500"}},
},
{
    name: "numeric multi value NOT IN",
    filters: []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Values: []string{"200", "500"}}},
    wantSQL:  "upstream_status_code NOT IN (?)",
    wantArgs: []any{[]string{"200", "500"}},
},
{
    name: "valuemap none mixed with score",
    filters: []filter.Filter{{Field: "score", Operator: enum.OpEqual, Values: []string{"none", "3"}}},
    wantSQL:  "(score IS NULL OR score = ?)",
    wantArgs: []any{"3"},
},
{
    name: "valuemap none mixed with score not equal",
    filters: []filter.Filter{{Field: "score", Operator: enum.OpNotEqual, Values: []string{"none", "3"}}},
    wantSQL:  "(score IS NOT NULL AND score != ?)",
    wantArgs: []any{"3"},
},
{
    name: "multi value with comparison rejected",
    filters: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3", "5"}}},
    wantErr: true,
},
{
    name: "combined cross field AND",
    filters: []filter.Filter{
        {Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}},
        {Field: "status", Operator: enum.OpEqual, Values: []string{"200", "500"}},
    },
    wantSQL:  "(user_name LIKE ? OR user_name LIKE ?) AND upstream_status_code IN (?)",
    wantArgs: []any{"%alice%", "%bob%", []string{"200", "500"}},
},
```

`TestToSQL` 内的 args 比较：现有 `for i, arg := range gotArgs { if arg != tt.wantArgs[i] {` 在 `[]string` 这种切片 arg 上会因为 `!=` 不能比较 slice 而编译失败，需要替换为：

```go
for i, arg := range gotArgs {
    if !reflect.DeepEqual(arg, tt.wantArgs[i]) {
        t.Errorf("ToSQL() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
    }
}
```

并在文件 import 块加入 `"reflect"`。

- [ ] **Step 2：跑测试，确认编译失败 / 用例失败**

```bash
go test -count=1 -run TestToSQL ./test/unit/filter/...
```

预期：编译过但用例失败（旧 ToSQL 仍把多值当单值处理，所有新用例 FAIL）。

- [ ] **Step 3：在 `internal/common/constant/filter.go` 末尾新增常量**

```go
	// ── Multi-value SQL fragments ──
	FilterSQLIN    = " IN (?)"
	FilterSQLNOTIN = " NOT IN (?)"
	FilterSQLOR    = " OR "

	// ── Multi-value parser errors ──
	FilterErrMultiValueWithComparison = "multi-value not supported with comparison operator: %s"
)
```

注意：原文件最后一行是 `)` 闭合 const 块，把上面这段插到 `)` 之前。

- [ ] **Step 4：重写 `internal/common/filter/parser.go` 的 `buildCondition` 以分流多值**

把现有 `buildCondition`、`buildSimpleCondition` 整段替换为：

```go
// buildCondition 构建单个 SQL 条件（单值或多值）
func buildCondition(f Filter, config FieldConfig) (sql string, args []any, err error) {
	column := config.SQLColumn

	if !isMultiValueAllowed(f.Operator) && len(f.Values) > 1 {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrMultiValueWithComparison, f.Operator)
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
	parts := make([]string, 0, len(f.Values))
	args = make([]any, 0, len(f.Values))
	frag := constant.FilterSQLLIKE
	joiner := constant.FilterSQLOR
	if f.Operator == enum.OpNotEqual {
		frag = constant.FilterSQLNOTLIKE
		joiner = constant.FilterSQLAND
	}
	for _, v := range f.Values {
		parts = append(parts, column+frag)
		args = append(args, "%"+v+"%")
	}
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

// buildValueMapCondition 处理含 ValueMap 的字段，支持单值与多值（含 NULL 项混合）
func buildValueMapCondition(column string, f Filter, config FieldConfig) (sql string, args []any, err error) {
	type resolved struct {
		isNull bool
		value  string
	}
	resolvedList := make([]resolved, 0, len(f.Values))
	for _, raw := range f.Values {
		if mapped, ok := config.ValueMap[raw]; ok {
			if mapped == nil {
				resolvedList = append(resolvedList, resolved{isNull: true})
			} else {
				resolvedList = append(resolvedList, resolved{value: *mapped})
			}
		} else {
			resolvedList = append(resolvedList, resolved{value: raw})
		}
	}

	// 单值快速路径，与原行为完全等价
	if len(resolvedList) == 1 {
		r := resolvedList[0]
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

	// 多值：NULL 与非 NULL 混合
	parts := make([]string, 0, len(resolvedList))
	args = make([]any, 0, len(resolvedList))
	joiner := constant.FilterSQLOR
	if f.Operator == enum.OpNotEqual {
		joiner = constant.FilterSQLAND
	}
	for _, r := range resolvedList {
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
```

- [ ] **Step 5：跑全部 filter 测试**

```bash
go test -count=1 ./test/unit/filter/...
```

预期：`PASS`，所有用例通过。

- [ ] **Step 6：跑全量 lint + 全量 test，确保没有破坏既有调用方**

```bash
make lint
make test
```

预期：lint 通过；`./test/...` 全绿（确认 audit_repository、jwt_session_queries 这些 ToSQL 消费者没有触碰 `.Value`，因此不需要改）。

- [ ] **Step 7：Commit**

```bash
git add internal/common/constant/filter.go internal/common/filter/parser.go test/unit/filter/parser_test.go
git commit -m "feat(filter): support multi-value OR within field via pipe separator"
```

---

## Task 3: 前端新增 MultiSelectPill 组件

**Files:**
- Create: `web/src/components/ui/multi-select-pill.tsx`

### 步骤

- [ ] **Step 1：创建组件文件**

写入 `web/src/components/ui/multi-select-pill.tsx`：

```tsx
"use client";

import * as React from "react";
import { ChevronDown, Check } from "lucide-react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

export interface MultiSelectPillProps {
  label: string;
  options: string[];
  value: string[];
  onChange: (next: string[]) => void;
  searchable?: boolean;
  emptyText?: string;
  className?: string;
  /** display label for option value (default = value itself) */
  formatOption?: (value: string) => string;
}

export function MultiSelectPill({
  label,
  options,
  value,
  onChange,
  searchable,
  emptyText = "No options in current range",
  className,
  formatOption,
}: MultiSelectPillProps) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");

  const showSearch = searchable ?? options.length > 8;
  const active = value.length > 0;

  const filtered = React.useMemo(() => {
    if (!query) return options;
    const q = query.toLowerCase();
    return options.filter((o) => o.toLowerCase().includes(q));
  }, [options, query]);

  const toggle = (v: string) => {
    if (value.includes(v)) {
      onChange(value.filter((x) => x !== v));
    } else {
      onChange([...value, v]);
    }
  };

  const clearLocal = () => onChange([]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-sm transition-colors",
              active
                ? "border-foreground bg-foreground text-background hover:bg-foreground/90"
                : "border-input bg-transparent text-muted-foreground hover:text-foreground hover:border-ring",
              className,
            )}
          >
            <span>
              {label}
              {active ? ` · ${value.length}` : ""}
            </span>
            <ChevronDown className="size-3.5" />
          </button>
        }
      />
      <PopoverContent
        align="start"
        className="w-auto min-w-[200px] max-w-[280px] gap-1 p-1"
      >
        {showSearch && (
          <div className="px-1 pb-1">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter…"
              className="h-7 w-full rounded-md border border-input bg-transparent px-2 text-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
            />
          </div>
        )}

        <div className="max-h-64 overflow-y-auto">
          {filtered.length === 0 ? (
            <div className="py-4 text-center text-xs text-muted-foreground">
              {emptyText}
            </div>
          ) : (
            filtered.map((opt) => {
              const checked = value.includes(opt);
              return (
                <button
                  key={opt}
                  type="button"
                  onClick={() => toggle(opt)}
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                >
                  <span
                    aria-hidden
                    className={cn(
                      "flex size-4 shrink-0 items-center justify-center rounded border",
                      checked
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-muted-foreground/30",
                    )}
                  >
                    {checked && <Check className="size-3" />}
                  </span>
                  <span className="truncate">
                    {formatOption ? formatOption(opt) : opt}
                  </span>
                </button>
              );
            })
          )}
        </div>

        {active && (
          <>
            <div className="-mx-1 my-1 h-px bg-border" />
            <button
              type="button"
              onClick={clearLocal}
              className="w-full rounded-sm px-2 py-1.5 text-left text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              Clear selection
            </button>
          </>
        )}
      </PopoverContent>
    </Popover>
  );
}
```

- [ ] **Step 2：跑前端 lint，确认组件文件没有问题**

```bash
cd web && npm run lint
cd ..
```

预期：无新增报错。如果报 `react/no-unescaped-entities` 之类，修复后再继续。

- [ ] **Step 3：Commit**

```bash
git add web/src/components/ui/multi-select-pill.tsx
git commit -m "feat(web): add MultiSelectPill component"
```

---

## Task 4: 前端 Audit 页接入

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx`

### 步骤

- [ ] **Step 1：替换 import**

把 audit 页顶部的 `Select`-相关 import 块整段替换：

把
```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
```
替换为
```tsx
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
```

- [ ] **Step 2：State 类型变更**

把
```tsx
const [filterUser, setFilterUser] = useState<string>("");
const [filterModel, setFilterModel] = useState<string>("");
const [filterStatus, setFilterStatus] = useState<string>("");
```
替换为
```tsx
const [filterUser, setFilterUser] = useState<string[]>([]);
const [filterModel, setFilterModel] = useState<string[]>([]);
const [filterStatus, setFilterStatus] = useState<string[]>([]);
```

- [ ] **Step 3：重写 `buildAuditFilter`**

把
```tsx
function buildAuditFilter(user?: string, model?: string, status?: string): string | undefined {
  const parts: string[] = [];
  if (user) parts.push(`user:${user}`);
  if (model) parts.push(`model:${model}`);
  if (status) parts.push(`status:${status}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
}
```
替换为
```tsx
function buildAuditFilter(user: string[], model: string[], status: string[]): string | undefined {
  const parts: string[] = [];
  if (user.length) parts.push(`user:${user.join("|")}`);
  if (model.length) parts.push(`model:${model.join("|")}`);
  if (status.length) parts.push(`status:${status.join("|")}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
}
```

- [ ] **Step 4：修改 `fetchLogs` 形参类型**

把 `fetchLogs` 的 `user: string`、`model: string`、`status: string` 三个形参分别改为 `string[]`，里面 `buildAuditFilter(user || undefined, ...)` 替换为 `buildAuditFilter(user, model, status)`（不再有 `||` 兜底）。

- [ ] **Step 5：替换三处 `<Select>` 块**

把当前 audit 页过滤区里 3 个 `<Select>` 块（覆盖 User、Model、Status）整段替换为：

```tsx
<MultiSelectPill
  label="User"
  options={userOptions}
  value={filterUser}
  onChange={(v) => {
    setFilterUser(v);
    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, v, filterModel, filterStatus);
  }}
/>
<MultiSelectPill
  label="Model"
  options={modelOptions}
  value={filterModel}
  onChange={(v) => {
    setFilterModel(v);
    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, v, filterStatus);
  }}
/>
<MultiSelectPill
  label="Status"
  options={statusOptions}
  value={filterStatus}
  onChange={(v) => {
    setFilterStatus(v);
    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, filterModel, v);
  }}
/>
```

- [ ] **Step 6：修改顶部 `Clear filters` 判定**

把
```tsx
{(filterUser || filterModel || filterStatus) && (
  <Button
    variant="ghost"
    size="sm"
    className="gap-1 text-muted-foreground"
    onClick={() => {
      setFilterUser("");
      setFilterModel("");
      setFilterStatus("");
      fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, "", "", "");
    }}
  >
    <X className="size-3.5" />
    Clear filters
  </Button>
)}
```
替换为
```tsx
{(filterUser.length > 0 || filterModel.length > 0 || filterStatus.length > 0) && (
  <Button
    variant="ghost"
    size="sm"
    className="gap-1 text-muted-foreground"
    onClick={() => {
      setFilterUser([]);
      setFilterModel([]);
      setFilterStatus([]);
      fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, [], [], []);
    }}
  >
    <X className="size-3.5" />
    Clear filters
  </Button>
)}
```

- [ ] **Step 7：修改初始 useEffect 调用**

把
```tsx
fetchLogs(1, 20, "", "24h", "", "", "", "", "");
```
替换为
```tsx
fetchLogs(1, 20, "", "24h", "", "", [], [], []);
```

同时检查 `refresh()` 函数 `fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, filterModel, filterStatus);` —— 因为 `filterX` 现在已是 `string[]`，类型自然兼容，无需改动。

- [ ] **Step 8：跑前端 lint**

```bash
cd web && npm run lint
cd ..
```

预期：通过。

- [ ] **Step 9：Commit**

```bash
git add web/src/app/(dashboard)/audit/page.tsx
git commit -m "feat(web/audit): use multi-select pills for User/Model/Status filters"
```

---

## Task 5: 前端 Sessions 页接入

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

### 步骤

- [ ] **Step 1：清理失效 import + 加新 import**

当前文件第 6 行：
```tsx
import type { SessionSummary, PageInfo, OptionItem } from "@/lib/types";
```
替换为：
```tsx
import type { SessionSummary, PageInfo } from "@/lib/types";
```

把
```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
```
替换为
```tsx
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
```

- [ ] **Step 2：State 类型变更**

把
```tsx
const [filterScore, setFilterScore] = useState<string>("");
const [scoreOptions, setScoreOptions] = useState<OptionItem[]>([]);
```
替换为
```tsx
const [filterScore, setFilterScore] = useState<string[]>([]);
const [scoreOptions, setScoreOptions] = useState<string[]>([]);
```

- [ ] **Step 3：重写 `buildSessionFilter`**

把
```tsx
const buildSessionFilter = (score: string): string | undefined => {
  if (!score) return undefined;
  return `score:${score}`;
};
```
替换为
```tsx
const buildSessionFilter = (scores: string[]): string | undefined => {
  if (scores.length === 0) return undefined;
  return `score:${scores.join("|")}`;
};
```

- [ ] **Step 4：修改 `fetchSessions` 形参类型与 fetchScoreOptions 的赋值**

`fetchSessions` 中 `score: string` 形参改为 `score: string[]`；内部调用 `buildSessionFilter(score)` 不变。

`fetchScoreOptions` 内：
```tsx
if (!rsp.error && rsp.items) setScoreOptions(rsp.items);
```
保持不变（`rsp.items` 已是 `string[]`，与新 state 类型一致）。

- [ ] **Step 5：替换 Score `<Select>` 块**

把
```tsx
<Select
  value={filterScore || "__all__"}
  onValueChange={(v) => {
    const val = (v as string) === "__all__" ? "" : (v as string);
    setFilterScore(val);
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, val);
  }}
>
  <SelectTrigger className="w-[140px]">
    <SelectValue placeholder="Score" />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="__all__">All Scores</SelectItem>
    {scoreOptions.map((opt) => (
      <SelectItem key={opt} value={opt}>{opt}</SelectItem>
    ))}
  </SelectContent>
</Select>
```
替换为
```tsx
<MultiSelectPill
  label="Score"
  options={scoreOptions}
  value={filterScore}
  onChange={(v) => {
    setFilterScore(v);
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, v);
  }}
/>
```

- [ ] **Step 6：修改 Clear 按钮判定**

把
```tsx
{filterScore && (
  <Button
    variant="ghost"
    size="sm"
    className="gap-1 text-muted-foreground"
    onClick={() => {
      setFilterScore("");
      fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, "");
    }}
  >
    <X className="size-3.5" />
    Clear
  </Button>
)}
```
替换为
```tsx
{filterScore.length > 0 && (
  <Button
    variant="ghost"
    size="sm"
    className="gap-1 text-muted-foreground"
    onClick={() => {
      setFilterScore([]);
      fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, []);
    }}
  >
    <X className="size-3.5" />
    Clear
  </Button>
)}
```

- [ ] **Step 7：修改初始化 useEffect**

把
```tsx
fetchSessions(persistedPage, persistedPageSize, "30d", "", "", { field: "created_at", dir: "desc" }, "", "");
```
替换为
```tsx
fetchSessions(persistedPage, persistedPageSize, "30d", "", "", { field: "created_at", dir: "desc" }, "", []);
```

- [ ] **Step 8：跑前端 lint**

```bash
cd web && npm run lint
cd ..
```

预期：通过；同时之前 `OptionItem` 失效的 TypeScript 错误也一并清理。

- [ ] **Step 9：Commit**

```bash
git add web/src/app/(dashboard)/sessions/page.tsx
git commit -m "feat(web/sessions): use multi-select pill for Score filter"
```

---

## Task 6: 全量验证 + 手动 dev 验证

**Files:** none

### 步骤

- [ ] **Step 1：全量 lint + test**

```bash
make lint
make test
cd web && npm run lint && cd ..
```

预期：全部通过。

- [ ] **Step 2：本地启动后端**

```bash
go run main.go server start --host localhost --port 8080
```

另开终端：
```bash
cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev
```

浏览器访问 `http://localhost:3000/web`。

- [ ] **Step 3：Audit 页验证清单**

- [ ] User pill 默认 idle 样式（透明 + muted 文字）
- [ ] 点开 User pill，看到 checkbox 列表；选 1 个 → pill 变 active 样式（深底白字）+ 显示 `User · 1`
- [ ] 选第 2 个 → pill 显示 `User · 2`，列表能正常返回（OR 语义生效）
- [ ] 同时选 Status `200` 和 `500` → 列表能同时看到这两种 status 的记录
- [ ] 同时选 Model 两个不同模型 → 看到两个模型的并集记录
- [ ] Popover 内点 `Clear selection` → 当前维度回到 idle
- [ ] 顶部 `Clear filters` 按钮 → 三个 pill 同时回到 idle
- [ ] options.length > 8 时 popover 顶部出现 Filter 输入框，输入能过滤选项

- [ ] **Step 4：Sessions 页验证清单**

- [ ] Score pill 默认 idle
- [ ] 同时勾选 `Unscored` 和 `3` → 列表显示 未评分会话 + 3 分会话 的并集（NULL OR =3 SQL 生效）
- [ ] Clear 按钮回到 idle
- [ ] 评分、删除、批量删除、行点击跳详情 全部不受影响（确认未破坏）

- [ ] **Step 5：移动端尺寸验证（开发者工具 Responsive 模式或 ≤768px）**

- [ ] Pill 与 TimeRangePicker 自动断行不溢出
- [ ] Popover 在小屏不超出视口
- [ ] 评分按钮、删除等行级操作不受影响

- [ ] **Step 6：合并到 master（如用户确认）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
# 用户决定是 PR 还是直接 merge
```

⚠️ 此步骤需要用户明确指示后再执行，不在自动化范围。

---

## Self-Review

- ✅ Spec 覆盖：§1 Pill 视觉 → Task 3；§2 Popover 结构 → Task 3；§3 交互 → Task 3；§4 后端解析器扩展 → Task 1+2；§5 前端组件与调用方 → Task 3+4+5；§6 测试 → Task 1+2 的单元测试 + Task 6 手动；§7 文件清单 → 与 plan File Structure 完全一致；§8 风险与回滚 → 单值路径全部保持原 SQL，6 个 commit 易于 revert。
- ✅ 无 placeholder：每个 step 都有完整代码或具体命令。
- ✅ 类型一致：`Filter.Values []string` 在 Task 1/2 始终一致；`MultiSelectPillProps.value: string[]` 与 audit/sessions 调用方传值一致；`buildAuditFilter`/`buildSessionFilter` 形参类型与 page state 类型一致。
