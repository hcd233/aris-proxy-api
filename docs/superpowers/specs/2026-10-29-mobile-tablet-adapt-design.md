# Web 端手机与平板适配设计

> 日期：2026-10-29　分支：`feature/mobile-tablet-adapt-2026-10-29`　范围：仅 `web/`，后端零改动

## 1. 意图

用户原话："我就想正常用，感觉目前移动端做的不是很好。"目标是在手机和平板上能顺手完成日常操作：看仪表盘、查会话和审计、管理上游、API Key、用户、定时任务等。不追求为移动端单独设计一套界面，而是让现有界面在小屏和触屏上不再别扭。

## 2. 现状基线（2026-10-29 实测）

用 demo 账户在生产环境（`https://api.lvlvko.top/web/`）以 390×844、DPR 2、移动端触屏模拟跑了 DOM 断言脚本（`/tmp/mobile-audit/audit.mjs`），结果如下：

- **整页横向溢出：0**。侧栏在 `<768px` 收为 Sheet 抽屉；sessions、apikeys、users、trigger、audit/model、upstream 六个页面在 `<768px` 已改成卡片列表。
- **触控目标过小**：时间范围和仪表盘维度的 `ToggleGroupItem` 高 24px；`Switch` 24×14；`Checkbox` 16×16；`Button size="sm"` 行内按钮高 28px；`FilterBar` 输入框可点高度 20px。
- **字号过小**：仪表盘约 52 处、模型审计约 57 处文字 ≤11px。根因之一是 `globals.css` 中 `:lang(zh)` 把 `--text-xs` 缩到 0.69rem（11.04px），另有 `text-[10px]`/`text-[11px]` 写死的地方。
- **筛选条横向挤出**：`TimeRangePicker` 宽 406px，在 324px 的容器里靠隐藏滚动条横滑（`no-scrollbar overflow-x-auto`），用户看不出后面还有选项；仪表盘页头同理（内容区 scrollWidth 462 / 390）。
- **表格需要横滑**：例如 audit/cron 共 7 列、宽 631px，手机上只能看到约一半。
- **内容区左右边距 16px**（`p-4`），偏紧。
- 管理员专属页面（users、cron、dataset、monitor、playground、harness、demo 等）demo 账户进不去，**未测**，需要在实施阶段补测（见 §9）。

## 3. 目标、非目标、反目标

**目标**
- 手机（360–430px 宽）和平板（768–1024px，竖屏和横屏）上所有页面都没有整页横向滚动，主要信息无需横滑即可看到。
- 触屏上可交互控件的可点区域 ≥44×44px。
- 手机上正文 ≥14px，次要信息 ≥12px。
- 桌面鼠标操作的观感和尺寸保持不变。

**非目标**
- 不做原生 App、PWA、离线能力或手势导航。
- 不改后端接口与数据结构。
- 不重新设计视觉风格、主题与动效世界；动效只补移动端弹层所需的规则。
- 已是卡片列表的 6 个页面，在手机上不改回表格。

**反目标**（即使功能上可用，出现下列情况也算失败）
- 桌面端布局或控件尺寸因本次改动发生变化。
- 切换语言时出现宽高跳变（违反 `docs/agents/web-frontend.md`「i18n 布局稳定性契约」）。
- 截断文字没有 Tooltip 或其他方式查看全文（违反 `truncate-requires-tooltip` lint）。
- 手机上隐藏的列里的信息完全看不到。

## 4. 断点与设备判定

| 名称 | 判定 | 用途 |
|---|---|---|
| 手机 | `<768px`（Tailwind `max-md`，既有 `useIsMobile`） | 卡片列表、精简表格、弹窗改为底部抽屉、筛选条改为下拉 |
| 平板 | `768–1023px`（`md` 到 `lg` 之前） | 侧栏默认收成图标栏；表格隐藏低优先级列 |
| 桌面 | `≥1024px`（`lg`） | 现状不变 |
| 触屏 | `pointer: coarse`（Tailwind v4 `pointer-coarse:` 变体） | 只放大可点区域，**与宽度无关**（平板横屏 1180px 也算触屏） |

决定（用户确认）：平板与手机都在本轮范围内。

## 5. 设计

### 5.1 数据表格：精简列 + 点行看详情（用户确认）

适用于手机上仍是"整表横滑"的 8 张表：

| 表 | 文件 | 手机保留列 | 平板额外显示 |
|---|---|---|---|
| 任务审计 | `app/(dashboard)/audit/cron/page.tsx` | 时间、任务名称、状态 | 耗时、触发来源 |
| Demo 访问审计 | `app/(dashboard)/audit/demo/page.tsx` | 时间、动作、模块 | 路径、IP |
| 定时任务 | `app/(dashboard)/cron/page.tsx` | 名称、启用 | 类型、表达式、操作 |
| Harness | `app/(dashboard)/harness/page.tsx` | 名称（模式作为下方次行文字） | 模式列、操作 |
| 分享管理 | `app/(dashboard)/shares/page.tsx` | 会话 ID、状态 | 创建时间、过期时间、操作 |
| Demo 会话 | `components/demo-sessions-manager.tsx` | 选择框、摘要、评分 | 时间、消息数 |
| 端点健康 | `components/dashboard/endpoint-health-panel.tsx` | 名称 + 1 个主指标 | 其余指标视宽度 |
| Top 排行 | `components/dashboard/top-ranking-panel.tsx` | 名称 + 1 个主指标 | 占比 |

"保留列"是初稿（用户确认"没问题"），实施时如果某列在手机上明显更关键，可以替换，但要在 §10 偏差记录里写明。

**机制**
- **列分级用纯 CSS**：每列标注一档可见性——常驻 / `md` 起 / `lg` 起。隐藏通过 `TableHead`/`TableCell` 上的 `hidden md:table-cell`、`hidden lg:table-cell` 实现，不引入 JS 测宽。为避免 8 个文件各自拼类名，在 `table.tsx` 的 `TableHead`/`TableCell` 上加可选属性（如 `priority?: "md" | "lg"`）映射到这些类；不传时行为完全不变。
- **点行看详情**：在 `<1024px` 且该表存在被隐藏的列时，点整行（或行内显式的"详情"热区，二选一由实现决定，但必须可用键盘操作，即 `role="button"` + `tabIndex` + Enter/Space）打开底部 `Drawer`，按"标签：值"列出该行全部字段，包括被隐藏列；行内操作（启用开关、删除、复制、编辑）在手机上移入抽屉底部，平板上若操作列可见则保留在行内。
- 抽屉复用已有 `components/ui/drawer.tsx`，抽成一个共享组件（建议 `components/row-detail-drawer.tsx`），由各表传入字段列表和操作区。抽屉必须有 `DrawerTitle`。
- 原本点行就会跳转的表（如 sessions 跳详情页）保持跳转，不加抽屉。
- 手机上剩余列宽按 §5.1 列表重新分配，`Table fit` 的列宽预算在三档宽度下都不超出容器（沿用 `web-frontend.md` 的 Table-Fit 规则与 `upstream/__tests__/table-column-budget.test.ts` 的思路）。

**已是卡片的 6 个页面**（sessions、apikeys、users、trigger、audit/model、upstream）：手机上保持卡片，只修控件尺寸和字号。平板上它们显示的是完整表格，同样套用列分级：隐藏 `lg` 档列；这些页面若已有详情入口则用原入口，否则同样接入行详情抽屉。

### 5.2 触控尺寸

在共享组件里用 `pointer-coarse:` 变体统一放大，不在页面里逐个改：

- `button.tsx`：各 size 在 coarse 下最小高度 44px（`icon*` 尺寸最小 44×44）。视觉上过大的场景（表格行内小按钮）允许只扩大点击区域（透明伪元素 `after:absolute after:-inset-*`），不放大外观。
- `toggle-group.tsx`/`toggle.tsx`、`input.tsx`、`input-group.tsx`、`select.tsx` 触发器、`combobox` 输入框、`textarea`：coarse 下最小高度 44px。
- `switch.tsx`、`checkbox.tsx`、`radio-group.tsx`：保持视觉尺寸，用伪元素把点击区域扩大到 ≥44×44，且不得与相邻控件的点击区域重叠（相邻控件间距不足时加大间距）。
- `FilterBar` 输入区：整条可点高度 ≥44px，点击空白处聚焦输入。
- 侧栏导航项、分页按钮、`DropdownMenuItem`、`SelectItem`：coarse 下行高 ≥44px。
- `button.tsx` 是项目定制过的文件，只追加 coarse 规则，不覆盖既有变体；`min-w-*` 预留（i18n Category Reserve）保持不变。
- 动效语法层（`styles/motion-{anthropic,moonshot}.css`）按 `data-slot` 命中，不受影响；如果放大后按压缩放过于明显，在语法层为 coarse 补规则。

### 5.3 字号

- `<768px` 下取消 `:lang(zh)`/`:lang(ja)` 对 `--text-xs` 和 `--text-sm` 的缩放，让 `text-xs` 保持 12px、`text-sm` 保持 14px；其他档位与桌面端的 CJK 缩放保持不变。
- 写死的 `text-[10px]`、`text-[11px]`、`text-[0.65rem]` 一类在手机上补 `max-md:text-xs`；图表刻度（recharts `tick` 字号）在手机上 ≥12px。
- 正文默认 `text-sm` 在手机上即为 14px，满足目标。

### 5.4 筛选条与页头

- `TimeRangePicker`：手机上改为单个 `Select`（选项：最近 1 小时 / 24 小时 / 7 天 / 30 天 / 自定义…），选"自定义"时打开现有的日期面板（手机上放进底部 `Drawer`）。桌面与平板保持现有按钮组。
- 仪表盘维度切换（费用 / 请求 / Token / 模型 / 用户 / API Key / 客户端）：手机上同样改为 `Select`。
- 页头（`PageHeader`）在手机上允许标题与操作区换行：标题一行，操作区换行后左对齐并 `flex-wrap`；仪表盘页头不得再撑出横向滚动。

### 5.5 页面框架

- 内容区边距：`p-4` → 手机 `px-5 py-4`；平板沿用 `md:p-8`，桌面不变。
- 安全区：根布局导出 `viewport`（`viewportFit: "cover"`），`MobileTopBar` 顶部加 `env(safe-area-inset-top)` 内边距；滚动容器底部加 `env(safe-area-inset-bottom)`；底部抽屉内容底部同样留出安全区。
- **平板侧栏**：`768–1023px` 默认收成图标栏（3rem），释放内容宽度；用户手动展开后照常记忆（复用 `sidebar-collapsed`）。会话详情页在平板上不再强制展开侧栏（`isSessionDetail` 的强制展开只在 `≥1024px` 生效），历史列表在平板上不显示于侧栏。
- **弹窗**：`<768px` 下 `DialogContent` 贴底、占满宽度、顶部圆角，最大高度 `90dvh` 内部滚动；`DialogFooter` 按钮纵向排列、全宽，主按钮在下（拇指可达）。改在 `dialog.tsx` 与 `alert-dialog.tsx` 的类名里，出场动效在两份动效语法层补 `max-md` 规则（从底部滑入）。平板与桌面维持居中弹窗。
- `Sheet`（侧栏抽屉）保持不变。

### 5.6 其他页面

login、callback、share、sessions/detail、playground、profile、dataset、monitor、demo 不在上面的表格清单里，实施时统一按 §5.2–5.5 的规则处理，再用 §9 的脚本逐页验证；发现的特有问题在 §10 记录。

## 6. 约束

- 遵守 `docs/agents/web-frontend.md` 全部契约：`output: "export"` 静态导出、`basePath` 不变、颜色走 CSS 变量、图标只用 `lucide-react`、截断必须带 Tooltip、i18n 宽高稳定、新增 shadcn 组件不用 `shadcn add` 直接安装。
- `components/ui/*` 只追加类名与可选 props，不改导出与命名；`shadcn-native.test.ts` 必须保持通过。
- 新增文案三种语言（en/zh/ja）同步。
- 不新增 npm 依赖。

## 7. 设计者自定的决定（未经用户逐条确认，审阅时请留意）

1. 触屏尺寸按 `pointer: coarse` 判定而不是按宽度，所以平板横屏和触屏笔记本也会变大。
2. 平板（768–1023px）侧栏默认收成图标栏；会话详情页在平板上不再强制展开侧栏，也不显示侧栏里的历史列表。
3. 手机上的弹窗统一改为贴底全宽样式，平板和桌面不变。
4. 手机上的时间范围和仪表盘维度改为下拉框。
5. 手机上取消中文和日文对 `text-xs`/`text-sm` 的字号缩放。

## 8. 留给实施者决定

- 列分级属性的具体命名与实现位置（只要不传时零行为变化）。
- 行详情的触发方式（整行可点 vs 行尾详情按钮），前提是键盘可用。
- 伪元素扩大点击区域的具体写法。
- 各表在平板上额外显示哪几列，可按 768px 实测宽度微调。

## 9. 验证

- 静态：`npm run lint && npm run test && npm run format:check`，worktree 内构建用 `npx next build --webpack`。
- **视口断言脚本**（`/tmp/mobile-audit/audit.mjs`，chrome-headless-shell + CDP，移动端触屏模拟）在 390×844、768×1024、1024×768 三档逐页跑，每页断言：
  1. `document.documentElement.scrollWidth <= innerWidth`；
  2. 主内容区内除 `Table` 容器外，没有 `scrollWidth > clientWidth` 的横滑元素；
  3. 触屏下可交互元素（含伪元素扩展后的命中区域，可用 `elementFromPoint` 在目标中心 ±22px 处采样）≥44×44；
  4. 手机档没有计算字号 <12px 的可见文本（装饰性 logo 字母除外）。
- 管理员页面：本地静态托管构建产物，用 CDP `Fetch` 拦截 `/api/web/v1/*`，把 `/user/current` 改成 admin 角色，管理员接口返回夹具数据，从而覆盖 demo 账户进不去的页面。
- 桌面回归：1440×900 鼠标模式（非触屏）下，对比改动前后每页关键控件尺寸（按钮、输入框、toggle 高度）完全一致。
- 交互：抽屉打开与关闭、键盘打开行详情、手机上时间范围 `Select` 切换后数据刷新，用 CDP 驱动点击和按键验证。
- 真机触摸手感无法在本机验证，交付时请用户在手机和 iPad 上复验。

## 10. 偏差记录

（实施过程中填写。）
