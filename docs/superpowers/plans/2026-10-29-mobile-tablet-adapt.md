# Web 端手机与平板适配实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task（用户规则禁止子 Agent，只走 Native）。Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `web/` 管理后台在手机（360–430px）和平板（768–1024px）上能正常使用：无横滑、触控目标 ≥44px、字号可读、表格精简列 + 行详情抽屉。

**Architecture:** 尺寸类问题集中在一个 `pointer: coarse` 的 CSS 层（按 `data-slot` 命中，和动效语法层同一手法），不逐页改；字号用 `max-md` 下的 CSS 变量覆盖解决；表格用 `TableHead/TableCell` 的 `priority` 属性做纯 CSS 列分级，加一个共享的 `RowDetailDrawer`；框架层（边距、安全区、平板侧栏、弹窗贴底）只改 layout 与 `components/ui` 少数文件。

**Tech Stack:** Next.js 16.3.3 静态导出 + React 19 + Tailwind 4.3.3（内置 `pointer-coarse:` / `max-md:` 变体）+ Base UI（Drawer/Select）+ vitest（node 环境，无 jsdom）+ chrome-headless-shell CDP 脚本做运行时断言。

**Spec:** `docs/superpowers/specs/2026-10-29-mobile-tablet-adapt-design.md`

## Global Constraints

- 断点：手机 `<768px`（`max-md` / `useIsMobile`）；平板 `768–1023px`；桌面 `≥1024px`（`lg`）；触屏 = `(pointer: coarse)`，与宽度无关。
- 触屏可点区域 ≥44×44px；手机正文 ≥14px、次要文字 ≥12px（装饰性 logo 字母除外）。
- 桌面鼠标模式（1440×900、`pointer: fine`）下所有控件尺寸与改动前逐个一致。
- 不新增 npm 依赖；不改 `basePath`、`output: "export"`；颜色只用 CSS 变量；图标只用 `lucide-react`。
- `components/ui/*` 只追加类名与可选 props，不改导出名；不加 tw-animate 进出场类；`shadcn-native.test.ts` 保持通过。
- 截断必须在 `TooltipTrigger` 子树内（`truncate-requires-tooltip` lint）；i18n 切换不得宽高跳变；新文案 en/zh/ja 三份 `src/locales/*.json` 同步。
- worktree：`.worktrees/mobile-tablet-adapt`，`web/node_modules` 已软链主工作区；构建用 `npx next build --webpack`；验证后删 `web/.next`、`web/out`。
- 每个任务结束在本分支本地提交，不推送；提交前跑 `npm run lint && npm run test && npm run format:check`（hook 会再跑一遍）。

## Review Focus

1. **行内交互元素冒泡触发行详情**：点行内开关/复制/删除按钮时不应同时打开抽屉 → Task 6 的 `shouldOpenRowDetail` 与 Task 9 的 CDP 交互断言覆盖。
2. **CJK 字号覆盖与 `:lang()` 优先级**：`max-md` 下必须同时压过 `:lang(zh)` 和 `:lang(ja)` 的 `--text-xs/--text-sm`，en 不受影响 → Task 2 测试按三种语言断言。
3. **桌面零回归**：`pointer-coarse` 规则误写成宽度条件或漏 `@media` 包裹，会让桌面控件变大 → Task 1 源码测试 + Task 9 桌面尺寸对比。
4. **ReadingLayout 负边距与新内边距脱节**：layout 手机内边距从 `p-4` 改 `px-5 py-4` 后，会话详情/分享页的 `-mx-4` 必须同步，否则左右露缝 → Task 3 同批修改，Task 9 断言 `reading` 根元素左右贴边。
5. **平板横屏（1024×768 触屏）**：属于桌面断点但是触屏，表格全列显示 + 44px 控件，可能把 `Table fit` 列宽撑爆 → Task 9 在 1024×768 触屏档断言无表格横滑。

---

### Task 0: 验证脚手架与改动前基线（不进仓库）

**Files:**
- Create: `/tmp/mobile-audit/serve.sh`、`/tmp/mobile-audit/fixtures.mjs`
- Modify: `/tmp/mobile-audit/audit.mjs`（已存在，生产 demo 账户版）

**Interfaces:**
- Produces: `node /tmp/mobile-audit/audit.mjs <chrome> <baseURL> <out.json> <W> <H> [--fine]`；环境变量 `ROUTES`（逗号分隔）、`FIXTURES=1`（启用 CDP `Fetch` 拦截 `/api/web/v1/*`）；输出每路由 `{docW, vw, overflow[], hscroll[], small[], tiny[], controls{}}`，其中 `controls` 是"按钮/输入框/toggle/select 触发器"的 `data-slot → [宽x高…]` 汇总，用于桌面回归对比。

- [ ] **Step 1:** `serve.sh <outDir> <port>`：在 `out/` 下 `ln -s . web` 后 `python3 -m http.server`，使 `http://127.0.0.1:<port>/web/…` 可访问静态产物。
- [ ] **Step 2:** `fixtures.mjs` 导出 `route(pathname, search) → {status, body} | null`。覆盖：`/user/current`（返回 admin，字段按 `src/lib/types.ts` 的当前用户类型）、`/demo/status`，以及 18 个 dashboard 路由首屏调用的列表/统计接口，每个列表 3 行、字段取自 `types.ts` 对应 `*Rsp`；其中至少一行放超长文本（≥120 字符的路径/UA/元数据）、一行放 CJK 长名称。未覆盖的路径返回 `{}` 200 并在输出里记 `unmockedPaths`。
- [ ] **Step 3:** `audit.mjs` 增加：`--fine` 时不开触摸模拟、`mobile:false`；`FIXTURES=1` 时 `Fetch.enable({patterns:[{urlPattern:"*/api/web/v1/*"}]})` 并用 `Fetch.fulfillRequest` 回包；登录态直接写 `localStorage`（假 token）；新增 `controls` 汇总。
- [ ] **Step 4:** 构建改动前基线：当前分支除 `docs/` 外与 `origin/master` 137b8e56 一致，在 `.worktrees/mobile-tablet-adapt/web` 直接执行 `npx next build --webpack`，把 `out` 复制到 `/tmp/mobile-audit/out-base`。
- [ ] **Step 5:** 跑三档基线：`390x844`、`768x1024`、`1024x768`（触屏），以及 `1440x900 --fine`，全部 `FIXTURES=1`，存 `/tmp/mobile-audit/base-*.json`。Expected：`unmockedPaths` 为空；1440 档结果作为桌面回归基准。

### Task 1: 触屏尺寸层

**Files:**
- Create: `web/src/styles/touch.css`
- Modify: `web/src/app/globals.css`（在 motion 语法层 import 之后 `@import "../styles/touch.css";`，位置按现有 import 习惯）
- Test: `web/src/styles/touch.test.ts`

**Interfaces:**
- Produces: 全局规则，无 TS 接口。后续任务不得再在业务处写 `min-h-11` 一类触屏补丁。

- [ ] **Step 1: 写失败测试** `touch.test.ts`（源码文本断言，风格同 `shadcn-native.test.ts`）：
  - `it("所有规则都在 pointer: coarse 媒体查询内")`：去掉注释后，文件顶层只有一个 `@media (pointer: coarse) { … }` 块，块外无选择器。
  - `it("覆盖全部可交互 slot")`：块内文本包含以下全部 `data-slot`：`button`、`toggle-group-item`、`toggle`、`input`、`input-group`、`textarea`、`select-trigger`、`combobox-trigger`、`dropdown-menu-item`、`select-item`、`combobox-item`、`sidebar-menu-button`、`switch`、`checkbox`、`radio-group-item`。
  - `it("小控件只扩点击区域不放大外观")`：`switch`/`checkbox`/`radio-group-item` 只出现在 `::after` 选择器中，且该规则含 `min-width: 44px` 与 `min-height: 44px`。
- [ ] **Step 2:** `cd web && npx vitest run src/styles/touch.test.ts` → FAIL（文件不存在）。
- [ ] **Step 3: 实现 `touch.css`**：`@media (pointer: coarse)` 内：
  - 外观放大组（`min-height: 44px`）：`button`、`toggle-group-item`、`toggle`、`input`、`input-group`、`select-trigger`、`combobox-trigger`、`dropdown-menu-item`、`select-item`、`combobox-item`、`sidebar-menu-button`，以及 dashboard 侧栏导航项 `[data-nav-item]`（combobox 的输入框经 `input-group` 命中）；`textarea` 用 `min-height: 5.5rem`。图标按钮（`[data-slot=button][data-size^="icon"]`；`button.tsx` 目前不输出 `data-size`，需在其根元素追加 `data-size={size}`）再加 `min-width: 44px`。
  - 点击区扩展组：`[data-slot=switch]::after, [data-slot=checkbox]::after, [data-slot=radio-group-item]::after { inset: auto; left: 50%; top: 50%; translate: -50% -50%; min-width: 44px; min-height: 44px; width: 100%; height: 100%; }`（三者已有 `relative` 与 `after:absolute`）。
  - `FilterBar` 的输入容器若不是上述 slot，给它的根元素加 `data-slot="filter-bar"` 并纳入外观组，且点击容器空白处聚焦输入（在 `filter-bar.tsx` 根元素加 `onPointerDown` 聚焦，已有则跳过）。
  - 注释写明：为什么用 unlayered CSS（压过 Tailwind utilities 层里的 `h-*`，`min-height` 本就胜过 `height`）。
- [ ] **Step 4:** 测试 PASS；`npm run lint`。
- [ ] **Step 5:** 构建 + `audit.mjs` 跑 `390x844` 与 `1440x900 --fine`：390 档 `small` 列表中不再出现上述 slot；1440 档 `controls` 与 `base-1440` 完全相等。
- [ ] **Step 6: Commit** `feat(web): 触屏设备控件可点区域统一放大到 44px`

### Task 2: 手机字号

**Files:**
- Modify: `web/src/app/globals.css`（`:lang(zh)`/`:lang(ja)` 块之后）
- Test: `web/src/app/mobile-type.test.ts`

**Interfaces:**
- Produces: 常量 `TINY_TEXT_CLASSES`（导出自 `web/src/styles/tiny-text.ts`，`readonly string[]`，列出允许在源码中出现的 <12px 任意值类，如 `"text-[10px]"`、`"text-[11px]"`、`"text-[0.65rem]"`）；CSS 中对应规则必须一一覆盖。

- [ ] **Step 1: 写失败测试** `mobile-type.test.ts`：
  - `it("max-md 下 zh/ja 恢复 text-xs=0.75rem、text-sm=0.875rem")`：`globals.css` 中存在 `@media (width < 48rem)`（或 `max-width: 767px`）块，块内 `:lang(zh)` 与 `:lang(ja)` 都声明 `--text-xs: 0.75rem` 与 `--text-sm: 0.875rem`；块位于原 `:lang()` 块之后（`indexOf` 比较）。
  - `it("源码中出现的每个 <12px 任意字号类都在 TINY_TEXT_CLASSES 里")`：遍历 `src/**/*.tsx`，正则 `/(?<![\w:-])text-\[(\d+(?:\.\d+)?)(px|rem)\]/g` 抽出类，换算 px <12 的必须 ∈ `TINY_TEXT_CLASSES`（防止以后新增 `text-[9.5px]` 漏网）。
  - `it("每个 TINY_TEXT_CLASSES 都在 max-md 规则里被提升到 0.75rem")`：CSS 中存在含 `[class~="text-[10px]"]` 形式选择器的规则，值为 `font-size: 0.75rem`。
- [ ] **Step 2:** 运行 → FAIL。
- [ ] **Step 3: 实现**：新建 `tiny-text.ts`（按 Step 1 第二条测试扫出的实际集合填写，当前已知 79 处）；`globals.css` 追加 `@media (width < 48rem)` 块：`:lang(zh), :lang(ja) { --text-xs: 0.75rem; --text-xs--line-height: …; --text-sm: 0.875rem; --text-sm--line-height: … }`（行高取 Tailwind 默认值 `calc(1/0.75)` 与 `calc(1.25/0.875)`），以及 `:is([class~="text-[10px]"], …) { font-size: 0.75rem; }`。`[class~=…]` 精确匹配类 token，不会命中 `md:text-[10px]`。
- [ ] **Step 4:** 测试 PASS。recharts 刻度：`components/dashboard/*-panel.tsx` 与 `components/charts/*` 里 `tick={{ fontSize: N }}` 或 `fontSize={N}` 中 N<12 的，改为 `useIsMobile() ? 12 : N`（逐个 grep，改完在 spec §10 记录文件清单）。
- [ ] **Step 5:** 构建 + audit `390x844`：所有路由 `tiny` 仅剩 logo 字母（`layout.tsx` 品牌方块）；`1440 --fine` 档 `tiny` 列表与基线一致。
- [ ] **Step 6: Commit** `feat(web): 手机端恢复 CJK 小号字并提升 12px 以下字号`

### Task 3: 页面框架（边距、安全区、平板侧栏、页头）

**Files:**
- Modify: `web/src/hooks/use-mobile.ts`、`web/src/app/layout.tsx`、`web/src/app/(dashboard)/layout.tsx:430-450,453-592`、`web/src/components/shared/reading-layout.tsx:53,58,116`、`web/src/components/page-header.tsx`、`web/src/app/(dashboard)/page.tsx:71-93`

**Interfaces:**
- Produces: `export function useIsCompact(): boolean`（`use-mobile.ts`，`(max-width: 1023px)`，实现与 `useIsMobile` 同构：`useSyncExternalStore`，服务端快照 `false`）。Task 6、Task 8 使用。

- [ ] **Step 1:** `use-mobile.ts` 抽出 `function useMediaQuery(query: string): boolean`（同文件私有），`useIsMobile` 与新增 `useIsCompact` 都基于它；`useIsMobile` 行为不变。
- [ ] **Step 2:** 根 `layout.tsx` 导出 `export const viewport: Viewport = { width: "device-width", initialScale: 1, viewportFit: "cover" }`。
- [ ] **Step 3:** dashboard layout：内容滚动容器 `p-4 md:p-8 lg:p-10` → `px-5 py-4 pb-[calc(env(safe-area-inset-bottom)+1rem)] md:p-8 lg:p-10`；`MobileTopBar` 根元素加 `pt-[env(safe-area-inset-top)]`，高度改为 `h-[calc(3.5rem+env(safe-area-inset-top))]`（按现有写法调整）。
- [ ] **Step 4:** `reading-layout.tsx` 手机分支 `-mx-4 -mt-4` → `-mx-5 -mt-4`；桌面分支 `-mx-4 -mt-4 -mb-4` → `-mx-5 -mt-4 -mb-4`（`md:`/`lg:` 不变）；`top-[-1rem]` 不变（顶部仍是 1rem）。
- [ ] **Step 5: 平板侧栏**：`collapsed` 初始化 effect 中，`saved === null` 时 `setCollapsed(window.matchMedia("(min-width: 768px) and (max-width: 1023px)").matches)`；`isSessionDetail` 强制展开 effect 与 `handleOpenChange` 的锁定、侧栏 toggle 的 `disabled`、`SessionHistorySidebar` 显示条件，全部改为 `isSessionDetail && !isCompact`（`isCompact = useIsCompact()`）。
- [ ] **Step 6: 页头**：`PageHeader` 操作区 `shrink-0` → `flex flex-wrap items-center gap-2 max-md:w-full`；dashboard 页头右侧 `div` 加 `max-md:w-full`，`TimeRangePicker` 加 `className="max-md:flex-1"`（Task 5 会把它换成 Select）。
- [ ] **Step 7:** 构建 + audit：`390x844` 下 `main` 内容区左右内边距 20px；`/sessions/detail/?id=…` 与 `/share/?id=…` 的阅读布局根元素 `getBoundingClientRect().left === 0 && right === innerWidth`（在 audit 里对 `.page-surface` 根加一条断言）；`768x1024` 下侧栏宽度 48px（图标栏）；`1440 --fine` 与基线一致。
- [ ] **Step 8: Commit** `feat(web): 手机边距与安全区，平板侧栏默认收起`

### Task 4: 手机弹窗贴底

**Files:**
- Modify: `web/src/components/ui/dialog.tsx:54-56,91-95`、`web/src/components/ui/alert-dialog.tsx:40-42,66-68`、`web/src/styles/motion-anthropic.css`、`web/src/styles/motion-moonshot.css`
- Test: 追加到 `web/src/styles/touch.test.ts`（同属移动端源码契约）

- [ ] **Step 1: 写失败测试**：`it("dialog/alert-dialog 手机贴底")`：两文件的 Content 类名包含 `max-md:top-auto`、`max-md:bottom-0`、`max-md:translate-x-0`、`max-md:translate-y-0`、`max-md:max-w-none`、`max-md:rounded-b-none`、`max-md:max-h-[90dvh]`；Footer 类名包含 `max-md:flex-col` 且 `max-md:[&>*]:w-full`；两份 motion css 都含 `@media (width < 48rem)` 下针对 `dialog-content`/`alert-dialog-content` 的 `[data-open]` 规则。
- [ ] **Step 2:** 运行 → FAIL。
- [ ] **Step 3: 实现**：Content 追加 `max-md:top-auto max-md:bottom-0 max-md:left-0 max-md:translate-x-0 max-md:translate-y-0 max-md:max-w-none max-md:rounded-b-none max-md:max-h-[90dvh] max-md:pb-[calc(env(safe-area-inset-bottom)+1rem)]`；Footer 追加 `max-md:flex-col max-md:[&>*]:w-full`（`flex-col` 不反转 → 主按钮在下）。motion 两份各加一组 `@media (width < 48rem)`：`[data-open]` 用 `transform: translateY(100%) → none` 的 `motion-sheet-up` 关键帧（只动 `transform`，不动 `translate` 属性，理由见 anthropic 文件既有注释），`[data-closed]` 反向；`still` 档由全局 reduced-motion 守卫兜底。
- [ ] **Step 4:** 测试 PASS；`shadcn-native.test.ts` 仍 PASS。
- [ ] **Step 5:** 构建 + CDP：`390x844` 打开 upstream 页"新建端点"弹窗，断言 `dialog-content` 底边 = `innerHeight`、宽 = `innerWidth`、footer 内最后一个按钮位于最下；`1440 --fine` 打开同一弹窗，矩形与基线一致。
- [ ] **Step 6: Commit** `feat(web): 手机端弹窗改为贴底抽屉样式`

### Task 5: 手机筛选条改下拉

**Files:**
- Modify: `web/src/components/ui/time-range-picker.tsx`、`web/src/components/dashboard/top-ranking-panel.tsx:55-70`、`web/src/components/dashboard/usage-trend-panel.tsx:86-100`

**Interfaces:**
- Consumes: `useIsMobile`；`components/ui/select.tsx`（`Select/SelectTrigger/SelectValue/SelectContent/SelectGroup/SelectItem`）；`components/ui/drawer.tsx`。
- Produces: `TimeRangePickerProps` 不变（所有调用点零改动）。

- [ ] **Step 1:** `time-range-picker.tsx`：把 `PopoverContent` 内部（日历 + 时间 + 底栏）抽成同文件组件 `CustomRangePanel`（props：现有状态与回调，外加 `months: 1 | 2`）。
- [ ] **Step 2:** `isMobile` 时渲染 `Select`：选项 = 4 个预设 + `custom`（文案 `time.custom`，已选自定义时显示 `customDisplayLabel`）；选预设调 `onChange(key,"","")`；选 `custom` 不改值，打开 `Drawer`（`DrawerTitle` 用 `time_range.select_date_range`），里面渲染 `CustomRangePanel months={1}`，应用后关闭。触发器宽度 `w-full`。非手机分支保持原 `ToggleGroup` + `Popover`（`months={2}`）。
- [ ] **Step 3:** 两个 dashboard 面板的维度 `ToggleGroup` 在 `isMobile` 时换成 `Select size="sm"`（同一组选项与回调）。
- [ ] **Step 4:** `npm run lint && npm run test`。
- [ ] **Step 5:** 构建 + CDP（`390x844`，`FIXTURES=1`）：dashboard、audit/model、sessions、audit/cron 四页 `hscroll` 中不再出现 `toggle-group`；在 audit/cron 上选"最近 7 天"后，拦截到的列表请求查询参数含 7d 对应的起止时间（与桌面点击同一预设得到的参数一致）；选"自定义"打开 drawer、选两天、应用后请求参数为自定义区间。`768x1024` 档仍是按钮组。
- [ ] **Step 6: Commit** `feat(web): 手机端时间范围与仪表盘维度改为下拉`

### Task 6: 表格列分级与行详情抽屉

**Files:**
- Modify: `web/src/components/ui/table.tsx`
- Create: `web/src/components/row-detail-drawer.tsx`
- Modify: `web/src/locales/{en,zh,ja}.json`（新增 `row_detail.view`：View details / 查看详情 / 詳細を表示）
- Test: `web/src/components/row-detail.test.ts`

**Interfaces:**
- Produces（Task 7、8 使用）：
  - `table.tsx`：`TableHead`/`TableCell` 新增可选 `priority?: "md" | "lg"`；导出 `function priorityClass(p?: "md" | "lg"): string`（`undefined → ""`，`"md" → "hidden md:table-cell"`，`"lg" → "hidden lg:table-cell"`）。
  - `row-detail-drawer.tsx`：
    - `export interface RowDetailField { label: ReactNode; value: ReactNode }`
    - `export function RowDetailDrawer(props: { open: boolean; onOpenChange: (open: boolean) => void; title: ReactNode; fields: RowDetailField[]; actions?: ReactNode }): JSX.Element`——底部 `Drawer`（`showSwipeHandle`），`DrawerTitle` 为 `title`，字段用 `<dl>` 两列网格（标签 `text-muted-foreground`、值 `break-all`，不截断），`actions` 放 `DrawerFooter`，底部留安全区。
    - `export function shouldOpenRowDetail(target: EventTarget | null, row: Element): boolean`——`target` 最近的 `button,a,input,label,select,textarea,[role=switch],[role=checkbox],[role=menuitem],[data-row-detail-ignore]` 若存在且不是 `row` 本身则返回 `false`。
    - `export function rowDetailProps(enabled: boolean, onOpen: () => void, label: string): React.HTMLAttributes<HTMLTableRowElement>`——`enabled=false` 返回 `{}`；否则返回 `role="button"`、`tabIndex=0`、`aria-label=label`、`data-row-detail=""`、`onClick`（经 `shouldOpenRowDetail` 过滤）、`onKeyDown`（Enter/Space 且 `e.target === e.currentTarget` 时打开并 `preventDefault`）、`className="cursor-pointer"`。

- [ ] **Step 1: 写失败测试** `row-detail.test.ts`（node 环境，用最小假对象模拟 `closest`）：
  - `priorityClass(undefined) === ""`、`priorityClass("md") === "hidden md:table-cell"`、`priorityClass("lg") === "hidden lg:table-cell"`。
  - `shouldOpenRowDetail`：target 的 `closest()` 返回 `null` → `true`；返回 `row` → `true`；返回另一按钮元素 → `false`；target 为 `null` → `true`。
  - `rowDetailProps(false, fn, "x")` 深等于 `{}`；`rowDetailProps(true, fn, "x")` 含 `role: "button"`、`tabIndex: 0`、`aria-label: "x"`。
  - 三份 locale 都含 `row_detail.view`。
- [ ] **Step 2:** `npx vitest run src/components/row-detail.test.ts` → FAIL。
- [ ] **Step 3:** 实现上述接口；`TableHead`/`TableCell` 把 `priority` 从 props 解构出来，`cn(…, priorityClass(priority), className)`，不传时输出类名与改动前逐字一致。
- [ ] **Step 4:** 测试 PASS；`shadcn-native.test.ts` PASS；lint PASS。
- [ ] **Step 5: Commit** `feat(web): 表格列分级与行详情抽屉组件`

### Task 7: 8 张横滑表接入

**Files:**（列分级按 spec §5.1 表格；"手机保留列"不加 `priority`，"平板额外显示"加 `priority="md"`，其余加 `priority="lg"`）
- Modify: `web/src/app/(dashboard)/audit/cron/page.tsx:261-355`、`audit/demo/page.tsx`、`cron/page.tsx`、`harness/page.tsx`、`shares/page.tsx`、`web/src/components/demo-sessions-manager.tsx`、`web/src/components/dashboard/endpoint-health-panel.tsx:52-115`、`top-ranking-panel.tsx:73-120`
- Test: 追加到 `web/src/components/row-detail.test.ts`

**Interfaces:**
- Consumes: Task 6 全部；`useIsCompact`（Task 3）。

- [ ] **Step 1: 写失败测试** `it("横滑表全部接入列分级与行详情")`：对上面 8 个文件读源码，断言都 import 了 `RowDetailDrawer` 与 `rowDetailProps`，且含 `priority="`；对 cron/harness/shares/demo-sessions 四个有操作列的文件，断言操作列 `TableHead` 带 `priority=`（手机移入抽屉）且 `RowDetailDrawer` 调用含 `actions=`。
- [ ] **Step 2:** 运行 → FAIL。
- [ ] **Step 3:** 逐文件改造，每个文件同一模式：`const isCompact = useIsCompact(); const [detail, setDetail] = useState<RowType | null>(null);` → `TableRow {...rowDetailProps(isCompact, () => setDetail(row), <行主名>)}` → 列加 `priority` → 文件末尾 `<RowDetailDrawer open={detail !== null} onOpenChange={(o) => !o && setDetail(null)} title={…} fields={[…全部列…]} actions={…} />`。具体要求：
  - 抽屉 `fields` 必须包含该表全部列（含手机可见列），值复用表格单元格里的渲染（徽标、格式化时间、`formatDurationMs` 等），长文本不截断。
  - 行内原有点击（audit/cron 的 TraceID 复制单元格、cron 的启用开关、shares 的复制链接）保持可用；它们的元素已是 button/switch 时被 `shouldOpenRowDetail` 自动排除，非交互元素（如 `TableCell onClick`）加 `data-row-detail-ignore`。抽屉内也给 TraceID 提供复制按钮。
  - `Table fit` 的表重新核列宽：手机档只剩保留列时，主名列不声明宽度，其余保留列的 `w-*` 之和 ≤ 390 档容器宽（约 310px）。
  - Dashboard 两张 dense 表：手机保留"名称 + 请求数"（endpoint-health）与"名称 + 占比"（top-ranking），其余 `priority="md"`。
- [ ] **Step 4:** 测试 PASS；lint PASS。
- [ ] **Step 5:** 构建 + audit（`FIXTURES=1`）：`390x844`、`768x1024`、`1024x768` 三档这 8 张表所在页 `hscroll` 中无 `table-container`；CDP 在 390 档点 audit/cron 第一行 → 出现 `[data-slot=drawer-popup]` 且其文本包含 fixture 中该行 metadata 全文；点 TraceID 单元格 → 不出现抽屉且剪贴板调用发生（拦截 `navigator.clipboard.writeText`）；cron 页点启用开关 → 不出现抽屉；Tab 聚焦到行按 Enter → 出现抽屉。`1440 --fine`：这些表的列数与基线一致、行无 `role=button`。
- [ ] **Step 6: Commit** `feat(web): 横滑表格手机端精简列并支持行详情`

### Task 8: 卡片页的平板表格与卡片控件

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`、`apikeys/page.tsx`、`users/page.tsx`、`trigger/page.tsx`、`audit/model/page.tsx`、`upstream/flat-view.tsx`、`upstream/grouped-view.tsx`、`upstream/shared.tsx`（按需）
- Test: `web/src/app/(dashboard)/upstream/__tests__/table-column-budget.test.ts`（若平板隐藏列改变了 upstream 列宽预算，同步更新预算断言）

**Interfaces:**
- Consumes: Task 6 全部；`useIsCompact`。

- [ ] **Step 1:** 在 `768x1024` 触屏档跑 audit，记录这 6 页表格容器的 `scrollWidth/clientWidth` 作为待修清单。
- [ ] **Step 2:** 每页按"最不常看的列先藏"给列加 `priority="lg"`，直到 768 与 1024×768 两档都无表格横滑；决定写进 spec §10（页面 → 隐藏列）。
- [ ] **Step 3:** 行详情：sessions 保持点行跳详情页；audit/model 若已有详情弹窗/抽屉则复用；apikeys、users、trigger、upstream 平铺/分组在 `isCompact && !isMobile` 时接 `RowDetailDrawer`（手机仍是卡片，不接）。
- [ ] **Step 4:** 手机卡片：audit 中 390 档这 6 页 `small` 列表为空（Task 1 应已解决大部分，剩余为业务处写死 `h-6`/`size-6` 的触发器，改用组件 size 或在 `touch.css` 外观组可命中的 slot）。
- [ ] **Step 5:** `npm run test`（含 column budget）、lint PASS；`1440 --fine` 对比基线一致。
- [ ] **Step 6: Commit** `feat(web): 平板端表格隐藏次要列，卡片控件放大`

### Task 9: 其余页面巡检与总验收

**Files:**
- Modify: 巡检中发现问题的页面（预期涉及 `login/page.tsx`、`share/page.tsx`、`playground/*`、`profile/page.tsx`、`dataset/page.tsx`、`monitor/page.tsx`、`demo/page.tsx`、`sessions/detail`）
- Modify: `docs/superpowers/specs/2026-10-29-mobile-tablet-adapt-design.md` §10、`docs/agents/web-frontend.md`（新增「移动端与平板契约」小节：断点表、`touch.css` 规则、`TINY_TEXT_CLASSES`、`priority` + `RowDetailDrawer` 用法、弹窗贴底）、`web/CONTEXT.md`（新增术语 Row Detail / Column Priority / Touch Layer）

- [ ] **Step 1:** 全路由（18 个 dashboard + login + share + sessions/detail）在四档跑 audit（`390x844`、`768x1024`、`1024x768` 触屏、`1440x900 --fine`），另加 `360x740` 与 `844x390`（手机横屏）两档只断言无整页溢出。
- [ ] **Step 2:** 对每个失败项修复；同类问题优先在共享层（`touch.css`、`globals.css`、`components/ui`）解决，页面级修复写进 spec §10。
- [ ] **Step 3:** 验收断言全部满足（spec §9 四条 + Review Focus 第 4、5 条）：所有路由 `docW <= vw`；主内容除 `table-container` 外无横滑；触屏档 `small` 为空；390 档 `tiny` 仅 logo；`1440 --fine` 的 `controls` 与 `base-1440` 逐项相等；切换 en/zh/ja 后各档 `docW` 与控件矩形无变化（宽高稳定）。
- [ ] **Step 4:** `npm run lint && npm run test && npm run format:check && npx next build --webpack` 全绿；删除 `web/.next`、`web/out`。
- [ ] **Step 5:** spec §10 记录与 spec 的已知偏差：§5.3 写死小字号原计划逐处补 `max-md:text-xs`，改为 `globals.css` 中按 `TINY_TEXT_CLASSES` 统一提升（一处规则覆盖 79 处，测试防漏）。
- [ ] **Step 6: Commit** `docs(web): 移动端与平板适配契约与巡检修复`
