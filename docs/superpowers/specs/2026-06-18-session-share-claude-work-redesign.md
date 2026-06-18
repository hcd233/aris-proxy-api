# Session 详情 / 分享页 Claude Work 风格重构设计

> 日期：2026-06-18
> 范围：Web 前端 session 详情页、公开分享页及其共享组件的视觉与布局重构
> 参考对象：Claude Work（claude.ai）的页面布局与消息渲染样式

---

## 1. 背景与动机

当前 session 详情页（`web/src/app/(dashboard)/sessions/detail/page.tsx` + `web/src/components/session-detail/session-detail-client.tsx`，968 行）和公开分享页（`web/src/app/share/page.tsx`，641 行）已在注释中声称模仿 claude.ai，但在以下三方面与 Claude Work 的实际观感有明显差距：

1. **视觉气质不对**：暖色 token 已就绪（`globals.css` 定义 `#FAF8F5` 暖白 / `#D97757` Claude 橙 / Source Serif 4 标题字体），但未被充分用好；顶栏操作拥挤、留白节奏与 Claude Work 不一致。
2. **布局结构不对**：桌面端顶栏把评分/分享/删除/工具等操作平铺成一排按钮，视觉噪音大；分享页桌面端用左侧元数据栏，与 session 详情页风格不统一。
3. **消息渲染细节差**：每条消息上方 "YOU"/"AI" 大写文字标签冗余；用户气泡用 `bg-muted` 偏冷；thinking 块带灰背景过重；工具卡片橙色边框过抢眼且折叠态无参数预览；代码块圆角偏小、背景过深；整体行距 1.7、消息间距 `space-y-7` 偏松。

目标：**完全从头重构**这两个页面及其共享消息渲染组件，复刻 Claude Work 美观、简洁、优雅的特点。

## 2. 非目标

- 不改后端 API、DTO、数据模型。
- 不改路由结构（`/sessions/detail?id=`、`/share/?id=` 保持不变）。
- 不改 dashboard 的左栏导航与 session 历史侧栏（已就绪，且自动在 session 详情时展开）。
- 不改 `ShareDialog`（创建分享链接的弹窗，功能完备，本次只调整触发它的顶栏按钮样式）。
- 不引入新依赖；`react-markdown` / `remark-gfm` / `rehype-highlight` / `mermaid` 引擎保留。

## 3. 设计决策（已与用户确认）

| # | 决策点 | 选择 |
|---|--------|------|
| D1 | 桌面端 session 详情布局 | **B2**：极简顶栏（返回 + 标题 + 消息数 + 内联星标 + 分享 + 删除 + 工具按钮）+ 居中阅读栏 ~680px + 可隐藏右栏（工具列表） |
| D2 | 评分控件呈现 | **内联星标**：5 颗小星，未评分淡色、hover 变橙、已评分前 N 颗亮橙 + 尾部 × 清除 |
| D3 | 评分提交确认 | **行内气泡确认**：第一次点星 → 该星高亮 + 旁边浮出 "Rate N? Yes/No" → 点 Yes 才提交（沿用现有 `scoreConfirmValue` 逻辑） |
| D4 | 工具面板 | 桌面端：可隐藏右栏，工具按钮点击切换展开/收起；移动端：保留 iOS bottom sheet |
| D5 | 移动端 session 详情 | 保留 iOS 风格 sticky header（背板模糊、滚动收缩）+ bottom sheet，顶栏内容换成 B2 的星标/分享/删除 |
| D6 | 分享页桌面端 | 与 session 详情页统一：极简顶栏（标题 + 创建时间/消息数 + 工具按钮）+ 居中阅读栏 + 可隐藏右栏；**去掉左侧元数据栏** |
| D7 | 分享页移动端 | 保留现有 iOS 风格（sticky header + bottom sheet），顶栏内容简化 |
| D8 | 消息渲染 6 项改进 | 全部采纳（见 §5） |

## 4. 架构与文件结构

### 4.1 当前结构问题

- `session-detail-client.tsx`（968 行）把桌面/移动两套布局 + 评分逻辑 + 工具列表 + 删除确认全揉在一个文件，维护困难。
- `share/page.tsx`（641 行）重复了 session 详情页的大部分布局逻辑，两份代码风格漂移风险高。
- `chat-message.tsx`（552 行）混合了内容提取、多模态渲染、reasoning、tool call、avatar、meta 等多个职责。

### 4.2 新组件结构

拆分为更小、单一职责的组件，两页共享。布局统一由 `shared/reading-layout.tsx` 负责，不再在 session-detail 下放独立的 desktop/mobile layout 文件——`ReadingLayout` 内部根据 `useIsMobile()` 自行切换桌面/移动两套骨架，调用方只传 slot 内容。

```
web/src/components/
├── chat/                          # 消息渲染（共享）
│   ├── chat-message.tsx           # 入口 + 角色分发（重写，~120 行）+ re-export buildToolResultsByID
│   ├── content-extract.ts         # extractContent / imageURLOf / buildToolResultsByID（提取，纯函数）
│   ├── multimodal-parts.tsx       # 图片/音频/文件/refusal 渲染（提取）
│   ├── reasoning-block.tsx        # thinking 折叠块（提取 + 样式重做）
│   ├── tool-call-card.tsx         # 工具调用卡片（提取 + 样式重做 + 参数预览）
│   ├── system-message.tsx         # system 消息（提取）
│   ├── user-message.tsx           # user 气泡（提取 + 样式重做）
│   ├── assistant-message.tsx      # assistant prose（提取 + 样式重做）
│   └── markdown-lite.tsx          # markdown 引擎（保留，微调代码块样式）
├── shared/                        # 两页共享的布局骨架
│   └── reading-layout.tsx         # 桌面 sticky 顶栏 + 阅读栏 + 可隐藏右栏；移动 iOS sticky header + bottom sheet
├── session-detail/                # session 详情页专用
│   ├── session-detail-client.tsx  # 入口 + 数据加载 + 传 slot 给 ReadingLayout（精简到 ~200 行）
│   ├── score-stars.tsx            # 内联星标评分 + 行内气泡确认（新，共享）
│   ├── tools-rail.tsx             # 桌面端可隐藏右栏内容（标题 + 关闭 + 工具列表 slot），作为 ReadingLayout 的 toolsPanel 传入
│   ├── tool-sidebar-item.tsx      # 工具列表项（从 session-detail-client 提取）
│   ├── collapsible-text.tsx       # Show more/less（从 session-detail-client 提取）
│   ├── session-history-sheet.tsx  # 保留
│   ├── session-history-sidebar.tsx# 保留
│   ├── session-history-list.tsx   # 保留
│   └── swipe-dismiss-sheet-body.tsx # 保留
└── share/
    └── share-dialog.tsx           # 保留

web/src/app/
├── (dashboard)/sessions/detail/page.tsx  # 不变
└── share/page.tsx                        # 精简：数据加载 + 传 slot 给 ReadingLayout
```

**删除的旧文件**：`tool-drawer.tsx`（被 `tools-rail.tsx` + `ReadingLayout` 的右栏 slot 取代）。

### 4.3 ReadingLayout 的 slot 契约

`reading-layout.tsx` 暴露以下 props（用 ReactNode slot，不强切三段）：

| prop | 类型 | 说明 |
|------|------|------|
| `header` | `ReactNode` | 顶栏整体内容（调用方自由编排返回/标题/操作组） |
| `children` | `ReactNode` | 阅读栏内容（消息列表） |
| `toolsPanel` | `ReactNode` | 工具面板内容（桌面注入右栏，移动注入 bottom sheet） |
| `toolsOpen` | `boolean` | 工具面板是否展开 |
| `onToolsOpenChange` | `(open: boolean) => void` | 工具面板展开/收起回调 |
| `toolsCount` | `number` | 工具数量（>0 时才渲染工具面板容器） |
| `headerCompact` | `boolean` | 移动端顶栏是否收缩（由调用方 IntersectionObserver 驱动） |
| `messagesScrollRootRef` | `Ref<HTMLDivElement>` | 消息滚动容器 ref（移动端需要，桌面用 window 滚动） |
| `onMessagesScroll` | `(e: UIEvent<HTMLDivElement>) => void` | 消息滚动回调（无限加载） |

`ReadingLayout` 内部职责：
- 桌面端：sticky 顶栏（`bg-background/95 backdrop-blur border-b`）+ `max-w-[680px]` 阅读栏 + 右栏（`toolsOpen` 控制 `width: 0 ↔ 280px`，`transition-[width] duration-200`）
- 移动端：iOS 风格 sticky header（背板模糊、`headerCompact` 驱动收缩动画）+ 阅读栏 + bottom sheet（`Sheet side="bottom"` + `SwipeDismissSheetBody`，`toolsOpen` 控制）
- 顶栏负边距抵消 dashboard padding 的策略由 `ReadingLayout` 内部处理，调用方无需关心

## 5. 消息渲染改进细节（D8）

### 5.1 去掉 YOU/AI 文字标签

**当前**：`MetaLine` 在每条消息上方渲染 `YOU` / `AI` 大写字母标签 + model + time。

**改进**：
- user 消息：无标签，仅靠右对齐的暖色气泡区分。
- assistant 消息：无文字标签，用头像（橙色圆点 + provider icon）区分；时间移到头像下方，小号 muted 文字。
- system 消息：保留 "System" 标签（因为 system 消息无头像，需要文字标识）。

### 5.2 用户气泡更暖更圆润

**当前**：`rounded-2xl rounded-br-sm bg-muted px-4 py-3`

**改进**：`rounded-[20px] rounded-br-[6px] bg-accent px-5 py-3.5 text-[15px] leading-[1.6]`
- `bg-muted`（`#F5F0EB`）→ `bg-accent`（`#F5E6E0` 暖粉），更暖
- 圆角 16px → 20px，右下角 4px → 6px，更圆润
- 内边距略增（`px-5` = 20px），行距 1.7 → 1.6

### 5.3 Thinking 块更轻

**当前**：`border-l-2 border-primary/30 bg-muted/30 px-4 py-3`，带灰背景。

**改进**：`border-l-2 border-border pl-4 pr-2 py-2`，**透明背景**，仅左侧细线，圆角 `rounded-r-md`。
- 折叠按钮保留 Brain 图标 + "Thought process"，hover 时 `bg-muted/40`（极轻反馈）
- 展开内容 `text-[13.5px] italic leading-[1.55] text-muted-foreground`

### 5.4 工具卡片紧凑 + 参数预览

**当前**：`border border-primary/25 bg-primary/[0.04]`，橙色边框过抢眼；折叠态只显示工具名。

**改进**：
- 边框 `border border-border`（中性），背景 `bg-card`，圆角 `rounded-lg`
- 图标容器 28px → 24px，整体 padding 收紧
- **折叠态显示一行参数预览**：从 `call.arguments`（JSON）提取第一个 key-value，用 `font-mono text-[11px] text-muted-foreground` 截断显示，如 `path: "internal/handler/stream.go"`
- 展开态保留 Input/Output 两段，样式用 `bg-muted/40`（比当前 `bg-muted/60` 更轻）

### 5.5 代码块更暖

**当前**：`bg-[#1F1A14]`，圆角 `rounded-lg`（8px），无外边框。

**改进**：`bg-[#26211C]`（稍提亮），圆角 `rounded-xl`（12px），外边框 `border border-[#3A322B]`（暖色边框）。
- 语言标签 `text-white/40` → `text-[#E8DDD3]/35`，更柔
- 代码文字 `text-[#E8DDD3]` 保留，行距 `leading-relaxed` → `leading-[1.55]`

### 5.6 间距/行距节奏更紧凑

**当前**：消息间距 `space-y-7`（桌面）/ `space-y-6`（移动），正文 `leading-[1.7]`。

**改进**：
- 桌面消息间距 `space-y-5`，移动 `space-y-4`
- 正文 `leading-[1.6]`（markdown 容器 + user 气泡 + assistant prose 统一）
- 阅读栏内边距 `py-8` → `py-6`，顶部 `pt-5` 保留

## 6. 桌面端布局规格（B2）

### 6.1 顶栏（sticky）

```
[← 返回]  Session #128  24 messages          [★★★★☆ ×] [分享] [🗑] [🔧3 ▸]
```

- 高度：`pt-[calc(2rem+0.25rem)] pb-3`（保留现有负边距抵消 dashboard padding 的策略）
- 背景：`bg-background/95 supports-[backdrop-filter]:backdrop-blur`，底部 `border-b border-border/70`
- 左侧：返回按钮（ghost icon）+ 标题（Source Serif 4，15px，semibold）+ 消息数（11px muted）
- 右侧操作组（gap-1）：
  - **评分**：`ScoreStars` 组件，5 颗 11px 小星；未评分 `text-muted-foreground/30 hover:text-primary`；已评分前 N 颗 `text-primary` + 尾部 `×` 清除按钮；点击触发行内气泡确认
  - **分享**：`Share2` 图标按钮，已分享时 `variant="secondary"` 高亮
  - **删除**：`Trash2` ghost icon，hover `text-destructive`
  - **工具**：`Wrench` 图标 + 数量徽标，点击切换右栏展开/收起；收起时 `variant="outline"`，展开时 `bg-secondary` 高亮 + 箭头由 ▸ 变 ◂

### 6.2 阅读栏

- 容器：`mx-auto w-full max-w-[680px] px-4 py-6 sm:px-6`
- 消息列表：`space-y-5`，每条消息用 `ChatMessage` 渲染
- 底部：滚动加载 sentinel + "end of conversation" 标记（保留现有样式）

### 6.3 可隐藏右栏（ToolsRail）

- 固定宽度 280px，`border-l border-border/70 bg-card`
- 收起时 `width: 0`，展开时 `width: 280px`，过渡 `transition-[width] duration-200 ease-out`
- 内容：标题行 "Available Tools (N)" + 关闭按钮 + 工具列表（`ToolSidebarItem`）+ 滚动加载 sentinel
- 工具列表仅在右栏展开时加载（保留现有 `toolsListEnabled` 逻辑）

## 7. 移动端布局规格

### 7.1 session 详情页

保留现有 iOS 风格 sticky header（背板模糊、滚动收缩、`headerCompact` 状态），顶栏内容调整为 B2：

```
[←] [History]  Session #128              [★★★★☆ ×] [分享] [🗑] [🔧3]
               24 messages · gpt-4o · ★4
```

- 返回 + History（保留 `SessionHistorySheet`）
- 中间标题区（滚动收缩时副标题折叠）
- 评分星标（尺寸略小，9px）
- 分享/删除/工具按钮（icon-sm）
- 工具点击打开 bottom sheet（保留 `SwipeDismissSheetBody`）

### 7.2 分享页

保留现有 iOS 风格，顶栏：

```
[Share图标]  Shared session #128          [🔧3]
             2h ago · 24 messages
```

- 无返回/评分/删除（只读页）
- 工具点击打开 bottom sheet

## 8. 数据流与逻辑保留

以下逻辑从现有代码原样迁移到新组件，不改动：

- `useInfiniteList` for messages / tools 的分页加载
- `toolsListEnabled` 条件（仅 metadata 加载完成且右栏/sheet 打开时加载工具）
- `buildToolResultsByID` 工具结果匹配
- `IntersectionObserver` 滚动加载 sentinel
- `handleScore` / `handleDeleteScore` 评分 API 调用
- `handleDelete` 删除 API 调用
- `ShareDialog` 分享链接创建
- 分享页的 `ShareError` 错误状态机（missing-id / rate-limited / not-found / unknown）
- 分享页的 IP 限流处理（404/429）

## 9. 验证计划

- **类型检查**：`cd web && npm run lint && npm run build`（AGENTS.md §12.4 要求）
- **视觉验证**：本地启动后端 `go run main.go server start --host localhost --port 8080` + 前端 `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev`，用 Chrome MCP 访问以下页面对比 Claude Work：
  1. `/web/sessions/` 列表页 → 点进一个有工具调用的 session 详情页
  2. `/web/share/?id=<uuid>` 分享页
  3. 移动端视口（375px）下的两个页面
- **回归**：评分、删除、分享创建、工具列表展开/滚动加载、消息无限滚动、reasoning 折叠、tool call 展开/折叠、代码块复制、mermaid 渲染
- **无单元测试**：Web 前端当前无强制测试框架（AGENTS.md §12.4），以 lint + build + 人工视觉验证为准

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 拆分组件后 props 传递层级深 | `reading-layout.tsx` 用 children slot 而非 props 链；数据加载逻辑留在 page 级组件 |
| 可隐藏右栏与 dashboard 左栏的负边距冲突 | 右栏在 `main` 区域内部，不受 dashboard 左栏影响；负边距策略仅用于顶栏抵消 padding |
| 移动端 `headerCompact` IntersectionObserver 在新组件中失效 | observer 绑定逻辑随 `mobile-layout.tsx` 一起迁移，sentinel ref 透传 |
| `chat-message.tsx` 拆分后 `buildToolResultsByID` 导出路径变化 | 保持从 `chat/chat-message.tsx` re-export，调用方无需改 import |
| 暗色模式下新样式（`bg-accent` / `bg-card` / 代码块暖色边框）观感 | `globals.css` 暗色 token 已定义对应变量，构建后用 Chrome MCP 暗色模式验证 |

## 11. 实现顺序建议

1. 提取共享组件：`content-extract.ts`、`multimodal-parts.tsx`、`reasoning-block.tsx`、`tool-call-card.tsx`、`system-message.tsx`、`user-message.tsx`、`assistant-message.tsx`，`chat-message.tsx` 改为薄入口 + re-export `buildToolResultsByID`
2. 改 `markdown-lite.tsx` 代码块样式（§5.5）
3. 新建 `score-stars.tsx`（内联星标 + 行内气泡确认）
4. 新建 `reading-layout.tsx`（共享桌面/移动布局骨架）
5. 新建 `tools-rail.tsx`（桌面可隐藏右栏）
6. 重写 `session-detail-client.tsx`：用 `ReadingLayout` + `ScoreStars` + `ToolsRail` 组合
7. 重写 `share/page.tsx`：用 `ReadingLayout` 组合，去掉左侧元数据栏
8. 提取 `tool-sidebar-item.tsx`、`collapsible-text.tsx`
9. `npm run lint && npm run build` 验证
10. 本地启动 + Chrome MCP 视觉验证 + 回归测试
