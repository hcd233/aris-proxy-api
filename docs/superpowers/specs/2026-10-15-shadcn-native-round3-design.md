# Web 端自研组件替换为 shadcn 原生组件（第三轮）设计

- 日期：2026-10-15
- 分支：`refactor/shadcn-native-round-three-2026-10-15`（worktree `.worktrees/shadcn-native-round3`）
- 前序：#192（Checkbox/ToggleGroup/InputGroup/Empty/Slider/Collapsible）、#193（Pagination/Combobox/Sidebar）

## 1. 目标与范围

把 `web/src` 里仍在手写、且 shadcn 官方 registry 已有对应组件的实现，替换为 shadcn 原生组件。范围是盘点报告的第一节（可直接替换）与第二节（需改交互）全部条目。

不在范围内：

- `score-dots`（悬停预览填充需自写，换 ToggleGroup 收益低）、dataset 步骤条（shadcn 无 Stepper）、`page-header`。
- 动效基础设施（`motion/*`、`locale-fade`）、品牌图标、终端装饰圆点、monitor 实时状态点。
- 分页「上一页 / 下一页」：`PaginationPrevious` / `PaginationNext` 渲染为 `<a>`，适用于 href 导航；本项目按钮驱动翻页，保持 `Button`。

## 2. 总原则

1. **外观跟随 shadcn 默认**：只用组件自带变体。例外只有两类：
   - 语义色 Badge（trigger 动作 deny / omit / capture、upstream 告警等）用 `className` 保留语义色，否则信息丢失；
   - 布局类 className（宽度、间距、栅格、截断）。
2. **不覆盖已安装组件**：`button.tsx`、`separator.tsx`、`label.tsx` 不跟随上游覆盖。separator、label 与上游只有格式差异；button 上游会删掉 i18n 宽度预留（`min-w-*`）与动效钩子 `data-variant`。
3. **依赖最小化**：
   - 生成的组件里 `import { cn } from "cn"` 一律改写为 `@/lib/utils`，不引入 npm 包 `cn`；
   - 只新增运行时依赖 `@shadcn/react`（MessageScroller 的 headless 实现，0.3.x，约 56KB，peer 仅 react ≥19）；
   - 修改 lock 后用 `npx npm@10 ci --dry-run` 做 CI parity 校验。
4. **动效契约**（`docs/agents/web-frontend.md`「动效契约」）：
   - 新组件删除生成代码中的 tw-animate 进出场类（`data-open:animate-in`、`fade-in-0`、`zoom-in-95`、`slide-in-from-*` 等）；
   - 在 `motion-anthropic.css` 与 `motion-moonshot.css` 中按 `data-slot` 各补一组规则（alert、item、progress-indicator、drawer-overlay、message-scroller-button 等）。比例条浮层内容不可交互，用已装的 Tooltip 即可，不引入 HoverCard；
   - **Drawer 例外**：其 transform 过渡由拖拽手势驱动（`--drawer-swipe-movement-y` 等变量），保留组件自带 transition，语法层只覆盖 overlay 与内容透明度，不覆盖 popup 的 transform。
5. **截断规则**：`truncate` / `line-clamp-1` 仍须处于 `TooltipTrigger` 渲染子树内（eslint `truncate-requires-tooltip`）。shadcn 组件内置的截断（`AttachmentTitle`、`AttachmentDescription`）位于 `components/ui/` 内部，调用方传入的长文本照常包 Tooltip。

## 3. 新增组件

通过 `./node_modules/.bin/shadcn add <name>` 生成（拒绝覆盖已有文件），随后：

- 改写 `cn` 导入；
- 删除 tw-animate 类；
- 按仓库 Prettier 格式化。

| 组件 | 用途 |
|---|---|
| spinner | 加载圈 |
| progress | 排行条、导出进度 |
| alert | 错误 / 拒答提示块 |
| item | 移动端卡片列表、会话列表 |
| field | 表单布局、带文字分隔线 |
| scroll-area | 代码 / JSON 预览、侧栏滚动区 |
| drawer | 工具底部弹层 |
| message-scroller | playground 对话区 |
| message / bubble | 聊天消息 |
| attachment | 多模态附件 |
| marker | 系统消息头 |

`ui/scroll-area.tsx` 在生成版基础上同时挂横向与纵向两个 `ScrollBar`。Base UI 在内容不溢出时不渲染滚动条，对纵向场景无副作用；trace 里不换行的 JSON 依赖横向滚动。

## 4. 替换清单（按提交分档）

### ① 反馈类

| 现状 | 位置 | 替换 |
|---|---|---|
| `Loader2 + animate-spin` | `app/callback/page.tsx`、`app/(dashboard)/dataset/page.tsx`、`upstream/model-dialog.tsx`、`upstream/pricing-dialog.tsx`、`components/filter-bar/filter-bar.tsx`、`components/share/share-dialog.tsx` | `Spinner`；按钮内用 `data-icon="inline-start"` |
| 手写进度条 | `dataset/page.tsx`（分布排行条、导出进度） | `Progress` |
| 错误 / 拒答块 | `app/login/page.tsx`、`chat/assistant-message.tsx` 与 `chat/multimodal-parts.tsx` 的 PartRefusal、`playground/chat-turn.tsx` 错误卡、`chat/markdown-lite.tsx` mermaid 渲染错误、`audit/model/page.tsx` 错误详情 | `Alert variant="destructive"` + `AlertTitle` / `AlertDescription`；重试按钮放 `AlertAction` |
| 手写标签 span | `upstream/shared.tsx`（4 处）、`trigger/page.tsx` 的 `ActionBadge`、`session-detail-client.tsx` 与 `share/page.tsx` 的工具数角标、`playground/chat-turn.tsx` 角色标签 | `Badge`；`ActionBadge` 用 `Badge render={<button/>}`，保留 deny / omit / capture 语义色 |
| 分隔线 | `login/page.tsx` 「或」分隔（两个 `h-px` span）、`markdown-lite.tsx` 的 `<hr>` | `FieldSeparator`（带文字）/ `Separator` |
| 原生 `<table>` | `charts/model-cost-bar-chart.tsx`、`charts/model-token-bar-chart.tsx`、`markdown-lite.tsx` GFM 表格 | `Table` 系列 |

### ② 表单与列表

| 现状 | 位置 | 替换 |
|---|---|---|
| `div.space-y-2 > Label + 控件`、`<label>` 包 Checkbox / Switch | `upstream/endpoint-dialog.tsx`、`upstream/model-dialog.tsx`、`upstream/pricing-editor.tsx`、`cron/schedule-editor.tsx`、`demo-config-card.tsx`、`profile/page.tsx`、`share/share-dialog.tsx` | `FieldGroup` / `Field` / `FieldLabel` / `FieldDescription`；勾选组用 `FieldSet` + `FieldLegend`；Switch / Checkbox 行用 `Field orientation="horizontal"`；demo 模块卡片用 `FieldLabel` 包 `Field` |
| 移动端卡片列表 | apikeys、sessions、trace、trigger、users、audit/model 六页的 `isMobile` 分支 | `ItemGroup` + `Item variant="outline"`（`ItemContent` / `ItemTitle` / `ItemDescription` / `ItemActions`） |
| 会话列表 | `session-detail/session-history-list.tsx`、`playground/session-sidebar.tsx` | `Item`（`render={<button/>}`，选中态 `variant="muted"`） |
| 手写滚动区 | `trace-detail-client.tsx`（6 处 pre）、`dataset/page.tsx` 格式预览、`reading-layout.tsx` 桌面工具栏、`session-history-list.tsx`、`playground/session-sidebar.tsx`、图表卡片表格容器 | `ScrollArea` |

不改：dashboard 主内容区与 ReadingLayout 的消息滚动容器（`messagesScrollRootRef` 被吸顶头部压缩、工具定位依赖为原生滚动元素）。

### ③ 组合修正

| 现状 | 替换 |
|---|---|
| `share-dialog.tsx` 的 `PopoverTrigger` 内又套原生 `<button>`（按钮嵌套，HTML 不合法） | `PopoverTrigger render={<Button variant="outline"/>}` |
| `pagination-bar.tsx` 每页条数：手画 ✓ + `ml-6` 占位 | `DropdownMenuRadioGroup` / `DropdownMenuRadioItem` |
| `language-switcher.tsx` 手写选中态 + 模板字符串拼 class | `DropdownMenuRadioGroup`；触发器用 `Button variant="ghost" size="xs"` |
| `permission-guard.tsx` 的 `GuardState` 手写卡片 | `Empty` |
| `(dashboard)/layout.tsx` 侧栏分组标题为自定义样式的 `CollapsibleTrigger` | `SidebarGroupLabel render={<CollapsibleTrigger/>}` |
| 复制按钮重复实现：`markdown-lite.tsx`、`upstream/grouped-view.tsx`、`trace-install-popover.tsx` | 统一用 `CopyButton`，其内部改为 `Button variant="ghost" size="icon-xs"/"xs"` |
| 排序表头 `upstream/flat-view.tsx`、分组折叠 `upstream/grouped-view.tsx` 的裸 button | `Button variant="ghost" size="xs"` / `"icon-xs"` |
| `reading-layout.tsx` 桌面工具栏关闭按钮（✕ 字符） | `Button variant="ghost" size="icon-sm"` + `XIcon` |
| `charts/stacked-ratio-bar.tsx` 的 `createPortal` + 手算坐标浮层 | 每段 `TooltipRoot` / `TooltipTrigger render={<div/>}` / `TooltipContent`，内容沿用全段明细列表；删除定位代码 |

### ④ filter-bar 改用 Combobox

保持 `filter-dsl.ts`、`use-filter-bar.ts`、`FilterBarProps` 不变，只重写 `filter-bar.tsx` 的渲染层。

- **根**：`Combobox`，单选，`value={null}` 恒定；`onValueChange(row)` 调用现有 `pick(row)`。
- **输入**：`inputValue` / `onInputValueChange` 受控（选项按下时 `eventDetails.isItemPress` 取消默认清空，由 `pick` 自行决定下一步输入值）；`filter={null}`，`items={rows}`，候选仍由现有 `useMemo` 计算。
- **输入框**：`ComboboxPrimitive.Input` 渲染进现有容器，容器作为 `anchor`（`useComboboxAnchor`），弹层宽度对齐输入容器。
- **弹层**：`ComboboxContent` + `ComboboxList` + `ComboboxItem`；标题行用 `ComboboxLabel`；加载态用 `Spinner`；无选项用 `ComboboxEmpty`。
- **已选标签**：继续手动渲染，改为 `Badge variant="outline"` + `Button variant="ghost" size="icon-xs"` 移除；自由关键词保留虚线边框区分（布局类 className）。
- **键盘**：方向键 / 回车 / Esc / 点击外部由 Base UI 接管。保留：
  - 退格删最后一个标签；
  - 输入框非空且无候选时回车生成关键词标签；
  - Tab 选中高亮项（`onKeyDown` 读 `onItemHighlighted` 记录的高亮值）。
- 不用 `ComboboxChips`：它要求 multiple 模式，与「先选字段、再选值」的两步交互冲突。

### ⑤ 底部弹层改用 Drawer

- 删除 `session-detail/swipe-dismiss-sheet-body.tsx`。
- `reading-layout.tsx` 移动端改为 `Drawer`（`swipeDirection="down"`、`showSwipeHandle`）+ `DrawerContent` + `DrawerHeader`（`DrawerTitle` 含工具图标与数量 Badge，右侧 `DrawerClose render={<Button variant="ghost" size="sm"/>}`「完成」）+ `ScrollArea` 内容区。
- 外观用 Drawer 默认（最高 `100dvh - 6rem`、`rounded-t-xl`）。
- `onToolsScroll` / `onToolsScrollRootChange` 继续挂到内容滚动元素：ScrollArea 的 viewport 通过 `ScrollAreaPrimitive.Viewport` 的 ref 暴露。为此 `ui/scroll-area.tsx` 增加 `viewportRef` 与 `onViewportScroll` 两个透传属性，这是唯一的组件 API 扩展。

### ⑥ 聊天区

**`chat/*`（session 详情、分享页共用）**

| 组件 | 改法 |
|---|---|
| `user-message.tsx` | `Message align="end"` + `MessageContent` + `Bubble variant="secondary"` / `BubbleContent`；复制按钮放 `MessageFooter` |
| `assistant-message.tsx` | `Message` + `MessageAvatar`（模型图标）+ `MessageContent`；正文用 `Bubble variant="ghost"`，推理块、工具卡片保持在 `MessageContent` 内；时间放 `MessageHeader` |
| `system-message.tsx` | 消息头用 `Marker variant="separator"`（`MarkerContent` 含 System 标签、时间、复制按钮）；正文用 `Bubble variant="muted"`，展开 / 收起保持 Collapsible |
| `multimodal-parts.tsx` | 文件与音频用 `Attachment` + `AttachmentMedia` + `AttachmentContent` / `AttachmentTitle` / `AttachmentDescription`；图片用 `Attachment orientation="vertical"` + `AttachmentMedia variant="image"` 缩略图，外包 `<a target="_blank" rel="noopener noreferrer">` 打开原图（沿用 `isSafeUrl` 的 http/https 校验）；拒答用 `Alert destructive` |

**`playground/*`**

| 组件 | 改法 |
|---|---|
| `chat-turn.tsx` | 用户回合 `Message align="end"` + `Bubble variant="secondary"`；assistant 回合 `Message` + `MessageHeader`（模型图标 + 名）+ `Bubble variant="ghost"`（保留 `stream-body` 与 `data-streaming` / `data-settling`，签名时刻动效依赖）；指标行与操作条放 `MessageFooter` |
| `conversation-view.tsx` | `MessageScrollerProvider autoScroll defaultScrollPosition="end"` + `MessageScroller` + `Viewport` + `Content aria-busy={sending}` + 每回合 `MessageScrollerItem messageId={m.id} scrollAnchor={m.role === "user"}` + `MessageScrollerButton`；删除手写的 `followRef` / `scrollTop` 跟随；空态 `Empty` 放在 Viewport 外 |

- `MessageScrollerItem` 自带 `content-visibility: auto`。回合入场动画（`.turn-enter`）只用 transform / opacity，兼容。
- `MessageScroller` 需要有高度约束的父容器：`playground/page.tsx` 现有 `flex-1` 布局满足，必要时补 `min-h-0`。

## 5. 不变量

- i18n 布局稳定性契约：Button 的 `min-w-*` 预留、`DialogDescription` 的 `min-h`、对话框横向行 `flex-wrap` 照旧。
- `basePath=/web`、静态导出、无 SSR 依赖。`@shadcn/react` 是纯客户端 React 组件，不引入服务端依赖。
- 截断带 Tooltip 的 lint 零违规。
- 业务逻辑零改动：API 调用、筛选 DSL、权限守卫判定、playground 会话逻辑均不改。

## 6. 验证

每个提交：

```sh
cd web && npm run lint && npm run test && npm run format:check && npx next build --webpack
```

`next build --webpack` 用于规避 worktree 软链 node_modules 被 Turbopack 拒绝。

lock 变更后：

```sh
npx npm@10 ci --dry-run
```

运行时抽查（构建产物 + chrome-headless-shell + CDP），CDP 驱动的交互：

- filter-bar：聚焦后弹层出现在 body portal、方向键高亮移动、回车选字段后输入变为 `key:`、再回车生成标签、退格删除标签；
- Drawer：移动视口下打开 / 点「完成」关闭；
- MessageScroller：追加内容后视口在底部，上滚后出现「跳到最新」按钮；
- 双主题：新组件 `data-slot` 命中 `motion-*.css` 规则（`getComputedStyle` 断言 animation-name / transition）。

触摸拖拽关闭 Drawer 无法在无头环境模拟，交付时注明需真机复验。

## 7. 风险

| 风险 | 缓解 |
|---|---|
| Base UI Combobox 的 `inputValue` 受控与 `isItemPress` 取消语义不符合预期，导致选字段后输入被清空 | ④ 单独提交；先写 harness 用 CDP 跑键盘流程再接入页面 |
| ScrollArea 隐藏原生滚动条后，依赖 `scrollTop` 的逻辑取不到滚动元素 | 只替换不读 `scrollTop` 的容器；ReadingLayout 工具区通过 `viewportRef` 透传 |
| `@shadcn/react` 0.3.x 仍在早期，API 可能变动 | 锁定精确版本；只在 playground 一处使用，回退成本低 |
| 聊天区外观变化明显 | 用户已确认采用 shadcn 默认外观 |
