# Web 端截断文案必须配 Tooltip 的 lint 规则设计

- 日期：2026-08-25
- 状态：已确认
- 范围：`web/` 前端工程新增自定义 ESLint 规则 `truncate-requires-tooltip` + 全量存量修复 + 契约文档更新；后端零改动

## 背景与目标

前端大量使用 Tailwind `truncate`（单行省略号）压缩表格单元格、列表项等长文本。视觉截断后用户无法获取完整内容，而项目已有 Tooltip 组件与「表格单行长内容单元格用 `max-w-[Nch] truncate` + Tooltip 展示完整内容」的既有契约（`docs/agents/web-frontend.md`），但纯靠人工遵守，新增代码不断出现裸 truncate。

目标：用 lint 规则在 CI 卡点强制「任何视觉截断的文案必须可通过悬停看到完整内容」，对齐既有先例 `web/eslint-rules/no-native-title.mjs`（禁止原生 `title`、强制 Tooltip 组件，error 级 + 零违规入库）。

现状盘点：`truncate` 分布于 27 个文件约 60 处（约半数已有 Tooltip 包裹）；`line-clamp-*` 仅 1 处（`session-history-list.tsx`，均为 `line-clamp-1`）。

## 需求决策（已与用户确认）

| 决策点 | 结论 | 备选（否决原因） |
|---|---|---|
| 检测范围 | `truncate` + `line-clamp-1` | 所有 `line-clamp-*`（卡片描述 line-clamp-2 内容长、tooltip 展示价值低，豁免）；仅 `truncate`（漏掉语义等同的 line-clamp-1） |
| 「已有 tooltip」判定 | 截断元素处于 `TooltipTrigger` 渲染子树内（含 render prop 与 children，允许中间嵌套包装元素） | Root 内任意层级（tooltip 可能挂在兄弟元素上，hover 不触发，漏报）；必须直接是 trigger 本身（嵌套包装的合理写法会被误报） |
| 严重级别与存量 | error + 同分支一次性修完全部存量 | warn 先行（易被忽视、长期不收敛）；error + 逐处 disable（注释噪音大） |

## 规则设计

### 文件与注册

- 新增 `web/eslint-rules/truncate-requires-tooltip.mjs`，结构对齐 `no-native-title.mjs`：`meta`（type `problem`、单 message、空 schema）+ `create` 访问器。
- `web/eslint.config.mjs` 新增注册块（与 no-native-title 并列），规则 ID `truncate-requires-tooltip/truncate-requires-tooltip`，级别 `error`。

### 检测逻辑（哪些元素命中）

访问 `JSXOpeningElement`，取其 `className` 属性值中**所有字符串字面量片段**：

1. `className="foo truncate"`——`Literal` 直接字符串；
2. `className={cn("truncate", cond && "x")}` / 三元 / 数组——遍历表达式子树收集全部字符串 `Literal`；
3. 模板字符串——收集所有 `TemplateLiteral.quasis` 静态段。

按空白切 token 后**精确匹配** `truncate` 或 `line-clamp-1`：

- `line-clamp-2/3+` 不命中（决策豁免）；
- `truncateText()`（JS 层截断，仅 1 处调用）、纯变量传递的类名（静态不可见）不在范围——宁漏报不误报；
- 条件性出现（`cn(cond && "truncate")`）命中字面量即报（该场景真实存在截断可能）。

### 判定逻辑（是否已有 tooltip）

命中元素沿 `node.parent` 链向上遍历，遇以下 `JSXOpeningElement` 组件名即停：

| 遇到 | 结果 | 理由 |
|---|---|---|
| `TooltipTrigger` | 通过 | trigger 渲染子树 = hover 热区，悬停截断文本必然弹 tip（base-ui Trigger 事件挂在 render 元素上，子树内 hover 均触发） |
| `TooltipContent` | 通过 | tooltip 内容内的截断属边角场景（嵌套 tooltip 技术上不可行），豁免 |
| `TooltipRoot` / `TooltipProvider` | 报错 | 在 Root 内但不在 Trigger 子树，hover 截断文本不触发；Provider 只提供主题上下文（layout 层全局包裹） |
| 原生元素 / 其他组件 | 继续向上 | 支持 `<TooltipTrigger render={<div><span className="truncate"/>…</div>}>` 嵌套包装 |
| 走到 AST 顶 | 报错 | 无任何 tooltip 覆盖 |

跨组件边界不可见（如 `<Button>` 组件内部的 truncate 在另一文件）——lint 单文件静态分析的固有限制，可接受。

### 错误消息

对齐 no-native-title 的英文消息风格：

> `` `<{{tag}}>` uses `{{cls}}` which visually truncates text. Wrap it in a Tooltip: `<TooltipTrigger render={<…/>}>` + `<TooltipContent>` with the full text so users can hover to read it. ``

## 测试策略

`web/eslint-rules/truncate-requires-tooltip.test.mjs`：ESLint 9 `RuleTester`（`parserOptions: { ecmaFeatures: { jsx: true } }`），覆写 `RuleTester.describe` / `RuleTester.it` 挂到 vitest（对齐 `filter-dsl` 的 vitest 先例）。约 12 个用例：

- **valid**：TooltipTrigger render 包裹（字符串 className）；Trigger children 包裹；嵌套包装（render 内多层原生元素）；TooltipContent 内；无截断类；`line-clamp-2`；`cn()` 内 `line-clamp-1` + Trigger 包裹。
- **invalid**：裸 `truncate`；裸 `line-clamp-1`；`TooltipRoot` 内但不在 Trigger 子树；`cn()` 动态类名字面量含 `truncate` 且无包裹；模板字符串 quasis 含 `truncate`。

## 存量修复（同分支一次修完，`npm run lint` 归零）

标准修复模式（对齐 `models/page.tsx` 既有写法）：

```jsx
// 前
<span className="block max-w-[12ch] truncate">{model.modelId}</span>
// 后
<TooltipRoot>
  <TooltipTrigger render={<span className="block max-w-[12ch] truncate">{model.modelId}</span>} />
  <TooltipContent className="max-w-xs break-all">{model.modelId}</TooltipContent>
</TooltipRoot>
```

分类处理：

- **动态内容**（modelId、alias、endpoint 等）：照上式加 tooltip，content 与截断文本相同。
- **永不截断的静态占位**（如 `<span className="… truncate">—</span>`）：直接删 `truncate` 类（比加 tooltip 更正确——内容恒定无截断可能）。
- **i18n 静态 hint**（翻译后 en/zh/ja 长度不一，可能超长）：照常加 tooltip。
- 修复逐文件进行，每文件修完过一遍 lint；无法自动判断的个别场景用 `eslint-disable-next-line truncate-requires-tooltip/truncate-requires-tooltip -- 说明原因` 手动豁免（预期极少）。

## 文档更新

`docs/agents/web-frontend.md` 布局契约段（表格单元格条款旁）补充：lint 已强制——`truncate` / `line-clamp-1` 元素必须处于 `TooltipTrigger` 渲染子树内，否则 `npm run lint` 报错。

## 分支与交付

- worktree：`.worktrees/truncate-tooltip-lint`
- 分支：`feature/truncate-tooltip-lint-2026-08-25`
- 提交内容：规则 + 注册 + RuleTester 测试 + 全量存量修复 + 契约文档
- 验收：`cd web && npm run lint && npm run test && npm run build` 全绿；合并 `master` 后 CI（docker-publish.yml 含 `web/**` path filter）自动发布

## 否决的备选实现

- **dev 运行时检测**（DOM `scrollWidth > clientWidth` + console.warn）：只报真实截断零误报，但不是 lint 规则、无法 CI 卡点、只覆盖被访问页面——与诉求不符。
- **强制 `<TruncatedText>` 包装组件 + 禁裸类**：60 处存量全部重写、失去 `max-w-[Nch]` 各异的 Tailwind 组合灵活性、与既有 `TooltipTrigger render` 惯例双轨并存——侵入性过大。
