# 会话详情页与分享页重设计

## 1. 目标

重设计已登录会话详情页（`/web/sessions/detail/?id=xxx`）与公开分享页（`/web/share/?id=xxx`），使其布局和视觉风格贴近 claude.ai 的对话阅读体验：

- 消息使用单列居中阅读区。
- 助手消息靠左，带 Claude 风格头像/圆点。
- 用户消息靠右，使用柔和圆角气泡。
- 顶部 chrome 极简化；导航与会话历史统一放在左侧边栏。

## 2. 设计方向

- **桌面端**：dashboard 左侧边栏改为**组合边栏**——上半部分为全局导航，中间用分隔线，下半部分为会话历史。
- **移动端**：保留当前全屏阅读视图（本身已接近 Claude 风格），在顶部 sticky header 增加 "History" 入口，点击后从底部弹出可搜索、可无限滚动的会话列表 sheet。
- **分享页**：使用同样的居中阅读列布局。公开页面不显示全局导航，侧边栏仅展示分享会话的元数据。

## 3. 桌面端布局

```
┌─────────────────────────────────────────────────────────────┐
│  Aris Proxy      │  Session #42                             │
│  ─────────────   │                                          │
│  Dashboard       │  ┌──────────────────────────────────┐   │
│  Sessions    ✓   │  │ ✦ 助手回复内容...                │   │
│  Shares          │  │                                  │   │
│  API Keys        │  │         ┌──────────────┐         │   │
│  Profile         │  │         │ 用户消息     │         │   │
│  ─────────────   │  │         └──────────────┘         │   │
│  History         │  │                                  │   │
│  Session #42 ✓   │  │ ✦ 助手回复内容...                │   │
│  Session #41     │  └──────────────────────────────────┘   │
│  Session #40     │                                          │
│  …               │                                          │
│  [loading…]      │                                          │
└─────────────────────────────────────────────────────────────┘
```

- **边栏宽度**：`w-64`（256 px），与现有 dashboard sidebar 一致。
- **全局导航**：保留现有导航项；当前所在项（`Sessions`）高亮。
- **分隔线**：在全局导航与历史之间放置 `Separator`。
- **历史区域**：
  - 标题：小字大写、加宽字距的灰度 "History"。
  - 每个条目显示会话摘要（或 `Session #{id}`）、消息数、相对时间。
  - 当前会话使用卡片表面高亮。
  - 历史区域顶部放置搜索输入框。
  - 使用 infinite scroll 分页。
- **主区域**：
  - 极窄顶栏：仅居中显示会话标题；右侧放置工具面板触发按钮。
  - 阅读列最大宽度 `max-w-3xl`，居中。
  - 工具面板改为**右侧悬浮抽屉**（slide-out drawer），点击顶栏工具按钮后从右侧滑出，覆盖在阅读列上方，不占用固定布局宽度。

## 4. 移动端布局

- 保留现有全屏、安全区域感知的阅读视图。
- 在 sticky header 左侧增加 **History** 按钮（位于现有工具/分享/删除操作左侧）。
- 点击后打开底部 sheet，内部为可搜索、无限滚动的会话列表。
- 该 sheet 复用现有工具面板同款的 `SwipeDismissSheetBody` 组件。

## 5. 分享页

- 公开分享页同样使用居中阅读列。
- 侧边栏**不显示全局导航**，也**不显示完整会话历史**，仅展示：
  - 紧凑的 "Shared session" 头部。
  - 被分享会话的标题与创建时间。
- 顶栏显示 "Shared session #{id}" 与只读元数据。

## 6. 组件与文件改动

| 文件 | 改动 |
|------|------|
| `web/src/app/(dashboard)/sessions/detail/page.tsx` | 保留 Suspense 包装；按需向下传递会话列表数据。 |
| `web/src/components/session-detail/session-detail-client.tsx` | 大幅重构：组合边栏布局、历史列表集成、搜索 + 无限滚动。 |
| `web/src/components/session-detail/session-history-list.tsx` | **新增**：可复用的历史列表，含搜索与 infinite scroll。 |
| `web/src/components/session-detail/session-history-sheet.tsx` | **新增**：移动端历史列表 bottom sheet 包装。 |
| `web/src/components/chat/chat-message.tsx` | 打磨助手/用户消息样式，贴近 Claude 风格（头像、气泡形状、元信息位置）。 |
| `web/src/app/share/page.tsx` | 适配组合边栏/阅读列布局；隐藏全局导航；工具面板改为右侧抽屉。 |
| `web/src/components/share/share-dialog.tsx` | 无布局改动，保持现状。 |
| `web/src/lib/types.ts` | 复用现有 `SessionSummary` 作为历史条目类型，无需新增类型。 |
| `web/src/components/ui/sheet.tsx` | 若现有 sheet 组件不支持右侧抽屉的宽度/动画需求，则微调样式。 |

## 7. 视觉 token

- **助手头像**：`rounded-md` 方形，`bg-primary/15 text-primary`（复用现有），尺寸 7（28 px）。
- **用户气泡**：`bg-secondary/80`，`rounded-3xl rounded-tr-md`，桌面端最大宽度 80 %，移动端 85 %。
- **阅读列**：`max-w-3xl`，居中，gutter 为 `px-4 sm:px-6`。
- **顶栏**：`h-12`，`border-b border-border/60`，标题居中，文字 `text-sm font-medium`。
- **侧边栏历史条目**：`rounded-lg`，`px-3 py-2.5`；当前项使用 `bg-card border border-border/70`。
- **搜索输入**：高度较小（`h-8`），历史区域内全宽，placeholder 为 "Search sessions…"。
- **分隔线**：`Separator`，`my-2`。

## 8. 交互

- **历史搜索**：300 ms debounce。空查询加载默认最近会话；非空查询调用 `api.listSessions({ keyword })`。
- **历史无限滚动**：`IntersectionObserver` sentinel 距底部 200 px 触发；每页 20 条。
- **历史条目点击**：跳转至 `/web/sessions/detail/?id={id}`（分享页对应跳转）。
- **当前会话**：不可点击，视觉高亮。
- **工具面板**：开关按钮位于顶栏右侧；桌面端为右侧悬浮抽屉，移动端保持底部 sheet。
- **分享/删除操作**：集中在顶栏右上角；移动端仅图标，桌面端空间允许时显示图标+文字。

## 9. 数据流

- `SessionDetailClient` 拉取：
  - 会话元数据（`api.getSessionMetadata`）。
  - 消息（`useInfiniteList` + `api.listSessionMessages`）。
  - 工具（`useInfiniteList` + `api.listSessionTools`）。
  - 会话历史（`useInfiniteList` + `api.listSessions`，带 `keyword`）。
- `SessionHistoryList` 接收：
  - `sessions`、`loading`、`hasMore`、`loadMore`、`keyword`、`onKeywordChange`、`currentSessionId`。
- 公开分享页仅拉取元数据、消息、工具（无已登录历史）。

## 10. 已确认决策

1. 历史条目不需要按日期分组；v1 保持平铺列表。
2. 桌面端工具面板采用**右侧悬浮抽屉**，不占用固定布局宽度；移动端保持底部 sheet。
3. 分享页侧边栏仅显示被分享会话的元数据，不展示全局导航与完整历史。

## 11. 验收标准

- 桌面端会话详情页符合组合边栏线框。
- 移动端会话详情保留现有阅读体验，并新增历史 sheet。
- 公开分享页呈现为精简版 Claude 对话视图。
- `cd web && npm run lint && npm run build` 通过。
- 现有 sessions/share E2E 测试通过（或按新选择器更新）。
