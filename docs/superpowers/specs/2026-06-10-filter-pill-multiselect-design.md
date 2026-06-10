# Audit / Sessions 过滤器 Pill 多选化设计

## 背景

当前 `web/src/app/(dashboard)/audit/page.tsx` 与 `web/src/app/(dashboard)/sessions/page.tsx` 的过滤条件均使用 shadcn `Select` 单选下拉：

- Audit：`User`、`Model`、`Status`
- Sessions：`Score`

痛点：
- 每个维度只能选一个值（看不了"既看 200 又看 500"、"对比 sonnet 和 opus 两个模型"等典型场景）。
- 视觉上四个 `Select` 触发器并排，缺乏 claude.ai 这类现代后台常见的"已应用筛选有视觉重量、未应用安静"的状态区分。
- 后端 `internal/common/filter/parser.go` 的语法本身只支持单值 + AND，是上层多选的硬性瓶颈。

## 范围（Scope）

**重新设计**：
1. 上述 4 个 `Select` 控件 → 自定义"多选 Pill"组件
2. 后端 `internal/common/filter/parser.go` 扩展为支持同字段多值 OR

**保持原样**（Out of scope，明确不动）：
- 搜索输入框（`Search` Input）的样式与行为
- `TimeRangePicker` 的样式与行为
- 所有行级操作：评分流程、行级删除、批量删除、表头排序、整行点击跳转、行展开、TraceID 复制
- 分页 / page size 选择器
- 状态持久化策略（`usePersistentState` localStorage 现状不变，**不**引入 URL query string）
- 整体页面布局（Card、Header、Title 等）

## 设计

### 1. Pill 视觉规范

复用现有 shadcn `base-nova` + Tailwind v4 + `neutral` 色板，**不引入新色 token**。

**未选（idle）：**
```
h-8 rounded-full px-3 gap-1.5 text-sm
border border-input bg-transparent
text-muted-foreground hover:text-foreground hover:border-ring
```
内容：`{Label} <ChevronDown className="size-3.5" />`，例如 `User ▾`

**已选（active，count ≥ 1）：**
```
border-foreground bg-foreground text-background
hover:bg-foreground/90
```
内容：`{Label} · {count} <ChevronDown className="size-3.5" />`，例如 `Model · 2 ▾`

**布局**：与 `TimeRangePicker` 同行紧邻，共用 `flex flex-wrap gap-2`；移动端断行，不引入横向滚动。

### 2. Popover 内部结构

容器：
```
min-w-[200px] max-w-[280px] p-1
rounded-md border border-border bg-popover shadow-md
```

内部从上到下：
1. **可选搜索过滤行**（仅当 `options.length > 8` 时渲染）：轻量 input，用于过滤选项列表，**不发请求**。
2. **选项列表**：每行 `flex items-center gap-2 px-2 py-1.5 rounded-sm cursor-pointer hover:bg-accent`
   - 左侧 4×4 checkbox，复用 sessions 页表头复选框的视觉模板（`border-primary bg-primary` + `lucide` `Check`）。
   - 右侧 label，缺省 = value。
3. **空状态**：选项列表为空时居中显示 `No options in current range`（`text-muted-foreground text-xs py-4`）。
4. **底部 Clear 行**（仅当当前维度 value.length > 0 时渲染）：`border-t border-border mt-1 pt-1` 内一个 ghost 文字按钮 `Clear selection`。

### 3. 交互行为

- 点击 Pill：toggle Popover；外部点击 / Esc 关闭
- 点击选项整行或 checkbox：toggle 该值的选中状态，并**立即触发上层 `onChange`**（即立即重新发请求，与现有即时筛选语义一致）
- Popover 不在选中后自动关闭，便于连续多选
- 顶部既有的 `Clear filters` ghost button 行为保持：一键清空当前页**所有**维度
- 键盘可达：Pill 按钮支持 Tab / Enter / Space；Popover 内焦点行为复用 base-ui 默认（不额外定制方向键导航）

### 4. 后端 Filter 解析器扩展

文件：`internal/common/filter/parser.go`

**新语法（向后兼容）：**
- 单值：`user:alice` → 不变
- 同字段多值：`user:alice|bob|charlie` → 同字段 OR
- 跨字段多值：`user:alice|bob model:sonnet|opus status:200|500` → 同字段 OR、跨字段 AND
- 引号保护：`user:"a|b"` 保留为字面值 `a|b`（不拆分）
- 选 `|` 作为分隔符：`,` 在 model 名里偶有；`|` 在 user / model / status / score 字段值里不会出现

**Filter 结构变更：**
```go
type Filter struct {
    Field    string
    Operator enum.Operator
    Values   []string  // 替代原 Value string；len ≥ 1
}
```

**Parse 逻辑：**
- `parsePart` 拆出 `field`、`operator`、`rawValue` 后：
  - 若 `rawValue` 整体被双引号包裹：strip 引号，`Values = [rawValue]`（不按 `|` 拆）
  - 否则：按 `|` 切分，逐项 strip 空白；空项剔除；至少保留 1 项。

**ToSQL 逻辑：**

| 配置 / 操作符 | len(Values) == 1 | len(Values) > 1 |
|---|---|---|
| `IsFuzzy` + `OpEqual` | `col LIKE ?`（`%v%`） | `(col LIKE ? OR col LIKE ? ...)` |
| `IsFuzzy` + `OpNotEqual` | `col NOT LIKE ?` | `(col NOT LIKE ? AND col NOT LIKE ? ...)` |
| `IsNumeric` + `OpEqual` | `col = ?` | `col IN (?)`（gorm 自动展开） |
| `IsNumeric` + `OpNotEqual` | `col != ?` | `col NOT IN (?)` |
| `ValueMap` 命中 NULL + `OpEqual` | `col IS NULL` | `(col IS NULL OR col = ?...)` 混合形式 |
| `ValueMap` 命中 NULL + `OpNotEqual` | `col IS NOT NULL` | `(col IS NOT NULL AND col != ?...)` |
| 比较操作 `>` / `<` / `>=` / `<=` | 原有行为 | **拒绝**：返回 `ErrBadRequest`，错误信息 `multi-value not supported with comparison operator: %s` |

跨字段：依然 `AND` 拼接（不变）。

**新增/复用常量**（`internal/common/constant/filter.go`）：
```go
FilterSQLIN     = " IN (?)"
FilterSQLNOTIN  = " NOT IN (?)"
FilterSQLOR     = " OR "
FilterErrMultiValueWithComparison = "multi-value not supported with comparison operator: %s"
```

字段配置（`auditFieldConfigs` / `sessionFieldConfigs`）**不改**——多值能力是 parser/ToSQL 内部派生的。

### 5. 前端组件与调用方

**新增组件**：`web/src/components/ui/multi-select-pill.tsx`

```ts
export type MultiSelectPillOption = { value: string; label?: string };

export interface MultiSelectPillProps {
  label: string;
  options: MultiSelectPillOption[];
  value: string[];
  onChange: (next: string[]) => void;
  searchable?: boolean;       // 默认 options.length > 8
  emptyText?: string;         // 默认 "No options in current range"
  className?: string;
}
```

实现：
- 用 base-ui `Popover`（已被项目使用：`@base-ui/react`）
- 触发器为 `<button>`，根据 `value.length` 切换 idle/active className
- Popover 内部按 §2 拼装；checkbox 视觉样式直接内联（不抽 `<Checkbox>` 通用组件，YAGNI）
- 不引入新依赖

**Audit 页改动**（`web/src/app/(dashboard)/audit/page.tsx`）：

- State 类型变更：`filterUser/filterModel/filterStatus`：`string` → `string[]`
- `buildAuditFilter` 改为接受三组 `string[]`，按 `|` 拼接：
  ```ts
  function buildAuditFilter(user: string[], model: string[], status: string[]): string | undefined {
    const parts: string[] = [];
    if (user.length)   parts.push(`user:${user.join("|")}`);
    if (model.length)  parts.push(`model:${model.join("|")}`);
    if (status.length) parts.push(`status:${status.join("|")}`);
    return parts.length ? parts.join(" ") : undefined;
  }
  ```
- 三处 `<Select>` → `<MultiSelectPill label="User" options={...} value={filterUser} onChange={...} searchable />`
- `Clear filters` 按钮判定改为 `filterUser.length || filterModel.length || filterStatus.length`，行为同步清空
- 初始化 `useEffect` 与 `fetchLogs` 调用形参类型同步

**Sessions 页改动**（`web/src/app/(dashboard)/sessions/page.tsx`）：

- `filterScore: string` → `filterScore: string[]`
- `buildSessionFilter`：
  ```ts
  const buildSessionFilter = (scores: string[]) =>
    scores.length ? `score:${scores.join("|")}` : undefined;
  ```
- 单个 `<Select>` → `<MultiSelectPill label="Score" options={scoreOptions} value={filterScore} onChange={...} />`
- Clear 按钮判定：`filterScore.length > 0`
- `usePersistentState` 不变（仅类型从 `string` 改为 `string[]`）

**类型 / API 客户端**：无改动（filter 字段后端依然是 `string`）

### 6. 测试

**后端单元测试**：`test/unit/filter/parser_test.go`

现有用例迁移：把 `Value: "x"` 期望改为 `Values: []string{"x"}`。

新增用例：
- `Parse`：
  - `"user:alice|bob"` → `Values: ["alice","bob"]`
  - `"user:!alice|bob"` → 同上但 `Operator: OpNotEqual`
  - `"score:none|3"` → `Values: ["none","3"]`
  - `"user:\"alice|bob\""` → `Values: ["alice|bob"]`（引号保护）
  - `"user:alice|"` / `"user:|alice"` → `Values: ["alice"]`（剔除空项）
- `ToSQL`：
  - fuzzy 多值 `OpEqual` → `(col LIKE ? OR col LIKE ?)` + `["%alice%","%bob%"]`
  - fuzzy 多值 `OpNotEqual` → `(col NOT LIKE ? AND col NOT LIKE ?)`
  - numeric 多值 `OpEqual` → `col IN (?)` + `[]any{[]string{"200","500"}}`（gorm 展开方式）
  - ValueMap 中 `score:none|3` → `(score IS NULL OR score = ?)` + `["3"]`
  - 多值 + 比较操作（如 `score:>3|5`）→ 报错

**前端**：无强制测试（项目当前 web/ 无测试），手动在 dev 环境验证：
- 选 0 / 1 / N 个值的请求 payload 正确
- Active pill 视觉正确（idle vs active）
- Popover 内 Clear 与顶部 Clear 行为
- 切换 TimeRange 后选项列表刷新（保持现有 `fetchOptions` 逻辑）

### 7. 文件变更清单

**后端：**
- `internal/common/filter/parser.go`（修改：Filter 结构 + Parse + ToSQL 多值分支）
- `internal/common/constant/filter.go`（新增 IN/NOTIN/OR + 错误信息常量）

**前端：**
- `web/src/components/ui/multi-select-pill.tsx`（新增）
- `web/src/app/(dashboard)/audit/page.tsx`（修改）
- `web/src/app/(dashboard)/sessions/page.tsx`（修改）

**测试：**
- `test/unit/filter/parser_test.go`（修改 + 新增用例）

### 8. 风险与回滚

- **向后兼容**：单值表达式行为完全不变，旧的前端和 API 客户端仍能正常工作。任何回滚只需 revert 上述 6 个文件。
- **性能**：`IsNumeric` 多值用 `IN`，PG 索引仍可用；`IsFuzzy` 多值改为多个 `LIKE` OR，单页常规 2-3 个 LIKE，对 audit 表（已对 created_at 时间窗口预过滤）影响可忽略。
- **数据迁移**：无。
- **配置**：无。

## 验收标准

1. `make lint` 与 `make test` 通过。
2. `test/unit/filter/parser_test.go` 覆盖单值 + 多值 + 引号保护 + 多值 + 比较操作错误的全部用例。
3. dev 环境下 audit 页同时选两个 Status（200 和 500）能看到混合记录；同时选两个 Model 能看到 OR 后的并集。
4. dev 环境下 sessions 页同时勾选 `Unscored` 和 `3` 能看到未评分 + 3 分会话并集。
5. 桌面 / 移动两个断点下 Pill 视觉与交互均符合 §1–§3。
6. 顶部 `Clear filters` 与 Popover 内 Clear 行为正确互不冲突。

## 实施顺序建议

1. 后端 parser 改造（含测试）→ 单独可验证、可合入。
2. 前端组件 + 两个页面接入 → 依赖 1 部署后才能端到端验证。

适合拆成 2 个 commit / 2 段 plan，但放同一个分支同一个 PR。
