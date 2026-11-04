# 会话详情 / 分享 / Playground 接入 shadcn AI Elements 设计

> 日期：2026-11-04 · 范围：仅 `web/` 前端，后端零改动

## 1. 意图

会话详情页（`/sessions/detail`）、分享页（`/share`）与 Playground（`/playground`）是项目里所有「对话 UI」。上一轮 shadcn 替换（2026-10-15 round3）已把 Message 层（`Message` / `Bubble` / `Marker` / `Attachment` / Playground 的 `MessageScroller`）换完，但 agent 每一轮里的「步骤」——思考、工具调用、代码块、调试指标、模型选择——仍是手写 `Collapsible` + div 拼装。

本次按 shadcn AI Elements（https://ui.shadcn.com/docs/ai-elements）的官方写法把这些步骤统一到 **Trace** 体系，代码高亮换成官方 **Code Block + shiki**，Playground 补齐 **Context** 与 **Model Selector**，并让详情/分享页也走 **MessageScroller**。目标是三处对话 UI 视觉与交互一致、全部可在 shadcn 文档里找到对应模式。

## 2. 目标

1. **Trace 基础设施**：新增 `components/ui/trace.tsx`；`shimmer` 工具类在项目内自定义（CI 锁定的 shadcn 4.8.0 `tailwind.css` 没有它）。
2. **Reasoning**：`chat/reasoning-block.tsx`（详情/分享）与 `playground/chat-turn.tsx` 内的 `ThinkingBlock` 合并为一个基于 Trace 的组件；流式中展开 + `status="running"` shimmer，结束自动收起，用户手动切换后以用户为准（`open ?? running`）。
3. **Tool**：`chat/tool-call-card.tsx` 改为 Trace 行；一条 assistant 消息的全部 `tool_calls` 放进同一个 `Trace`；有结果的行是 `TraceCollapsible`，展开显示输入/输出 `CodeBlock`；无结果的行 `status="pending"`。
4. **Code Block + shiki**：引入官方 `code-block.tsx`、`use-highlighted-code.ts`、`highlighter.tsx`；Markdown 代码块、工具输入/输出、工具定义参数 schema、dataset 页 JSON 预览全部改用它；删除 `rehype-highlight` 与 `highlight.js` 依赖及相关 CSS。
5. **Chain of Thought（详情/分享页）**：连续的工具步骤折叠成一行摘要，展开才看到逐条步骤（规则见 §5.4）。
6. **MessageScroller（详情/分享页）**：消息列表换成 `MessageScroller`，从头阅读，提供「跳到开头 / 跳到结尾」按钮；无限加载与移动端标题收缩迁移到 viewport 上。
7. **Context（Playground）**：消息底部指标改为 `HoverCard` 悬停卡片。
8. **Model Selector（Playground）**：模型选择从 `Select` 改为 `DropdownMenu` + `DropdownMenuRadioGroup`，按提供商分组；新增思考强度菜单，请求体带 `reasoning_effort`。

## 3. 非目标

- 不按工具名做专用样式（Terminal / Plan / Queue / Subagent / Commit）。
- 不改 `PendingDots`（首 token 前的主题签名动效，`motion-signature.css` ⑤）与 `.stream-body` 流式签名结构。
- 不改 Mermaid 渲染。
- 不改后端；不改 API Key 选择器；不加 Playground 附件上传。
- 不引入 AI SDK（`ai` / `@ai-sdk/react`）——数据仍是 OpenAI 统一格式 `MessageItem` / `PlaygroundMessage`，状态由我们自己映射。

## 4. 约束（项目硬规则）

- **禁止直接 `shadcn add`**：用 `./node_modules/.bin/shadcn add <name> --dry-run --view <path>` 或 `shadcn view <name>` 取源码落盘；`from "cn"` 改 `@/lib/utils`；`@/registry/base-nova/...` 改项目别名；`IconPlaceholder` 换成 `lucide-react` 具体图标。
- 删除 tw-animate 进出场类（`data-open:animate-in` 等）与 `scroll-fade-*` / `shimmer`（官方类，CI 版本不存在——由我们自定义的同名 utility 承接，见 §5.1）/ `scrollbar-*` 空操作类。动效写在 `styles/motion-{anthropic,moonshot}.css` 语法层，按 `data-slot` 命中。
- `shadcn-native.test.ts` 守护上述规范，新增文件必须通过。
- 新依赖：`npm install --package-lock-only --save-exact shiki@<最新稳定版>`，再 `npm pack` 解包进共享 `node_modules`；删除依赖同理只改 lock；提交前 `npx npm@10 ci --dry-run` 做 CI parity。
- `truncate-requires-tooltip` lint：`TraceTitle` / `CodeBlockTitle` 自带 `truncate`，组件内加带理由的 `eslint-disable`；业务侧长标题用 Tooltip 包。
- 主题只有两套：`anthropic`（浅色）与 `moonshot`（深色，`[data-theme="moonshot"]`，自定义变体 `moonshot:`）。项目不使用 `.dark` 类。
- Prettier `format:check` 在 CI 里跑，改完必须 `npm run format`。
- 在 `.worktrees/` 下开发，分支 `feature/ai-elements-chat-2026-11-04`；软链主工作区 `web/node_modules`。

## 5. 设计

### 5.1 基础组件与工具类

| 新文件 | 来源 | 适配 |
|---|---|---|
| `components/ui/trace.tsx` | `shadcn view trace` | cn 路径；Collapsible 走项目 `@/components/ui/collapsible`；chevron 图标用 `ChevronRight`（与现有 `in-data-panel-open:rotate-90` 惯例一致） |
| `components/ui/code-block.tsx` | `shadcn view code-block` | 同上；`CodeBlockCopyButton` 复用项目 `lib/clipboard.ts` 的 `copyTextToClipboard`（降级兼容） |
| `components/ui/hover-card.tsx` | `shadcn view hover-card` | 同上；删进出场动画类 |
| `hooks/use-highlighted-code.ts` | `shadcn view use-highlighted-code` | 原样 |
| `lib/highlighter.tsx` | 同上 | 见下文主题与语言 |

**shimmer**：在 `globals.css` 用 `@utility shimmer` 与 `@utility shimmer-none` 定义（文字渐变扫光：`background-clip:text` + 复用已有 `@keyframes shimmer`，颜色基于 `currentColor` / `--muted-foreground`，两主题自适应）；`prefers-reduced-motion` 全局守卫已覆盖 animation。

**shiki 主题**：官方用 `text-(--shiki-light) dark:text-(--shiki-dark)`，项目改为 `text-(--shiki-light) moonshot:text-(--shiki-dark)`。主题对选 `github-light` / `github-dark`（官方默认；anthropic 暖色纸感下 github-light 足够中性）。

**shiki 语言**（按需懒加载）：官方列表（bash/css/diff/html/javascript/json/jsx/markdown/python/sql/tsx/typescript/yaml）+ 本项目会话里高频的 `go`、`rust`、`java`、`toml`、`dockerfile`、`xml`；别名补 `golang→go`、`rs→rust`。未收录语言 → 不高亮，按纯文本行渲染（官方 hook 行为）。

**CodeBlock 外观**：采用官方风格（`border-input` 细边框、跟随主题），对话 UI **不再**使用 `--code-bg/--code-border/--code-text` 固定深色底；这三组变量 `components/portal/feature-art.tsx` 仍在用，**保留不删**。

**依赖版本**：`shiki@4.6.0`（撰写时 npm 最新稳定版），`--save-exact`。

### 5.2 Reasoning（三页共用）

新文件 `components/chat/reasoning.tsx`，导出 `Reasoning({ text, streaming })`：

```
Trace
└─ text 为空 → TraceLine(status=running?) 仅一行「思考中」
└─ 否则 TraceCollapsible(open = userOpen ?? streaming, onOpenChange=setUserOpen)
   ├─ TraceCollapsibleTrigger > TraceLine(status = streaming ? "running" : undefined)
   │    ├─ TraceIcon: streaming ? <Spinner/> : <Brain/>
   │    ├─ TraceTitle: streaming ? t("playground.thinking.streaming") : t("chat.thought_process")
   │    └─ TraceDescription: 字数（如「1.2k 字」）——无耗时数据，用字数代替
   └─ TraceCollapsibleContent: whitespace-pre-wrap 正文（保持原 italic/muted 语气）
```

- 详情/分享页传 `streaming={false}`，默认收起（与现状一致）。
- 删除 `chat/reasoning-block.tsx` 与 `chat-turn.tsx` 内的 `ThinkingBlock`。

### 5.3 Tool（三页共用，目前只有详情/分享页有工具调用）

新文件 `components/chat/tool-calls.tsx`，导出 `ToolCalls({ calls, resultsByID })`，替代 `tool-call-card.tsx`：

```
Trace
└─ 每个 call：
   ├─ 有结果 → TraceCollapsible
   │   ├─ Trigger > TraceLine
   │   │    ├─ TraceIcon <Wrench/>
   │   │    ├─ TraceTitle  {call.name}           （Tooltip 包裹，长名可看全）
   │   │    └─ TraceDescription 首个参数预览      （previewFirstArg，Tooltip 看全）
   │   └─ Content
   │        ├─ CodeBlock size=sm  title=t("tool_call.input")  lang=json  内容 prettyJSON(args)
   │        └─ CodeBlock size=sm  title=t("tool_call.output") lang=json|纯文本（能 JSON.parse 则 json）
   │        └─ call.id 作为 CodeBlockFooter 小字
   └─ 无结果 → TraceLine status="pending"（同上 icon/title/description），不可展开
```

- `CodeBlockContent` 加 `className="max-h-64"`（官方长输出建议）；输出超过 200 行时在 Content 底部放「在侧板查看全部」按钮，打开 `Sheet` 显示带行号的完整输出（官方 Long output 模式）。
- `CodeBlock` 自带复制按钮，替代原两个 `CopyButton`。

### 5.4 Chain of Thought（详情/分享页）

**分组规则**（纯函数 `groupTranscript(messages)`，放 `components/chat/transcript-groups.ts`，vitest 覆盖）：

- 先剔除工具结果消息（`role==="tool"` 或 `role==="user"` 且有 `tool_call_id`，现状已在 `ChatMessage` 里 return null）。
- 「步骤段」= 连续的、带 `tool_calls` 的 assistant 消息组成的最大序列。
- 步骤段内工具调用总数 **≥ 2** 时折叠为一个 Chain of Thought；只有 1 个调用时照常平铺（折叠一个调用没有意义）。
- 不带 `tool_calls` 的 assistant 消息（最终回答）、user、system 消息照常平铺，断开步骤段。

**渲染**：

```
Trace
└─ TraceCollapsible(defaultOpen=false, keepMounted)
   ├─ Trigger > TraceLine
   │    ├─ TraceIcon <ListTree/>
   │    ├─ TraceTitle 「执行了 {n} 次工具调用」 / "Ran {n} tool calls"
   │    └─ TraceDescription 涉及的不同工具名前 3 个 + 「等」（如 Read、Grep、Edit 等）
   └─ Content：按原顺序渲染段内每条 AssistantMessage（含其文本、Reasoning、ToolCalls）
```

- 每个分组整体是一个 `MessageScrollerItem`（messageId 用组内首条消息 id）。
- 无限加载时分组在已加载数据上计算，新页到达后末尾分组可能扩大——可接受。

### 5.5 MessageScroller（详情/分享页）

抽出共享组件 `components/session-detail/session-transcript.tsx`，详情页与分享页都用它渲染消息列表（消除两处重复的列表 + sentinel + 结束语代码）。

- `MessageScrollerProvider autoScroll={false} defaultScrollPosition="start"`。
- `ReadingLayout` 改为：由它提供 MessageScroller 的 Root/Viewport（桌面与移动端都是**固定高度容器 + viewport 内部滚动**），`children` 放进 `MessageScrollerContent`。
- 两个 `MessageScrollerButton`：`direction="start"`（顶部）与 `direction="end"`（底部），aria 文案走 i18n。
- 无限加载：`IntersectionObserver` 的 `root` 改为 Viewport 的 ref（Viewport 是 `ComponentPropsWithRef`）。
- **移动端标题收缩**：原逻辑依赖整页滚动 + 页面顶部 sentinel；改为 sentinel 放在 viewport 内容顶部、`IntersectionObserver` root 设为 viewport，`headerCompact` 语义不变。移动端 header 从 `sticky` 改为 flex 布局里的固定行（不再需要 sticky）。
- 「对话结束」标记改用 `Marker`（项目规则：分隔与系统提示用 Marker）。
- 加载中骨架与空态保持不变（空态继续用现有样式，不在本次范围内改 Empty）。

### 5.6 Context（Playground）

替换 `chat-turn.tsx` 的指标 Collapsible。

```
HoverCard
├─ HoverCardTrigger(delay=10, closeDelay=100, render=<Button variant="ghost" size="xs"/>)
│    文案：有上限 → 「{percent}% context」；无上限 → 「{total} tok」
│    后缀：· {totalMs} · {cost}（与现状一致），中断时追加「已中断」
└─ HoverCardContent (w-72, flex-col gap-3)
     ├─ 头部：「Context」 | 「{used} / {limit}」（无上限只显示 used）
     ├─ Progress value=percent aria-label   （无上限时不渲染）
     ├─ 明细：输入 / 缓存 / 输出 token；首 token / 总耗时；预估成本 + 估算说明
     └─ 操作：复制请求 JSON、复制 curl（保留现有两个按钮）
```

- **上限来源**：Playground 已在拉 `listModelsPage`，把 `contextLength` 带进 `PlaygroundModelOption`；`contextLength <= 0` 视为未知。
- **used 口径**：`promptTokens + completionTokens`（OpenAI 的 completion 已含 reasoning token，不重复加）。percent 用 `Math.round`，最小显示 `<1%`。
- 触屏无 hover：HoverCard 在触屏上点击触发（Base UI 行为），无需额外处理；需在移动端视口验证。

### 5.7 Model Selector（Playground）

替换 `composer.tsx` 的模型 `Select`：

```
DropdownMenu
├─ DropdownMenuTrigger render=<InputGroupButton size="sm" .../>   （ml-auto 贴右）
│    <ProviderIcon/> 别名（截断 + Tooltip） <ChevronDown/>，aria-label=`${t("playground.model")}: ${alias}`
└─ DropdownMenuContent align="start" (max-h 限高可滚动)
   └─ DropdownMenuRadioGroup value=model
      ├─ DropdownMenuGroup  DropdownMenuLabel「OpenAI」  RadioItem×N（closeOnClick，带 ProviderIcon）
      ├─ DropdownMenuSeparator
      └─ …（按提供商分组，「其他」组放最后）
```

- **分组键**：`findProviderKey(alias)`（`lib/provider-key`）返回的是**匹配前缀**而非厂商（如 `gpt`、`o3`、`openai` 是三个 key），因此新增纯函数 `providerGroupOf(alias)`（放 `lib/provider-key.ts`，vitest 覆盖）把 key 归并为厂商组：`openai/gpt/o1/o3/dall/tts/codex → OpenAI`、`anthropic/claude/claudecode → Anthropic`、`google/gemini → Google`、`zhipu/chatglm/glm → Zhipu`、`moonshot/kimi → Moonshot`、`tencent/hunyuan/hy → Tencent`、`bytedance/doubao/volcengine → ByteDance`、`xiaomimimo/mimo → Xiaomi`、`siliconcloud/silicon → SiliconCloud`；其余 key 自成一组（显示名首字母大写）；无 key 归入「其他」。组顺序按组内模型数降序，「其他」最后；组内按别名字母序。
- **思考强度菜单**（新增，紧挨模型菜单）：

```
DropdownMenu
├─ Trigger render=<InputGroupButton size="sm"/>  「<span muted>Effort</span> {label}」<ChevronDown/>
└─ Content  DropdownMenuGroup  DropdownMenuLabel「思考强度」
     RadioGroup：默认（不传） / Low / Medium / High
```

- `PlaygroundParams` 增加 `reasoningEffort: "" | "low" | "medium" | "high"`，默认 `""`，随 `saveParams` 持久化（旧存储缺字段 → 默认）。
- `buildChatBody(messages, model, reasoningEffort)`：非空时写入 `reasoning_effort`；`requestSnapshot` 与 curl 自动包含。后端 `internal/dto/openai/chat.go` 已透传该字段，无需改动。
- 「默认」不传字段，避免不支持推理的模型被上游拒绝。
- API Key `Select` 不动。

## 6. 删除清单

- `components/chat/reasoning-block.tsx`、`components/chat/tool-call-card.tsx`
- `chat-turn.tsx` 内 `ThinkingBlock`、`DetailCopyButton`（并入 Context 卡片）
- `markdown-lite.tsx` 内手写 `CodeBlock` 与 `rehype-highlight` 接入
- `dataset/page.tsx` 的 `highlight.js` 用法与 `[&_.hljs-*]` 类
- `globals.css` 的 hljs 主题样式（`--code-*` 变量保留，portal 在用）
- 依赖：`highlight.js`、`rehype-highlight`（确认无其他引用后）

## 7. 验证

- `npm run lint`、`npm run test`、`npx tsc --noEmit`、`npm run format:check`、`npm run build`（worktree 内按已知 Turbopack 软链规避）。
- vitest：`groupTranscript`、`buildChatBody` 的 `reasoning_effort` 分支、模型分组函数、Context 百分比/无上限分支。
- 无头浏览器（chrome-headless-shell + CDP）在构建产物上断言：
  - 详情页：MessageScroller 视口存在且可滚动、跳到开头/结尾按钮生效、无限加载触发、移动端（390×844）标题收缩、Chain of Thought 展开、Tool 展开后 CodeBlock 有 shiki token（`--shiki-light` 样式存在）。
  - 两主题下 shiki 颜色变量切换（moonshot 用 `--shiki-dark`）。
  - Playground：Model Selector 分组渲染、选择后请求体（拦截 fetch）含/不含 `reasoning_effort`、HoverCard 悬停出现、无上限分支。
  - `prefers-reduced-motion` 下 shimmer 停止。
- 包体：记录引入 shiki、移除 hljs 前后 `out/_next/static` 的 JS gzip 总量；shiki 必须是懒加载 chunk，不进首屏。

## 8. 留给实施者的决定

- 各组件的精确间距、字号，以「与相邻 Message/Bubble 协调」为准。
- i18n key 命名（中英文都要补）。
- Chain of Thought 描述里工具名的截断策略细节。
- 长输出 Sheet 的阈值（暂定 200 行）可按实际效果微调。

## 9. 本文中由 agent 做出的决定（用户未逐条确认）

1. CodeBlock 改为官方浅边框风格，弃用固定深色代码底。
2. dataset 页 JSON 预览一并迁到 shiki，从而彻底移除 highlight.js。
3. shiki 主题用 github-light / github-dark；在官方语言列表外加 go/rust/java/toml/dockerfile/xml。
4. Reasoning 描述用字数而非耗时（无耗时数据）。
5. Chain of Thought 仅在步骤段工具调用 ≥2 时折叠。
6. 抽出 `session-transcript.tsx` 供详情页与分享页共用。
7. 思考强度提供「默认（不传）」选项且为默认值。
8. Context 无上限时降级为只显示 token 数、不显示进度条。
9. 模型分组按 `providerGroupOf`（在 `findProviderKey` 之上归并厂商），组按模型数降序、「其他」最后。
10. 工具长输出 >200 行时提供 Sheet 查看全文（官方 Long output 模式）。
11. 「对话结束」标记改用 `Marker`。

## 10. 实施偏差（2026-11-04）

1. **shiki 版本 4.2.0（非 4.6.0）**：4.2.0 已作为 `@lobehub/ui` 的 peer 在 lock 中并已安装，直接声明避免双版本。
2. **shiki core 与正则引擎也走动态 import**：静态 import 会把约 52KB(gz) 打进所有含代码块页面的首屏；改后四个对话类页面首屏 JS 均下降（移除 hljs 的收益），全站 chunk 总量 +154KB 均为按需加载的语言/主题。
3. **无语言围栏不再自动检测语言**：shiki 无自动检测，按纯文本渲染（原 rehype-highlight `detect: true`）。
4. **`HighlightedCode` 区分 `containerClassName`（外框）与 `className`（滚动区）**；`CodeBlockCopyButton` 内部复用项目 `CopyButton`（降级复制 + 失败 toast）。
5. **跳到开头/结尾用 `behavior="instant"`**：`MessageScrollerItem` 的 `content-visibility:auto` 按 10rem 估高占位，平滑滚动途中内容增高会停在旧终点（实测 5680/6689）；瞬时跳转实测到底。
6. **Trace 标题的 Tooltip 嵌在 `TraceTitle` 内部**：`TooltipTrigger render={<TraceTitle/>}` 会用 `tooltip-trigger` 覆盖 `data-slot="trace-title"`，导致 Trace 的 running shimmer、图标间距等按 slot 匹配的样式失效。
7. **`shimmer` utility 内置 reduced-motion 静止态**：父级 `data-[status=running]:*:data-[slot=trace-title]:shimmer` 优先级高于 `motion-reduce:shimmer-none`。
8. **`groupModels` 放在 `lib/provider-key.ts`**（playground-logic 约定无运行时项目内 import）；归并表补齐 whisper/gemma/palm/command/zai/codex 等前缀。
9. **`contextSummary` 不含 token 文案**，由 `ContextCard` 用 `formatTokenCount` 拼接（同上约定）。
10. **`ReadingLayout`**：删除未使用的 `onMessagesScroll`；滚动 viewport 改为整列宽、内容列 768px 居中；移动端 header 由 sticky 改为固定行；新增 `standalone`（分享页）。
