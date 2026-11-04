# AI Elements 对话 UI 接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 会话详情、分享页、Playground 的对话步骤统一到 shadcn AI Elements（Trace / Reasoning / Tool / Chain of Thought / Code Block+shiki / MessageScroller / Context / Model Selector）。

**Architecture:** 新增 3 个 ui 原语（trace、code-block、hover-card）+ shiki 高亮器；`components/chat/` 新增 reasoning、tool-calls、transcript-groups、highlighted-code 四个模式组件，详情/分享页共用新的 `session-transcript.tsx`；Playground 的 composer 与 chat-turn 换 Model Selector 与 Context。纯逻辑全部抽纯函数用 vitest 覆盖。

**Tech Stack:** Next.js 16 静态导出、React 19、Tailwind v4、Base UI、shadcn 4.8.0（CI 锁定）、shiki 4.6.0、vitest。

**Spec:** `docs/superpowers/specs/2026-11-04-ai-elements-chat-design.md`

## Global Constraints

- 禁止直接 `shadcn add`；组件源码已导出在 `/tmp/aiel/{trace,code-block,hover-card}.src.tsx`（不存在则 `./node_modules/.bin/shadcn view <name>` 重新取），`use-highlighted-code` 的两个文件用 `shadcn view use-highlighted-code` 取。
- 落盘改写：`from "cn"` → `@/lib/utils`；`@/registry/base-nova/ui/*` → `@/components/ui/*`；`@/registry/base-nova/lib/highlighter` → `@/lib/highlighter`；`IconPlaceholder` → lucide 具体图标（ChevronRight / Check / Copy）。
- 删除 tw-animate 类（`animate-in|animate-out|fade-in-0|fade-out-0|zoom-in-95|zoom-out-95|slide-in-from-*|slide-out-to-*`）与 CI 版本不存在的 `scroll-fade-*`、`scrollbar-*`；`shimmer` / `shimmer-none` 保留（Task 1 自定义）。
- 主题变体：`dark:` 一律改 `moonshot:`（项目无 `.dark`）。
- 依赖：`npm install --package-lock-only --save-exact shiki@4.6.0`；删依赖 `npm uninstall --package-lock-only highlight.js rehype-highlight`；改 lock 后 `npx npm@10 ci --dry-run` exit=0。共享 node_modules 安装：`npm pack shiki@4.6.0` 及其缺失依赖解包进主工作区 `web/node_modules`（先 `ls` 确认缺哪些）。
- i18n 三份同步：`web/src/locales/{zh,en,ja}.json`。
- 每个 Task 结束：`npm run lint && npm run typecheck && npm run test && npm run format:check` 全绿再提交。
- 工作目录：`.worktrees/ai-elements-chat`，分支 `feature/ai-elements-chat-2026-11-04`；`web/node_modules` 软链主工作区。

## Review Focus

1. **工具结果消息夹在工具调用之间**（真实 agent 会话的常态：assistant(tool_calls) → tool → assistant(tool_calls) → tool）——分组必须跳过 tool 结果而不是被它打断。→ Task 5 测试 `groupTranscript` 用例 B。
2. **超大工具输出 / 代码块**（单段 >200KB）——shiki 同步分词会卡死主线程，应退化为纯文本行。→ Task 2 测试 `shouldHighlight`。
3. **未知语言围栏或空语言**（```foo、```）——不报错、按纯文本渲染。→ Task 2 测试 `resolveLanguage`。
4. **localStorage 里残留非法 `reasoningEffort`**（旧版本或手改）——回落默认 `""`，不把非法值发给上游。→ Task 6 测试 `loadParams`。
5. **流被中断时没有 usage**——Context 触发器显示 `—` 而非 `NaN%`。→ Task 7 测试 `contextSummary`。

---

### Task 0: Worktree 与依赖

**Files:** 无代码改动

- [ ] **Step 1:** `git worktree add -b feature/ai-elements-chat-2026-11-04 .worktrees/ai-elements-chat master`；`ln -s ../../../web/node_modules .worktrees/ai-elements-chat/web/node_modules`；`git -C .worktrees/ai-elements-chat log --oneline -1` 确认与 master 同 HEAD。
- [ ] **Step 2:** 把 spec 与本 plan 复制进 worktree（主工作区里是未提交文件），之后的改动都在 worktree 内。
- [ ] **Step 3:** 在 worktree `web/` 下 `npm install --package-lock-only --save-exact shiki@4.6.0`；按 lock 新增条目把 shiki 及其子包解包进主工作区 `web/node_modules`；`node -e "require.resolve('shiki/core')"` 成功。
- [ ] **Step 4:** `npx npm@10 ci --dry-run` exit=0；提交 `chore(web): 引入 shiki 4.6.0` 连同 spec 与 plan。

### Task 1: Trace 与 HoverCard 原语 + shimmer

**Files:**
- Create: `web/src/components/ui/trace.tsx`、`web/src/components/ui/hover-card.tsx`
- Modify: `web/src/app/globals.css`（尾部加 utility）、`web/src/styles/motion-{base,anthropic,moonshot}.css`（`popover-content` 出现处并列加 `[data-slot="hover-card-content"]`）
- Test: `web/src/components/shadcn-native.test.ts`

**Interfaces:**
- Produces: `Trace, TraceLine({status?: "running"|"error"|"pending"}), TraceIcon, TraceTitle, TraceDescription, TraceCollapsible, TraceCollapsibleTrigger, TraceCollapsibleContent`（签名同官方）；`HoverCard, HoverCardTrigger, HoverCardContent`；CSS utility `shimmer`、`shimmer-none`。

- [ ] **Step 1: 写失败测试** 在 shadcn-native.test.ts 新增 `describe("⑦ AI Elements")`：`expectGenerated(["trace", "hover-card"])`；断言 `globals.css` 含 `@utility shimmer {` 与 `@utility shimmer-none {`；断言 `motion-base.css` 含 `hover-card-content`。
- [ ] **Step 2:** `npx vitest run src/components/shadcn-native.test.ts` → FAIL（文件不存在）。
- [ ] **Step 3: 落盘 trace.tsx** 按全局约束改写；`TraceTitle` 的 `truncate` 上方加 `// eslint-disable-next-line truncate-requires-tooltip/truncate-requires-tooltip -- 调用方负责包 Tooltip`（先例：`ui/attachment.tsx:98`）。
- [ ] **Step 4: 落盘 hover-card.tsx** 删动画类；Positioner 加 `collisionPadding={8}`（与 popover.tsx 一致）。
- [ ] **Step 5: shimmer utility**（globals.css 尾部，keyframes 放 root 级）：

```css
@keyframes trace-shimmer { from { background-position: 100% 0; } to { background-position: -100% 0; } }
@utility shimmer {
  background-image: linear-gradient(90deg, currentColor 35%, color-mix(in oklab, currentColor 30%, transparent) 50%, currentColor 65%);
  background-size: 200% 100%;
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: trace-shimmer 2s linear infinite;
}
@utility shimmer-none { animation: none; background-image: none; -webkit-text-fill-color: currentColor; }
```

- [ ] **Step 6:** 测试 PASS；全套校验；提交 `feat(web): 引入 Trace / HoverCard 原语与 shimmer 工具类`。

### Task 2: Code Block + shiki，替换全部 hljs

**Files:**
- Create: `web/src/components/ui/code-block.tsx`、`web/src/lib/highlighter.tsx`、`web/src/hooks/use-highlighted-code.ts`、`web/src/components/chat/highlighted-code.tsx`
- Test: `web/src/lib/highlighter.test.ts`
- Modify: `web/src/components/chat/markdown-lite.tsx`、`web/src/app/(dashboard)/dataset/page.tsx`、`web/src/app/globals.css:10-13`（删 hljs import 与注释）、`web/package.json` / lock（卸 `highlight.js`、`rehype-highlight`）、`shadcn-native.test.ts`

**Interfaces:**
- Produces:
  - `resolveLanguage(language: string): LanguageName | "ansi" | null`（`lib/highlighter.tsx` 导出；官方 `getLanguage` 改名导出）
  - `shouldHighlight(code: string): boolean`（同文件，`code.length <= 200_000`）
  - `loadHighlighter(language)`、`useHighlightedCode(value, language): ReactNode[]`（官方签名）
  - `HighlightedCode({ code, language, title?, size?: "default"|"sm", className?, lineNumbers?, wrap? })`：组合 `CodeBlock` + `CodeBlockHeader(CodeBlockTitle + CodeBlockActions>CodeBlockCopyButton)` + `CodeBlockContent`，每行 `CodeBlockLine`；`shouldHighlight` 为假时传 `language=""`。
  - `CodeBlockCopyButton({ value })`：内部渲染项目 `CopyButton variant="icon"`（替代官方 navigator.clipboard 实现）。

- [ ] **Step 1: 写失败测试** `highlighter.test.ts`：
  - `resolveLanguage("ts") === "typescript"`、`("golang") === "go"`、`("RS") === "rust"`、`("dockerfile") === "dockerfile"`、`("foo") === null`、`("") === null`、`("ansi") === "ansi"`
  - `shouldHighlight("a".repeat(200_000)) === true`、`shouldHighlight("a".repeat(200_001)) === false`
  - shadcn-native：`expectGenerated(["code-block"])`；markdown-lite 不含 `rehype-highlight` 与 `hljs`、含 `<HighlightedCode`；dataset 页不含 `highlight.js` 与 `dangerouslySetInnerHTML`；`package.json` 不含 `"highlight.js"`。原「markdown-lite 复用 CopyButton」用例改为只对 grouped-view 断言，并新增断言 `components/ui/code-block.tsx` 含 `<CopyButton`。
- [ ] **Step 2:** 运行 → FAIL。
- [ ] **Step 3: 落盘 highlighter / hook / code-block** 语言表在官方 13 种上加 `go, rust, java, toml, dockerfile, xml`（`shiki/langs/<name>.mjs`），别名加 `golang→go, rs→rust`；shiki token 类改 `text-(--shiki-light) moonshot:text-(--shiki-dark)`；code-block 的 `dark:bg-input/30` 改 `moonshot:bg-input/30`；`CodeBlockTitle` 的 truncate 同 Task 1 处理。
- [ ] **Step 4: HighlightedCode**（`components/chat/highlighted-code.tsx`，`"use client"`）。
- [ ] **Step 5: markdown-lite** 删 `rehypeHighlight` 插件与本地 `CodeBlock`；`code` 组件块级分支改 `<HighlightedCode code={value} language={lang} title={lang || "text"} className="my-3" />`；Mermaid 与行内 code 不变。
- [ ] **Step 6: dataset 页** 删 hljs、`escapeHtml`、`formatAndHighlight`；新增 `prettyJSON(s)`（parse 成功返回 2 空格缩进，否则原文）；预览处改 `<HighlightedCode code={prettyJSON(formatPreview.sharegptJson)} language="json" size="sm" className="max-h-96" />`（去掉外层 ScrollArea 与 `--code-bg`；CodeBlockContent 自带 overflow-auto。若违反「代码 / JSON 预览用 ScrollArea」用例：该用例只断言无 `overflow-auto` 字面量于 page.tsx，仍成立）。
- [ ] **Step 7:** `npm uninstall --package-lock-only highlight.js rehype-highlight`；`grep -rn "highlight.js\|rehype-highlight\|hljs" web/src` 为空；`npx npm@10 ci --dry-run` exit=0。
- [ ] **Step 8:** 测试 PASS；全套校验；提交 `feat(web): 代码块换用 shadcn Code Block + shiki，移除 highlight.js`。

### Task 3: Reasoning（三页共用）

**Files:**
- Create: `web/src/components/chat/reasoning.tsx`
- Delete: `web/src/components/chat/reasoning-block.tsx`
- Modify: `web/src/components/chat/assistant-message.tsx`、`web/src/components/playground/chat-turn.tsx`（删 `ThinkingBlock`）、locales、`shadcn-native.test.ts`
- Test: `web/src/components/chat/reasoning.test.ts`

**Interfaces:**
- Produces: `Reasoning({ text: string; streaming: boolean })`；`formatCharCount(n: number): string`（`<1000` 原数，否则 `1.2k`，去尾 `.0`）。

- [ ] **Step 1: 写失败测试** `formatCharCount(980) === "980"`、`(1200) === "1.2k"`、`(12000) === "12k"`；shadcn-native：`reasoning-block.tsx` 不存在；assistant-message 与 chat-turn 均含 `<Reasoning`；chat-turn 不含 `ThinkingBlock`。原「思考块与工具卡之间只保留列表 gap」用例删除 reasoning-block 那一行。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: 实现** 结构见 spec §5.2：`text.trim()===""` 且非流式 → `null`；空文本流式 → 单行 `TraceLine status="running"`；否则 `TraceCollapsible open={userOpen ?? streaming} onOpenChange={setUserOpen}`。Description 文案 i18n key `chat.reasoning_chars` = `{count} 字` / `{count} chars` / `{count} 文字`。TraceTitle 包 Tooltip 不需要（标题是短固定文案，在 eslint-disable 理由内）。
- [ ] **Step 4:** assistant-message 用 `<Reasoning text={reasoning_content} streaming={false} />`；chat-turn 用 `<Reasoning text={reasoning} streaming={sending} />`。
- [ ] **Step 5:** PASS；全套校验；提交 `feat(web): 思考过程统一为 Trace Reasoning`。

### Task 4: Tool（工具调用）

**Files:**
- Create: `web/src/components/chat/tool-calls.tsx`、`web/src/components/chat/tool-payload.ts`
- Delete: `web/src/components/chat/tool-call-card.tsx`
- Modify: `web/src/components/chat/assistant-message.tsx`、`web/src/components/session-detail/tool-sidebar-item.tsx`（参数 schema 区块下方加 `<HighlightedCode language="json" size="sm">` 显示 `JSON.stringify(params, null, 2)`，折叠在原 Collapsible 内）、locales、`shadcn-native.test.ts`
- Test: `web/src/components/chat/tool-payload.test.ts`

**Interfaces:**
- Consumes: `HighlightedCode`（Task 2）、Trace 原语（Task 1）、`ToolResultInfo`、`lookupToolResult`（`content-extract.ts`）
- Produces:
  - `formatToolPayload(raw: string): { code: string; language: "json" | "" }`：空串 → `{code:"", language:""}`；可 JSON.parse → 2 空格缩进 + `"json"`；否则原文 + `""`。
  - `previewFirstArg(argsJSON: string): string`（从 tool-call-card 迁入）
  - `countLines(s: string): number`
  - `LONG_OUTPUT_LINES = 200`
  - `ToolCalls({ calls: UnifiedToolCall[]; resultsByID: Record<string, ToolResultInfo> })`

- [ ] **Step 1: 写失败测试** `formatToolPayload("")`、`('{"a":1}')` → `{code:'{\n  "a": 1\n}', language:"json"}`、`("plain text")` → `{code:"plain text", language:""}`；`previewFirstArg('{"path":"/x","n":2}') === 'path: "/x"'`、`("not json") === ""`；`countLines("a\nb\n") === 2`。shadcn-native：`tool-call-card.tsx` 不存在；assistant-message 含 `<ToolCalls`；tool-calls.tsx 含 `<TraceCollapsible`、`status="pending"`、`<SheetTrigger`。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: 实现** 结构见 spec §5.3。有结果 → `TraceCollapsible`；无结果（`call.id` 为空或查不到）→ `TraceLine status="pending"`。标题与描述各自包 `TooltipRoot`（文案即全文）。展开内容：输入 `HighlightedCode title={t("tool_call.input")} size="sm" className="max-h-64"`；输出同理（`title={t("tool_call.output")}`，内容为空时显示 `t("tool_call.empty_output")`）；`countLines(output) > LONG_OUTPUT_LINES` 时下方放 `Sheet`：`SheetTrigger render={<Button variant="link" size="xs" />}` 文案 `t("tool_call.view_full")`，`SheetContent side="right" className="sm:max-w-2xl"` 内 `SheetHeader>SheetTitle{call.name}` + `HighlightedCode lineNumbers className="min-h-0 flex-1"`。`call.id` 放在 Content 末尾 `text-[11px] font-mono` 小字。
- [ ] **Step 4:** assistant-message：`tool_calls` 段替换为 `<ToolCalls calls={tool_calls} resultsByID={toolResultsByID} />`，删外层 `flex flex-col gap-3` div 并同步删除 shadcn-native 中对该字符串的断言。
- [ ] **Step 5:** PASS；全套校验；提交 `feat(web): 工具调用改为 Trace Tool，输入输出用 Code Block`。

### Task 5: Chain of Thought + MessageScroller（详情 / 分享页）

**Files:**
- Create: `web/src/components/chat/transcript-groups.ts`、`web/src/components/chat/chain-of-thought.tsx`、`web/src/components/session-detail/session-transcript.tsx`
- Modify: `web/src/components/shared/reading-layout.tsx`、`web/src/components/session-detail/session-detail-client.tsx`、`web/src/app/share/page.tsx`、`web/src/components/chat/chat-message.tsx`（导出 `isToolResultMessage`）、locales、`shadcn-native.test.ts`
- Test: `web/src/components/chat/transcript-groups.test.ts`

**Interfaces:**
- Produces:
  - `isToolResultMessage(m: MessageItem): boolean`（`role==="tool"` 或 `role==="user"&&!!tool_call_id`）
  - `type TranscriptEntry = { kind: "message"; message: MessageItem; index: number } | { kind: "steps"; id: number; messages: MessageItem[]; callCount: number; toolNames: string[] }`
  - `groupTranscript(messages: MessageItem[]): TranscriptEntry[]`
  - `ChainOfThought({ entry: Extract<TranscriptEntry,{kind:"steps"}>; toolResultsByID })`
  - `SessionTranscript({ messages: MessageItem[]; hasMore: boolean; sentinelRef: Ref<HTMLDivElement>; emptyText: string; endText: string })`
  - `ReadingLayout` 新增 props：`standalone?: boolean`（分享页传 true：无负 margin、高度 `h-[100dvh]`）；`messagesScrollRootRef` 现在指向 `MessageScrollerViewport`；`headerSentinelRef` 移入 `MessageScrollerContent` 顶部。

- [ ] **Step 1: 写失败测试** `transcript-groups.test.ts`（构造最小 MessageItem 工厂）：
  - A 平铺：`[user, assistant(text)]` → 两个 `message` 条目，index 0/1。
  - B 夹 tool 结果：`[user, a(tc:Read), tool, a(tc:Grep,Read), tool, tool, a(text)]` → `[message(user), steps{callCount:3, toolNames:["Read","Grep"], messages: 两条 assistant}, message(a text)]`，steps.id 为首条 assistant 的 id。
  - C 单次调用不折叠：`[user, a(tc:Read), tool, a(text)]` → `[message, message(a tc), message(a text)]`。
  - D system 打断：`[a(tc:X), tool, system, a(tc:Y), tool]` → 不形成 ≥2 的段 → 全部 `message`（tool 结果不出现在输出中）。
  - E 段落在列表末尾（无限加载截断）：`[user, a(tc:A), tool, a(tc:B)]` → `[message, steps{callCount:2}]`。
  - shadcn-native：detail 与 share 页都含 `<SessionTranscript`；reading-layout 含 `<MessageScrollerViewport` 与 `direction="start"`；session-transcript 含 `<Marker`。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: groupTranscript** 先滤掉 tool 结果；顺序扫描，把连续「`role==="assistant"` 且 `tool_calls.length>0`」的消息收成段，段内调用总数 ≥2 → `steps`，否则逐条 `message`；`toolNames` 去重保序；`message.index` 为原数组下标（供入场动画 delay）。
- [ ] **Step 4: ChainOfThought** 结构见 spec §5.4（`ListTree` 图标，标题 i18n `chat.steps_title` = `执行了 {count} 次工具调用` / `Ran {count} tool calls` / `{count} 回のツール呼び出し`，描述取 `toolNames` 前 3 个用 `、`/`, ` 连接，超过加 `t("chat.steps_more")`），`TraceCollapsibleContent keepMounted` 内逐条 `<AssistantMessage>`。
- [ ] **Step 5: SessionTranscript** 渲染 `MessageScrollerItem`（messageId 取 `String(message.id)` / `steps-${id}`）+ `ChatMessage` / `ChainOfThought`；底部 sentinel Skeleton（`hasMore`）；结束用 `<Marker variant="separator"><MarkerContent>{endText}</MarkerContent></Marker>`；空态保持原 markup。
- [ ] **Step 6: ReadingLayout** 桌面与移动端主区都改为 `MessageScrollerProvider autoScroll={false} defaultScrollPosition="start"` > `MessageScroller className="min-h-0 flex-1"` > `MessageScrollerViewport ref={messagesScrollRootRef}` > `MessageScrollerContent`（原 padding 移到 Content，`gap-5`），外加两个 `MessageScrollerButton`（`direction="start"`、`direction="end"`，sr-only 文案 i18n `reading_layout.jump_start` / `jump_end`）。移动端：根容器 `h-[calc(100dvh-3.5rem)] flex-col`（standalone 时 `h-[100dvh]`），header 去掉 `sticky top-[-1rem]`，sentinel 移入 Content 顶部。
- [ ] **Step 7: 两个页面** 消息列表替换为 `<SessionTranscript>`；`headerCompact` 的 IntersectionObserver 加 `root: messagesScrollRootRef.current`；分享页 `<ReadingLayout standalone>`。
- [ ] **Step 8:** PASS；全套校验；提交 `feat(web): 会话详情与分享页接入 MessageScroller 与 Chain of Thought`。

### Task 6: Model Selector + reasoning_effort（Playground）

**Files:**
- Modify: `web/src/lib/provider-key.ts`、`web/src/app/(dashboard)/playground/playground-logic.ts`、`web/src/app/(dashboard)/playground/page.tsx`、`web/src/components/playground/composer.tsx`、locales、`shadcn-native.test.ts`
- Test: `web/src/lib/provider-key.test.ts`、`web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts`

**Interfaces:**
- Produces:
  - `providerGroupOf(alias: string): string`（返回组键；无 key → `"other"`）、`PROVIDER_GROUP_LABELS: Record<string,string>`（openai→OpenAI、anthropic→Anthropic、google→Google、zhipu→Zhipu、moonshot→Moonshot、tencent→Tencent、bytedance→ByteDance、xiaomi→Xiaomi、siliconcloud→SiliconCloud；未列出的首字母大写）
  - `groupModels<T extends {alias:string}>(models: T[]): { key: string; label: string; items: T[] }[]`（playground-logic；组按 items 数降序、同数按 key 字母序，`other` 恒最后，组内按 alias 字母序）
  - `type ReasoningEffort = "" | "low" | "medium" | "high"`；`REASONING_EFFORTS: ReasoningEffort[]`
  - `PlaygroundParams` 增加 `reasoningEffort: ReasoningEffort`
  - `buildChatBody(messages, model, reasoningEffort: ReasoningEffort = "")`
  - `PlaygroundModelOption` 增加 `contextLength: number`（供 Task 7）

- [ ] **Step 1: 写失败测试**
  - provider-key：`providerGroupOf("gpt-4o")==="openai"`、`("o3-mini")==="openai"`、`("claude-sonnet-4-5")==="anthropic"`、`("gemini-2.5-pro")==="google"`、`("glm-4.6")==="zhipu"`、`("kimi-k2")==="moonshot"`、`("hy3")==="tencent"`、`("deepseek-v3")==="deepseek"`、`("my-local-model")==="other"`、`("GPT-4O")==="openai"`。
  - `groupModels([{alias:"gpt-b"},{alias:"zzz"},{alias:"claude-x"},{alias:"gpt-a"}])` → keys `["openai","anthropic","other"]`，openai items `["gpt-a","gpt-b"]`。
  - `buildChatBody(msgs,"m","high")` 含 `reasoning_effort:"high"`；`buildChatBody(msgs,"m")` 与原用例完全相等（无该字段）。
  - `loadParams`：存 `{model:"m",apiKeyID:1,reasoningEffort:"max"}` → `reasoningEffort:""`；存 `"low"` → `"low"`；原两个用例的期望补 `reasoningEffort:""`。
  - shadcn-native：composer 含 `DropdownMenuRadioGroup`、`reasoningEffort`，不含 `<Select` 用于模型（断言模型区段：`aria-label={\`${t("playground.model")}` 存在）。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: 实现逻辑层** `providerGroupOf` 建一张 key→组 的归并表（spec §5.7 列出的归并），其余 key 原样作组键。
- [ ] **Step 4: page.tsx** `selectableModels` 映射带上 `contextLength: m.contextLength`；`runGenerate` 调 `buildChatBody(context, params.model, params.reasoningEffort)`；`DEFAULT_PARAMS.reasoningEffort = ""`。
- [ ] **Step 5: composer** 模型 Select 换成 DropdownMenu（结构 spec §5.7）：Trigger `render={<InputGroupButton size="sm" className={PILL_CLASS 改用的胶囊样式} />}`，内容 `ProviderIcon` + `TruncatedLabel` + `ChevronDown`；Content `align="start" className="max-h-80 w-auto min-w-56"`；`DropdownMenuRadioGroup value onValueChange` 包所有组，组间 `DropdownMenuSeparator`，项 `DropdownMenuRadioItem closeOnClick`。思考强度菜单紧随其后：Trigger 文案 `<span className="text-muted-foreground">{t("playground.effort")}</span> {label}`，Label `t("playground.effort.title")`，选项 `""→t("playground.effort.default")`、low/medium/high → `t("playground.effort.low"|...)`。API Key Select 不动。
- [ ] **Step 6:** PASS；全套校验；提交 `feat(web): Playground 模型选择改为 Model Selector 并支持 reasoning_effort`。

### Task 7: Context 悬停卡片（Playground）

**Files:**
- Create: `web/src/components/playground/context-card.tsx`
- Modify: `web/src/app/(dashboard)/playground/playground-logic.ts`、`web/src/components/playground/chat-turn.tsx`（删指标 Collapsible 与 `DetailCopyButton`）、`web/src/components/playground/conversation-view.tsx` 与 `page.tsx`（把当前模型的 `contextLength` 透传到 ChatTurn：按 `meta.model` 在 `models` 里查）、locales、`shadcn-native.test.ts`
- Test: `playground-logic.test.ts`

**Interfaces:**
- Consumes: `PlaygroundModelOption.contextLength`（Task 6）、HoverCard（Task 1）、`Progress`、`formatTokenCount`、`formatDurationMs`、`formatCost`、`buildCurl`
- Produces:
  - `contextSummary(usage: ChatUsage | undefined, limit: number): { used: number | null; percent: number | null; label: string }`：usage 缺失 → `{used:null, percent:null, label:"—"}`；`limit<=0` → `percent:null`、label = `${formatTokenCount(used)} tok`；否则 `percent=Math.round(used/limit*100)`，label = `percent===0&&used>0 ? "<1%" : \`${percent}%\``（不带「context」字样，由组件拼 i18n）。
  - `ContextCard({ meta: PlaygroundTurnMeta; contextLength: number })`
  - `ChatTurn` 新增 prop `contextLength: number`

- [ ] **Step 1: 写失败测试** `contextSummary(undefined, 128000)` → label `"—"`；`({promptTokens:8420,completionTokens:2240,cachedTokens:0}, 128000)` → `{used:10660, percent:8, label:"8%"}`；`(…100 tokens…, 128000)` → label `"<1%"`；`(…, 0)` → percent null、label `"10.7k tok"`（以 `formatTokenCount` 实际格式为准，先跑一次确认其输出再写期望）。shadcn-native：chat-turn 含 `<ContextCard`、不含 `DetailCopyButton`；context-card 含 `<HoverCardTrigger` 与 `<Progress`。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: 实现 ContextCard** 结构 spec §5.6：Trigger `render={<Button variant="ghost" size="xs" className="h-6 px-1.5 text-[11px] text-muted-foreground" />}` 文案 `label`（有 percent 时追加 ` ${t("playground.context")}`）+ ` · ${formatDurationMs(totalMs)} · ${formatCost(cost)}` + 中断后缀；Content `className="w-72 flex flex-col gap-3"`：头部行、`Progress`（percent 非 null 时）、明细三行、估算说明、两个复制按钮（项目 `Button variant="outline" size="sm"` + `Copy` 图标 `data-icon="inline-start"`，点击 `copyTextToClipboard`）。HoverCard `delay={10} closeDelay={100}`。
- [ ] **Step 4:** chat-turn 的 `MessageFooter` 内用 `<ContextCard meta={meta} contextLength={contextLength} />` 替换原 Collapsible。
- [ ] **Step 5:** PASS；全套校验；提交 `feat(web): Playground 指标改为 Context 悬停卡片`。

### Task 8: 运行时验证与收尾

**Files:** 无产品代码（发现问题回到对应 Task 修）

- [ ] **Step 1: 构建** worktree 内 `npx next build --webpack`（Turbopack 拒绝跨 root 软链）；记录 `out/_next/static/chunks` JS gzip 总量，与 master 构建对比；确认 shiki 语言/主题在独立 chunk（首屏 HTML 引用的 chunk 不含 `github-light`）。
- [ ] **Step 2: 无头验证**（chrome-headless-shell + CDP 脚本，`--use-angle=swiftshader`；harness 用 esbuild 打包临时入口挂载组件、mock 数据，放 `web/.harness/`，结束删除）：
  - SessionTranscript：Review Focus 1 的数据渲染出 1 个 Chain of Thought；点开后 ToolCalls 展开出现 `[data-slot=code-block-line]` 且 span 带 `--shiki-light`；两个跳转按钮点击后 `scrollTop` 分别为 0 / 最大；390×844 视口滚动后 header 收缩。
  - `[data-theme=moonshot]` 下 token 计算色取 `--shiki-dark`。
  - `--force-prefers-reduced-motion` 下 `[data-slot=trace-title]` 的 `animation-name` 为 `none`。
  - ComposerHarness：打开模型菜单出现分组 Label；选 effort=high 后拦截 `fetch` 请求体含 `"reasoning_effort":"high"`。
  - ContextCard：hover 触发器后 `[data-slot=hover-card-content]` 可见；无上限 meta 不渲染 `[data-slot=progress]`。
- [ ] **Step 3:** `npm run lint && npm run typecheck && npm run test && npm run format:check`；`npx npm@10 ci --dry-run`。
- [ ] **Step 4:** spec 末尾追加「§10 实施偏差」记录实际与设计不同之处；用 `write_memory`/记忆沉淀可复用经验；提交。
- [ ] **Step 5:** 询问用户提 PR 还是合并 master。
