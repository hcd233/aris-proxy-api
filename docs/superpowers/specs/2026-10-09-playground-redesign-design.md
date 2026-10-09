# Playground 布局与交互重设计 — 设计文档

> 分支：`feature/playground-chat-redesign-2026-10-09`
> 状态：待用户评审
> 决策方式：brainstorming（含浏览器 mockup 两屏：布局骨架三选一 → 交互细节确认）

## 1. 背景与目标

Playground 现状（`web/src/app/(dashboard)/playground/page.tsx`，314 行单文件）是左右双卡片的
表单式调试页：左卡片「请求」（模型 / API Key / temperature / max_tokens / stream 开关 +
消息行编辑器：role 下拉 + **单行 Input**），右卡片「输出」（`<pre>` 纯文本）。主要痛点：

1. 消息内容是单行输入框，长 prompt 无法多行编辑；
2. 输出无 markdown / 代码渲染，长文本难读；
3. 非对话式——每次发送清空输出，多轮要手动加行拼消息，assistant 回复不能自动进入上下文；
4. 无 token 用量 / 耗时 / 费用展示，无实际请求体预览，调试价值低；
5. 错误只弹 toast，不留痕在对话上下文中。

**目标**：重设计为**对话式调试工作台**——自然多轮对话为主心智，同时保留网关调试所需的
可观测性（用量、耗时、估算费用、请求体、复制为 curl）。纯前端重构，后端零改动。

## 2. 范围

### 2.1 包含

1. 页面布局重构：页内两栏（本地会话列表 + 全宽对话流），参数收进右侧滑出抽屉。
2. 对话运行链路：发送 / SSE 流式渲染 / 停止 / 重跑 / 编辑（截断重发）/ 删除 / 插入 system/assistant 消息。
3. 本地多会话持久化（localStorage），会话切换 / 新建 / 重命名 / 删除。
4. 消息级折叠调试详情：tokens / 耗时 / 估算费用 + 展开（输入输出 token、首字延迟、请求体 JSON、复制为 curl）。
5. 错误内联展示（错误卡 + 重试）。
6. i18n（en/zh/ja）、双主题（anthropic/moonshot）适配、响应式与键盘操作。
7. 纯逻辑单测（vitest）扩展。

### 2.2 明确不做（YAGNI）

| 不做项 | 理由 |
|---|---|
| 后端会话持久化 / 多设备同步 | playground 流量走 `SkipStore` 不污染会话数据集是既有设计；本地多会话已覆盖调试场景 |
| 消息拖拽排序 | 全自由编辑（增删改）已覆盖调试构造需求，顺序调整是低频操作 |
| 参数预设 / 模板管理 | v1 先把单次调试体验做顺，预设体系是独立特性 |
| 前端重算时段/上下文区间定价 | 规则复杂且易与审计口径漂移；只用「无条件默认规则（代表档）」估算并标注 |
| 消息多分支 / 树状对比 | 对比多次生成结果是 v2 话题，v1 重跑即覆盖基本对比 |
| 后端改动（usage/费用透传接口） | OpenAI 格式已含 usage（流式靠 `stream_options.include_usage`）；不为前端展示加接口 |

## 3. 已确认的产品决策

| 决策点 | 结论 | 备选与理由 |
|---|---|---|
| 核心形态 | 对话式工作台 | 备选：调试工作台 / 双模式 / 只修痛点；对话式同时解决自然多轮与长文阅读 |
| 编辑自由度 | 全自由编辑 | 备选：追加式 / 行编辑器保留；需保留 system/assistant 注入等网关调试能力 |
| 历史持久化 | 本地多会话（localStorage） | 备选：单对话草稿 / 后端持久化 / 不持久化 |
| 调试信息 | 消息级折叠详情 | 备选：全局状态栏 / 全部展开 / 不展示；对话流干净与深挖能力兼得 |
| 页面布局 | 两栏 + 参数抽屉（mockup 方案 B） | 备选 A 三栏常驻参数栏（三层侧栏挤压对话宽度）、C 单栏沉浸（工作台属性弱化） |
| 消息操作按钮 | hover 浮现（mockup 方案 H） | 备选：常驻操作条（视觉噪音大）；触屏改为点按消息唤出 |

## 4. 布局与交互设计

### 4.1 布局骨架（方案 B）

```
┌──────────────────────────────────────────────┐
│ 会话列表（w-60，可收起） │ 对话流 + 底部输入区（flex-1）      │
│  [+ 新建对话]                │  ⚙ 点开 → 参数抽屉右侧滑出       │
│  ▪ 会话 1（当前）            │                                  │
│  ▪ 会话 2                    │  ● user: …                       │
│  ▪ 会话 3                    │  ✦ assistant: …（markdown）      │
│                              │    hover: 复制/编辑/重跑/删除    │
│                              │    1.2k tok · 3.4s · $0.0120 ⌄  │
│                              │  [输入消息…            ] [发送▶] │
└──────────────────────────────────────────────┘
```

- 会话列表常驻左栏（桌面）；`< lg` 收起为顶部按钮唤出的抽屉。
- 对话流全宽，消息气泡视觉对齐 `components/chat/` 既有样式。
- 参数抽屉（模型 / API Key / temperature / max_tokens / 流式开关）从右侧滑出；窄屏全宽。
- 输入框内嵌当前生效值 chips（模型、流式）与 ⚙ 按钮——弥补抽屉「参数不可一眼确认」的代价。

### 4.2 消息操作（hover 浮现）

- 每条消息 hover 浮现操作条：复制 / 编辑 / 重跑（仅 assistant）/ 删除；移开即隐；触屏点按唤出。
- **编辑**：消息原地变多行编辑框，「保存并重跑」= 更新内容、移除其后的全部消息，并以截断后的序列为上下文自动发起一次生成（若编辑的是 assistant 消息、序列以其结尾，则语义为续写）；保存前警示文案：其后 N 条将被移除。「取消」还原。
- **重跑**：仅 assistant 消息有——移除该条回复本身（保留其前所有消息），以此前文重新生成。语义统一为「重新生成这一轮回复」。
- **删除**：直接移除该消息，不触发请求。

### 4.3 输入区（composer）

- 多行 textarea：Enter 发送 / Shift+Enter 换行 / Esc 取消当前编辑态。
- `+ system` / `+ assistant` 按钮：追加对应角色的空消息到序列尾部（进入下一轮请求上下文）。
- 发送中按钮变为「停止」（AbortController），已生成部分保留并标「已中断」。

## 5. 组件架构

```
web/src/app/(dashboard)/playground/
├── page.tsx                 布局骨架 + 状态装配（权限守卫保留）
├── playground-logic.ts      纯逻辑（无 React）：请求体构建、SSE 解析、费用估算、
│                            localStorage 读写与淘汰
└── __tests__/playground-logic.test.ts

web/src/components/playground/
├── session-sidebar.tsx      会话列表：新建 / 切换 / 重命名 / 删除
├── conversation-view.tsx    对话流 + 自动滚底（用户手动上滚则暂停跟随，回到底部恢复）
├── chat-turn.tsx            单条消息：气泡 + hover 操作条 + 指标行 + 编辑态 + 错误卡
├── composer.tsx             输入区（含角色插入与发送/停止）
└── params-drawer.tsx        参数抽屉
```

- assistant 文本渲染复用 `components/chat/markdown-lite.tsx`（含代码块）。
- 复用 `selectableModels` / `fetchAllPages` / `ownedKeys` / `buildChatBody` 等既有纯函数；
  保留「管理员只列本人名下 Key 与模型别名」的租户隔离逻辑。
- 图标统一 `lucide-react`；样式 Tailwind v4 + CSS 变量（不写内联 hex）；toast 用 `sonner`。

## 6. 状态与持久化

- `localStorage("playground.sessions.v1")`：
  `{ version: 1, activeId, sessions: [{ id, title, createdAt, updatedAt, messages: [...] }] }`，
  消息结构 `{ id, role, content, meta? }`，`meta` 存该轮的 usage / 耗时 / 估算费用 / 请求快照 / 中断标记。
- `localStorage("playground.params.v1")`：模型 / Key / temperature / max_tokens / 流式开关（跨会话记忆）。
- 会话标题 = 首条非空 user 消息前 30 字符，自动维护。
- 淘汰：最多 50 个会话，超出删除 `updatedAt` 最旧者；写入失败（配额满）降级为内存态并 toast 提示。
- 版本降级：读到未知 `version` 视为空列表重建，不抛错。
- 模型 / Key 列表仍按现状实时拉取（enabled 模型 + 本人名下 Key），不进 localStorage。

## 7. 对话运行链路

1. **发送**：composer 追加 user 消息 → `buildChatBody(messages, params)`（空行剔除、可选参数仅有效时携带）
   → body 增加 `stream_options: { include_usage: true }` → `api.playgroundChatStream`。
2. **流式解析**：`extractDeltaContent` 扩展为 `parseSSELine(line)`：
   返回 `{ type: "delta", text } | { type: "usage", usage } | { type: "done" } | { type: "skip" }`，
   畸形 JSON 归为 `skip`。usage 帧为 OpenAI 尾帧（`choices: []` + `usage`）。
3. **渲染**：增量写入末条 assistant 消息，按帧节流 setState（rAF 合并），避免长回复逐字重排 markdown 的卡顿。
4. **usage 缺失退化**：上游不回 usage 时指标行只显耗时，token/费用位显示 `—`。
5. **停止**：abort 后保留已生成文本，`meta.interrupted = true`。
6. **非流式模式**（stream 开关关闭）：沿用 `api.playgroundChat`，一次性写入响应并解析 `usage`。

## 8. 调试指标（消息级折叠详情）

- **指标行**（每条 assistant 消息下，恒定短占位不截断）：`{总 tokens} tok · {耗时} · {估算费用}`；
  点击展开/收起详情。中断轮追加 `· 已中断`。
- **展开详情**：
  - 输入 / 输出 / 缓存读取 tokens（来自 usage：`prompt_tokens` / `completion_tokens` / `prompt_tokens_details.cached_tokens`）；
  - 首字延迟（发送 → 首个内容增量的时间）与总耗时；
  - 请求体 JSON 查看（本轮实际发送的 messages 快照）；
  - **复制为 curl**：指向网关 OpenAI 兼容端点 `POST /api/openai/v1/chat/completions` 的 `curl` 命令
    （复制出去即可作真实调用命令使用），`Authorization` 头用 `<API_KEY>` 占位脱敏。
- **费用估算**：纯函数 `estimateCost(usage, pricing)`——取模型 `pricing.rules` 中的无条件默认规则
  （`time_windows` 空 + `context_min=0` + `context_max=0`，即 PricingInline 的「代表档」口径），
  按 `input_price × (prompt − cached) + cache_read_price × cached + output_price × completion` 计算；
  未计价（`currency` 空）显示 `—`。详情内标注「估算，精确值以审计为准」。
- 错误不弹 toast 了事：**错误卡**内联在对应轮次（状态码 + message + 重试按钮）；401 仍走全局 token 刷新不展示。

## 9. i18n / 主题 / 响应式 / 可访问性

- **i18n**：en / zh / ja 全量 key（`useT`）；按钮沿用 Category Reserve 的 `min-w` 档位
  （default `min-w-20`、sm `min-w-16`）；页面根容器套 `LocaleFade`。
- **主题**：anthropic / moonshot 双主题仅用 CSS 变量，自动跟随全局主题。
- **响应式**：`< lg` 会话列表收为抽屉（顶部按钮唤出）；参数抽屉窄屏全宽；对话气泡 `min-w-0` 防长单词撑破。
- **截断**：会话标题等 `truncate` 元素按 lint 规则配 Tooltip；恒定短占位（`—`）不加截断类。
- **键盘与可访问性**：Enter 发送 / Shift+Enter 换行 / Esc 取消编辑；icon-only 按钮带 `aria-label`；
  操作按钮为真实 `<button>` 且有可见 focus 态；流式输出区域 `aria-live="polite"`。

## 10. 测试与验证

- **vitest**（`playground-logic.test.ts`）：
  - `parseSSELine`：内容增量 / usage 尾帧 / `[DONE]` / 畸形 JSON / 非 `data:` 行；
  - `estimateCost`：默认规则命中、cached 拆分计价、未计价 `—`、空 rules；
  - 会话存储：写读回环、50 条淘汰最旧、未知版本降级、标题截取；
  - `buildChatBody` 既有用例回归 + `stream_options` 携带。
- **验证三件套**：`npm run lint && npm run test && npm run build`（按约定不起本地 dev server）。
- **开发流程**：`.worktrees/` 下 `feature/playground-chat-redesign-2026-10-09` 分支，
  worktree 内软链主工作区 `node_modules`（禁止 `npm ci`），完成后清理 `web/.next` / `web/out`。

## 11. 风险与开放问题

| 风险 | 影响 | 对策 |
|---|---|---|
| 上游不支持 `stream_options.include_usage` | 流式拿不到 usage | 指标行退化只显耗时（已设计）；不视为缺陷 |
| 长回复逐帧 markdown 重渲染卡顿 | 流式体验差 | rAF 节流 + 气泡内容 `min-w-0`；必要时流式期降级纯文本、结束再渲染 markdown |
| localStorage 配额满 | 会话丢失 | 写入失败降级内存态 + toast；50 条会话上限 |
| 编辑截断重跑误操作 | 丢上下文 | 截断前展示「其后 N 条将被移除」警示；v1 不做撤销（YAGNI） |
