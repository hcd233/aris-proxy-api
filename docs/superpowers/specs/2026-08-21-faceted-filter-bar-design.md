# Faceted 筛选栏：列表筛选条件展示重设计

- 日期：2026-08-21
- 状态：已评审（交互范式经视觉伴侣三方案 demo 对比后选定）
- 范围：仅 `web/` 前端；后端零改动

## 1. 背景与痛点

当前 11 个 dashboard 列表页的筛选 UI 存在四个痛点（用户确认全部成立）：

1. **已选条件不可见**：`MultiSelectPill` 只显示「标签 · N」，用户必须逐个打开 popover 才能知道当前筛的是什么值。
2. **工具栏杂乱**：`sessions` 页工具栏有 6 个控件（TimeRangePicker + 3 pill + 搜索框 + 批删按钮），筛选维度增加会线性膨胀。
3. **视觉不统一**：pill 激活态（`bg-accent/60`）、TimeRangePicker 分段控件、`users` 页 shadcn `Select` 是三套视觉语言；`audit/model` 页甚至内联重复了搜索框 markup 而未复用 `SearchInput` 组件。
4. **代码重复难维护**：每个筛选控件的 `onChange` 都要把全部筛选状态按位置重传给 fetch 函数（`fetchSessions(1, pageSize, timeRange, cs, ce, sort, keyword, v, ...)` 达 10 个位置参数），新增一个筛选维度需改动 N 处调用点。

### 决策记录

通过视觉伴侣提供三个可交互 demo（同一 sessions 场景、同一筛选状态）对比后，**用户选定方案 C：Faceted 组合式搜索**——筛选条件以 token 形式与自由文本混排于一条输入框。

两个追加决策：

- **时间范围保持独立分段控件**：`TimeRangePicker` 不进入 token 体系，继续作为独立控件置于筛选栏旁。
- **引入 vitest**：为 DSL 纯函数模块建立单测（web 端此前无任何测试框架）。

落选方案存档：方案 A（Chips 已选条件行）、方案 B（统一筛选按钮+面板）的 demo 文件保留在 `.superpowers/brainstorm/30126-1787282154/content/`。

## 2. 核心发现：后端 DSL 已就绪

`internal/common/filter/parser.go` 实现了完整的 faceted filter DSL：

| 语法 | 语义 |
|---|---|
| `field:value` | 等于（`=`） |
| `field:!value` | 不等（`!=`） |
| `field:>v` / `<v` / `>=v` / `<=v` | 比较运算 |
| `field:a\|b\|c` | 多值 OR |
| `field:"hello world"` | 引号包裹含空格值（整体为单值字面量） |
| `field:min-max` | range 区间（`IsRange` 字段，BETWEEN） |
| 空格分隔多个 part | AND 连接 |

字段级能力由 `FieldConfig` 声明：`IsFuzzy` / `IsNumeric` / `IsJSONBArray` / `IsRange` / `ValueMap`（含 NULL 语义，如 sessions 的"未评分"映射）。

**结论：本次改造纯前端，后端 API、`filter` 参数格式、选项接口（`listSessionOptions` / `listAuditOptions`）全部不变。**

## 3. 架构设计

### 3.1 新增模块 `web/src/components/filter-bar/`

| 文件 | 职责 |
|---|---|
| `filter-dsl.ts` | 纯函数层。`serializeTokens(tokens) → filter string`；`parseFilterString(str, facets) → tokens`。与后端 DSL 严格对齐（引号、`\|` 多值、`min-max` range）。不含 React 依赖，是 vitest 的测试对象。 |
| `types.ts` | `FacetDef`（声明式 facet 配置）、`FilterToken`、`FilterBarState` 类型。 |
| `use-filter-bar.ts` | 状态 hook。token 数组状态、`usePersistentState` 持久化、旧 localStorage key 迁移、变更时向上抛出结构化 `queryParams`。 |
| `filter-bar.tsx` | 主组件。token 渲染、自由文本 input、建议下拉（facet key 建议 → facet 值建议两级）、键盘导航、清除全部。 |

### 3.2 声明式 facet 配置（消灭位置参数）

```ts
// types.ts
interface FacetDef {
  /** 后端 DSL 字段名，如 "score" / "model" / "messageCount" */
  key: string;
  /** i18n 后的展示标签，如 "评分" */
  label: string;
  /** enum（默认）：从选项列表选值；range：min-max 区间预设（如消息数） */
  kind?: "enum" | "range";
  /** 序列化目标：filter DSL（默认）或独立 query 参数（如 users 页 permission） */
  target?: "filter" | "param";
  /** 静态选项或异步加载（异步时按当前时间范围调用现有 options 接口） */
  options?: string[] | (() => Promise<string[]>);
  /** 选项值的展示格式化（如 score "5" → "★5"） */
  formatValue?: (value: string) => string;
}

interface FilterToken {
  /** facet key；自由文本 token 为 null */
  key: string | null;
  value: string;
}
```

页面用法（以 sessions 为例）：

```ts
const { barProps, queryParams, activeCount } = useFilterBar({
  persistKey: "dashboard.sessions",
  facets: [
    { key: "score", label: t("sessions.filter_score"), options: scoreOptionsLoader, formatValue: (v) => `★${v}` },
    { key: "model", label: t("sessions.filter_model"), options: modelOptionsLoader },
    { key: "messageCount", label: t("sessions.filter_message_count"), kind: "range", options: ["1-10", "11-50", "51-"] },
  ],
  freeText: { placeholder: t("sessions.search_placeholder") },
});
// queryParams = { filter?: string; keyword?: string }
// fetch 收敛为 fetchSessions({ page, pageSize, sort, timeRange..., ...queryParams })
```

`useFilterBar` 在 token 变化时以**对象参数**通知页面 refetch，页面 fetch 函数签名从 10 个位置参数收敛为单个 params 对象。

### 3.3 token ↔ DSL 映射

- 每个 facet 值一个 token：`评分 ★5`、`模型 claude-sonnet-4.5`。
- 同 key 多 token 序列化时合并为 `key:v1|v2`；不同 key 空格连接。
- 自由文本 token（`关键词 "退款"`）**不进入 filter DSL**，映射到现有 `keyword`（sessions/trace 等）或 `query`（audit 页）参数。
- `target: "param"` 的 facet（users 页 `permission`）序列化为独立 query 参数，不进 DSL。
- `kind: "range"` 的值原样传递（后端 `IsRange` 解析 `min-max`）；开放式区间（如 `51-`）的表示以对齐后端 `parseRangeValue` 为准——若不支持开放式则 v1 只提供闭区间预设，**不引入新的 range 语法**。
- UI 只生成 `=` 语义；`!=` / 比较运算符不进 UI，但 `parseFilterString` 须能容忍（持久化数据向前兼容）。

### 3.4 工具栏布局

```
┌──────────────────────────────────────────────────────────────┐
│ [TimeRangePicker(分段)] [FilterBar ████████████████ flex-1] [批删等操作] │
└──────────────────────────────────────────────────────────────┘
```

- 有时间范围的页面（sessions、audit/model、audit/cron）：TimeRangePicker 在左，FilterBar 占满剩余宽度。
- 无时间范围的页面：FilterBar 独占一行。
- 移动端：自动换行为两行（TimeRangePicker 一行、FilterBar 一行）。
- FilterBar 内部：token 区 + input 自适应换行（`flex-wrap`），最小高度 40px，focus 时 `border-ring` + `ring-2 ring-ring/30`（沿用现有 focus 语言）。
- 清除全部：bar 内有 token 时在右侧显示 `✕ 清除`；结果行「N 条 · 已应用 M 项条件」置于表格上方（复用现有 `text-xs text-muted-foreground` 规格）。

### 3.5 交互契约

键盘：
- 输入字符 → 建议列表第一级：匹配的 facet key（i18n label 与英文 key 均参与匹配）。
- `key:`（含中文冒号 `：`）→ 第二级：该 facet 的值建议（异步选项带加载态；输入继续过滤）。
- `Enter` / `Tab` / 点击 → 生成 token 并清空 input；`↑` `↓` 导航建议；`Esc` 收起建议。
- input 为空时 `Backspace` → 删除最后一个 token。
- 输入不匹配任何 facet key 时按 `Enter` → 生成自由文本（关键词）token，并立即触发查询（对齐现有回车搜索行为）。

鼠标：
- 点 facet key 建议 → input 变为 `key:` 草稿态并展开值建议。
- 点值建议 → 生成 token。
- 点 token 的 `×` → 删除该 token。
- 点清除全部 → 清空所有 token（不动 TimeRangePicker）并触发查询。

选项加载：
- 展开值建议时按需调用现有 options 接口（sessions 按当前 `startTime/endTime`，与现状一致）；结果在 bar 会话内缓存，时间范围变化后失效重取。
- `enum` facet 值建议支持输入过滤（复用现有 `MultiSelectPill` 的 >8 选项出现搜索框的经验阈值：>8 时显示过滤框）。

明确不做（YAGNI）：
- token 间左右光标移动、拖拽排序（后续增强候选）。
- `!=` / 比较运算符的 UI 入口。
- 筛选条件的保存/命名视图（saved views）。
- 时间范围进入 token 体系（用户决策：保持独立分段控件）。

### 3.6 状态与持久化

- token 数组 JSON 序列化后经 `usePersistentState` 存 localStorage，新 key：`dashboard.<page>.filters`。
- 迁移：首次挂载时读取旧 key（如 `dashboard.sessions.filterScore` / `filterModel` / `filterMessageCount` / `keyword`）转换为 token 后写入新 key，并删除旧 key。
- `audit/cron`、`audit/model` 当前未持久化筛选（`useState`），迁移后统一获得持久化行为（与 sessions 对齐，属行为改进）。
- 自由文本 token 沿用现有语义：input 草稿（`searchInput`）与已生效关键词（`keyword`）分离；token 只表示已生效值。

### 3.7 视觉规范

- token：`h-7 rounded-lg border border-ring/40 bg-accent/60`，key 段 `text-muted-foreground`，value 段 `font-medium`，`×` 按钮 hover `text-destructive`；入场 `animate-scale-in`（沿用 Silk Paper tokens，受全局 reduced-motion 守卫约束）。
- 自由文本 token：虚线描边（`border-dashed border-input`）区分于 facet token。
- 建议下拉：`PopoverContent` 风格（`rounded-lg border bg-popover shadow-sm`），分组标题 `text-[0.65rem] uppercase tracking-wider text-muted-foreground`。
- 整体沿用 anthropic 主题 token，不引入新色值。

## 4. 页面迁移矩阵

| 页面 | facets | freeText | TimeRangePicker | 备注 |
|---|---|---|---|---|
| `sessions` | `score`、`model`、`messageCount`(range) | `keyword` | 保留 | 选项接口 `listSessionOptions` |
| `audit/model` | `user`、`model`、`status`、`ua` | `query` | 保留 | 顺带收编内联搜索框 markup |
| `audit/cron` | `type`、`status` | `query` | 保留 | 现状未持久化，迁移后获得持久化 |
| `users` | `permission`（`target:"param"`，静态 4 选项） | `keyword` | 无 | 退役 shadcn `Select` |
| `trace` | — | `keyword` | 无 | 退化为增强搜索框 |
| `cron` | — | `keyword` | 无 | 同上 |
| `models` | — | `keyword` | 无 | 同上 |
| `apikeys` | — | `keyword` | 无 | 同上 |
| `endpoints` | — | `keyword` | 无 | 同上 |
| `trigger` | — | `keyword` | 无 | 同上 |

**排除**：
- `dataset`：其"筛选"是导出配置语义（minScore 滑块、导出参数、StatPill 展示），不是列表筛选，保持现状。
- `shares`：无任何筛选控件，无需改动。
- `monitor`、dashboard 首页、`profile`：非列表页。`TimeRangePicker` 在 monitor 等图表页继续服役，不退役组件本身。

退役清单：`MultiSelectPill` 组件、各页零散的 `buildXxxFilter` 函数、`users` 页 `Select` 筛选、`audit/model` 内联搜索框 markup。

## 5. i18n

新增 key（zh/en/ja 三语）：

```
filter_bar.suggest_facets      // 建议列表第一级分组标题："筛选维度"
filter_bar.suggest_values      // 第二级："选择「{facet}」的值"
filter_bar.keyword_hint        // 自由文本建议："关键词："xxx""
filter_bar.clear_all           // "清除全部"
filter_bar.applied_count       // "已应用 {count} 项条件"
filter_bar.loading_options     // "加载选项…"
filter_bar.no_options          // "无匹配选项"
```

facet label 复用现有 `sessions.filter_score`、`audit.filter_user` 等 key，不重复定义。

## 6. 测试计划

### 6.1 vitest 单测（新增）

- 最小接入：`vitest` + `npm run test` script，仅覆盖纯函数模块 `filter-dsl.ts`（不引 React 测试库）。
- 用例（约 20 个）：
  - serialize：单 token、同 key 多 token 合并 `|`、多 key 空格连接、含空格值加引号、range 值原样、自由文本/`param` token 不进 DSL。
  - parse：`field:v`、`field:a|b`、引号值、未知 key 容错（丢弃并保留其余）、非法片段容错。
  - round-trip：`parse(serialize(tokens)) ≡ tokens`（合法输入集）。
  - 迁移函数：旧 localStorage key 组合 → tokens 的转换。

### 6.2 手工验证清单

- 11 个目标页面逐页：筛选生效（网络请求参数正确）、token 增删、清除全部、刷新后持久化恢复、旧 key 迁移一次后不再出现。
- 键盘全流程：key 建议 → `:` → 值建议 → Enter → Backspace 删除。
- 移动端窄屏换行。
- 三语切换（zh/en/ja）facet label 与提示语。
- `npm run lint`、`next build` 通过。

## 7. 实施顺序（供 writing-plans 参考）

1. vitest 接入 + `filter-dsl.ts` + 单测（纯函数先行，TDD）。
2. `types.ts` + `use-filter-bar.ts` + `filter-bar.tsx` 组件。
3. `sessions` 页迁移（最复杂页，验证范式）+ 旧 key 迁移。
4. `audit/model`、`audit/cron` 迁移。
5. `users` + 6 个 freeText-only 页迁移。
6. 退役 `MultiSelectPill` 与散落 filter 构造函数、i18n 补全、手工清单过检。

## 8. 风险

| 风险 | 缓解 |
|---|---|
| `filter-dsl.ts` parse/serialize 边界错误（引号、`\|`、range） | vitest 单测 + round-trip 用例先行 |
| 旧 localStorage 数据导致恢复异常 | 迁移代码容错（try/catch + 丢弃非法值）；迁移失败退化为空筛选 |
| 异步选项在时间范围切换后陈旧 | 选项缓存以 `startTime+endTime` 为 key，变更即失效 |
| 11 页迁移回归面广 | sessions 先行验证范式；每页手工清单；lint + build 卡口 |
