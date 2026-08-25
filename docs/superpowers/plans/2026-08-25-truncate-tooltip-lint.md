# Web 端 lint 规则实现计划：截断文案必须配 Tooltip（truncate-requires-tooltip）

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.（本项目用户规则：不使用子 Agent，由当前会话顺序执行。）

**Goal:** 新增自定义 ESLint 规则 `truncate-requires-tooltip`（error 级），强制所有视觉截断文案（`truncate` / `line-clamp-1`）处于 `TooltipTrigger` 渲染子树内；同分支一次性修复全部存量违规，`npm run lint` 归零。

**Architecture:** ① 规则本体放 `web/eslint-rules/truncate-requires-tooltip.mjs`，对齐 `no-native-title.mjs` 先例（本地规则 + flat config 单规则插件注册）。② 检测 = JSX 元素 className 的字符串字面量片段（Literal / 模板 quasis / cn() 等表达式子树）token 精确命中 `truncate`|`line-clamp-1`；判定 = 沿 parent 链遇 `TooltipTrigger`|`TooltipContent` 通过、遇 `TooltipRoot`|`TooltipProvider` 报错、到顶报错。③ 存量按 lint 实际清单分组修复。设计依据：`docs/superpowers/specs/2026-08-25-truncate-tooltip-lint-design.md`。

**Tech Stack:** ESLint 9 flat config / RuleTester（挂 vitest）/ Next.js 16.2.6 / Tailwind v4 / shadcn `@/components/ui/tooltip`（`@base-ui/react`）。

## Global Constraints

- 仅改 `web/` 与 `docs/agents/web-frontend.md`；后端零改动；不新增 npm 依赖。
- 修复不动原 DOM 样式与结构：`TooltipTrigger` 用 `render` prop 包裹原截断元素，`TooltipContent` 放同内容（对齐 `models/page.tsx` 既有模式：`className="max-w-xs break-all"`）。
- 恒定短占位（`—`、`N/A` 等不会截断的静态串）**删 `truncate` 类**而非加 tooltip。
- 确实无法加 tooltip 的场景用 `eslint-disable-next-line truncate-requires-tooltip/truncate-requires-tooltip` + 原因注释（预期极少）。
- 每个修复文件需已 import Tooltip 组件（多数文件已有；没有的补 `TooltipRoot/TooltipTrigger/TooltipContent` 导入）。
- 验收：`cd web && npm run lint && npm run test && npm run build` 全绿。
- 所有中文回复，提交信息遵循仓库规范；提交前 `git branch --show-current` 核实分支。

---

### Task 1: 规则本体 + RuleTester 测试

**Files:**
- Create: `web/eslint-rules/truncate-requires-tooltip.mjs`
- Create: `web/eslint-rules/truncate-requires-tooltip.test.mjs`

**Interfaces:**
- Produces: `truncate-requires-tooltip` ESLint 规则（default export），messageId `needsTooltip`，数据 `{ tag, cls }`。

- [ ] **Step 1: 写规则本体**

```js
/**
 * 截断文案必须配 Tooltip。
 *
 * 背景：`truncate` / `line-clamp-1` 会让文本视觉截断，用户拿不到完整内容。
 * 项目契约（docs/agents/web-frontend.md）：截断文本用 Tooltip 组件展示完整内容：
 *   <TooltipRoot>
 *     <TooltipTrigger render={<span className="… truncate">…</span>} />
 *     <TooltipContent>完整内容</TooltipContent>
 *   </TooltipRoot>
 *
 * 检测：className 的字符串字面量片段（直接字符串、模板字符串 quasis、
 * cn()/三元/逻辑/数组表达式内的字面量）按空白切 token 精确命中
 * `truncate` 或 `line-clamp-1`。纯变量传递的类名静态不可见，放行
 * （宁漏报不误报）。line-clamp-2+（卡片描述）按设计豁免。
 *
 * 判定：沿 parent 链向上，遇 TooltipTrigger（渲染子树 = hover 热区）或
 * TooltipContent（嵌套 tooltip 不可行，边角豁免）视为达标；遇
 * TooltipRoot / TooltipProvider（在 Root 内但不在 Trigger 子树，hover
 * 不触发）或到 AST 顶则报错。中间的原生元素/其他组件正常穿越。
 */
const TRUNCATING_CLASSES = new Set(["truncate", "line-clamp-1"]);
const TOOLTIP_PASS_COMPONENTS = new Set(["TooltipTrigger", "TooltipContent"]);
const TOOLTIP_FAIL_COMPONENTS = new Set(["TooltipRoot", "TooltipProvider"]);

function jsxElementName(opening) {
  if (opening.name.type === "JSXIdentifier") return opening.name.name;
  return null; // JSXMemberExpression（Foo.Bar）不参与判定
}

function collectStringLiterals(node, out) {
  if (!node) return;
  switch (node.type) {
    case "Literal":
      if (typeof node.value === "string") out.push(node.value);
      return;
    case "TemplateLiteral":
      for (const quasi of node.quasis) out.push(quasi.value.raw);
      return;
    case "JSXExpressionContainer":
      collectStringLiterals(node.expression, out);
      return;
    case "ConditionalExpression":
      collectStringLiterals(node.consequent, out);
      collectStringLiterals(node.alternate, out);
      return;
    case "LogicalExpression":
      collectStringLiterals(node.left, out);
      collectStringLiterals(node.right, out);
      return;
    case "CallExpression":
      for (const arg of node.arguments) collectStringLiterals(arg, out);
      return;
    case "ArrayExpression":
      for (const el of node.elements) collectStringLiterals(el, out);
      return;
    default:
      return; // Identifier / MemberExpression / SpreadElement 等动态值：不可见
  }
}

function findTruncatingClass(classNameAttr) {
  const literals = [];
  collectStringLiterals(classNameAttr.value, literals);
  for (const literal of literals) {
    for (const token of literal.split(/\s+/)) {
      if (TRUNCATING_CLASSES.has(token)) return token;
    }
  }
  return null;
}

function coveredByTooltipTrigger(node) {
  for (let p = node.parent; p; p = p.parent) {
    if (p.type !== "JSXOpeningElement") continue;
    const name = jsxElementName(p);
    if (!name) continue;
    if (TOOLTIP_PASS_COMPONENTS.has(name)) return true;
    if (TOOLTIP_FAIL_COMPONENTS.has(name)) return false;
  }
  return false;
}

const truncateRequiresTooltip = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Require visually truncated text (truncate / line-clamp-1) to be hoverable via the Tooltip component.",
      recommended: true,
    },
    schema: [],
    messages: {
      needsTooltip:
        "`<{{tag}}>` uses `{{cls}}` which visually truncates text. Wrap it in a Tooltip: `<TooltipTrigger render={<.../>}>` + `<TooltipContent>` with the full text so users can hover to read it.",
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        const classNameAttr = node.attributes.find(
          (attr) =>
            attr.type === "JSXAttribute" &&
            attr.name.type === "JSXIdentifier" &&
            attr.name.name === "className",
        );
        if (!classNameAttr) return;
        const cls = findTruncatingClass(classNameAttr);
        if (!cls) return;
        if (coveredByTooltipTrigger(node)) return;
        context.report({
          node: classNameAttr,
          messageId: "needsTooltip",
          data: { tag: jsxElementName(node) ?? "unknown", cls },
        });
      },
    };
  },
};

export default truncateRequiresTooltip;
```

- [ ] **Step 2: 写 RuleTester 测试（挂 vitest）**

```js
import { describe, it } from "vitest";
import { RuleTester } from "eslint";
import rule from "./truncate-requires-tooltip.mjs";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

ruleTester.run("truncate-requires-tooltip", rule, {
  valid: [
    { code: "<span className='text-sm'>{x}</span>" }, // 无截断类
    { code: "<TooltipTrigger render={<span className='truncate'>{x}</span>} />" }, // 主流模式
    { code: "<TooltipTrigger><span className='truncate'>{x}</span></TooltipTrigger>" }, // children 模式
    { code: "<TooltipTrigger render={<div><span className='truncate'>{x}</span></div>} />" }, // 嵌套包装
    { code: "<TooltipContent><span className='truncate'>{x}</span></TooltipContent>" }, // tooltip 内容内豁免
    { code: "<p className='line-clamp-2'>{x}</p>" }, // 多行截断豁免
    { code: "<TooltipTrigger render={<span className={cn('line-clamp-1', cond && 'font-bold')}>{x}</span>} />" },
    { code: "<span className={`${dynamic} text-sm`}>{x}</span>" }, // 模板无截断字面量
    { code: "<span className={dynamic}>{x}</span>" }, // 纯变量放行
  ],
  invalid: [
    { code: "<span className='text-sm truncate'>{x}</span>", errors: [{ messageId: "needsTooltip" }] },
    { code: "<p className='line-clamp-1'>{x}</p>", errors: [{ messageId: "needsTooltip" }] },
    { code: "<TooltipRoot><div><span className='truncate'>{x}</span></div></TooltipRoot>", errors: [{ messageId: "needsTooltip" }] }, // Root 内非 Trigger 子树
    { code: "<TooltipProvider><span className='truncate'>{x}</span></TooltipProvider>", errors: [{ messageId: "needsTooltip" }] },
    { code: "<span className={cn('truncate', cond && 'font-bold')}>{x}</span>", errors: [{ messageId: "needsTooltip" }] }, // cn() 字面量命中
    { code: "<span className={`prefix truncate`}>{x}</span>", errors: [{ messageId: "needsTooltip" }] }, // 模板 quasis 命中
  ],
});
```

- [ ] **Step 3: 跑测试验证**

Run: `cd web && npx vitest run eslint-rules`
Expected: 全部用例通过。若失败按报错修正规则（先确认测试本身无误，再修规则——测试是行为的权威定义）。

- [ ] **Step 4: Commit**

```bash
rtk git add web/eslint-rules/truncate-requires-tooltip.mjs web/eslint-rules/truncate-requires-tooltip.test.mjs
rtk git commit -m "feat(web): 新增 truncate-requires-tooltip ESLint 规则与 RuleTester 测试"
```

### Task 2: 注册规则并产出存量违规清单

**Files:**
- Modify: `web/eslint.config.mjs`

- [ ] **Step 1: 注册规则**

在 `eslint.config.mjs` 顶部 import，并在 no-native-title 注册块后并列新增：

```js
import truncateRequiresTooltip from "./eslint-rules/truncate-requires-tooltip.mjs";
```

```js
  // 自定义规则：截断文案（truncate / line-clamp-1）必须处于 TooltipTrigger 渲染子树内，
  // 保证用户可悬停查看完整内容
  {
    plugins: {
      "truncate-requires-tooltip": {
        rules: { "truncate-requires-tooltip": truncateRequiresTooltip },
      },
    },
    rules: {
      "truncate-requires-tooltip/truncate-requires-tooltip": "error",
    },
  },
```

- [ ] **Step 2: 跑 lint 产出精确违规清单**

Run: `cd web && npx eslint . 2>&1 | tee /tmp/truncate-violations.txt`
Expected: 仅 `truncate-requires-tooltip` 违规（无其他规则新违规）。清单即存量修复的权威依据；据此把违规文件填入 Task 3 / Task 4 的实际文件列表（plan 预分组仅作参考，以清单为准；清单为空文件直接跳过）。

- [ ] **Step 3: Commit**

```bash
rtk git add web/eslint.config.mjs
rtk git commit -m "feat(web): 注册 truncate-requires-tooltip 规则为 error 级"
```

### Task 3: 存量修复——(dashboard) 页面组

**Files:**（以 Task 2 清单为准；预分组）
- Modify: `web/src/app/(dashboard)/models/page.tsx`、`apikeys/page.tsx`、`endpoints/page.tsx`、`cron/page.tsx`、`audit/{model,cron,demo}/page.tsx`、`sessions/page.tsx`、`shares/page.tsx`、`trace/page.tsx`、`dataset/page.tsx`、`trigger/page.tsx`、`layout.tsx`、`web/src/app/share/page.tsx`

**修复决策树（逐违规点执行）：**

1. 动态内容（modelId/alias/名称/URL 等）→ 标准模式：

```jsx
<TooltipRoot>
  <TooltipTrigger render={<span className="原类名不动">{原内容}</span>} />
  <TooltipContent className="max-w-xs break-all">{原内容}</TooltipContent>
</TooltipRoot>
```

2. i18n 静态文案（翻译后可能超长）→ 同上式加 tooltip。
3. 恒定短占位（`—`、`N/A`）→ 删 `truncate` 类（容器 `max-w-*` 可一并清理）。
4. 确实无法 tooltip（纯装饰/极罕见）→ `eslint-disable-next-line` + 原因注释。

注意：`TooltipRoot/TooltipTrigger/TooltipContent` 未 import 的文件补导入（对齐既有 `import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip"`）；`TooltipProvider` 已在全局 layout，无需每处包裹。

- [ ] **Step 1: 按 Task 2 清单逐文件修复（每个文件修完即时 `npx eslint <file>` 确认清零）**
- [ ] **Step 2: 组内全量验证**

Run: `cd web && npx eslint "src/app/**" && npx tsc --noEmit`
Expected: app 目录零违规、无类型错误。

- [ ] **Step 3: Commit**

```bash
rtk git add web/src/app
rtk git commit -m "fix(web): dashboard 页面截断文案补 Tooltip（truncate-requires-tooltip 存量修复）"
```

### Task 4: 存量修复——组件组

**Files:**（以 Task 2 清单为准；预分组）
- Modify: `web/src/components/session-detail/*`、`chat/*`、`trace-detail/*`、`filter-bar/filter-bar.tsx`、`export-claudecode-dialog.tsx`、`export-dialog-shared.tsx`、`demo-sessions-manager.tsx`、`trace-install-popover.tsx`、`ui/multi-select-pill.tsx`、`ui/time-range-picker.tsx`

同 Task 3 修复决策树。特别注意：`ui/` 下是可复用组件，若截断出现在组件内部且调用方无法控制（如 `multi-select-pill` 内部标签截断），评估该组件语义——内部确实截断就补 tooltip（组件内部自包含 TooltipRoot/Trigger/Content），而非 disable。

- [ ] **Step 1: 按清单逐文件修复 + 每文件 eslint 确认**
- [ ] **Step 2: 组内全量验证**

Run: `cd web && npx eslint "src/components/**" && npx tsc --noEmit`
Expected: components 目录零违规、无类型错误。

- [ ] **Step 3: Commit**

```bash
rtk git add web/src/components
rtk git commit -m "fix(web): 业务组件截断文案补 Tooltip（truncate-requires-tooltip 存量修复）"
```

### Task 5: 契约文档 + 全量验收

**Files:**
- Modify: `docs/agents/web-frontend.md`

- [ ] **Step 1: 更新 web-frontend.md**

在「Layout-Pattern Height Fix」表格单元格条款后补一条：

```md
- **截断与 Tooltip（lint 强制）**：`truncate` / `line-clamp-1` 元素必须处于
  `TooltipTrigger` 渲染子树内（`web/eslint-rules/truncate-requires-tooltip.mjs`，
  error 级）；`line-clamp-2+`（卡片描述）豁免。恒定短占位直接删截断类，不加 tooltip。
```

- [ ] **Step 2: 全量验收**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 三项全绿（lint 含新规则零违规、vitest 含新规则测试、build 静态导出成功）。

- [ ] **Step 3: 沉淀工程经验后提交**

按仓库规范用 update_memory 沉淀本次可复用经验，然后：

```bash
rtk git add docs/agents/web-frontend.md
rtk git commit -m "docs(web): 布局契约补充截断文案 Tooltip lint 强制条款"
```

- [ ] **Step 4: 汇报**

向用户汇报分支与提交清单，询问是否提 MR / 直接合并 master（禁止擅自操作）。
