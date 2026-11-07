# Web 移动端适配规范（Lint 强制）

日期：2026-11-07；分支：`feature/web-mobile-lint-2026-11-07`

## 1. 目标与边界

- **目标**：把移动端最常见、可静态判定的适配缺陷固化为 ESLint `error` 规则，新代码不通过就进不了 `master`。
- **强制链路**：
  1. 本地 `.githooks/pre-commit` 对暂存的 `web/**/*.{ts,tsx,js,jsx,mjs,cjs}` 跑 eslint，失败即拒绝提交；
  2. CI `lint.yml` 的 `web-lint` job 跑全量 `npm run lint` + `npm test`，它是 `master` 分支保护的 required check；
  3. 规则自身由 `web/eslint-rules/mobile/mobile.test.mjs` 守护（RuleTester 正反例 + CSS 扫描）。
- **不在本规范内**：布局形态与交互设计（抽屉导航、断点形态切换、表格渐进列）仍按 `docs/agents/web-frontend.md` 的既有契约人工把关，见 §5。

## 2. 设计基线

| 项 | 取值 | 依据 |
| --- | --- | --- |
| 最小支持视口 | 360 × 640（兼顾 320 不崩） | 主流 Android 宽度；iPhone SE 为 375 |
| 手机内容区上限 | **320px** | 360 − 左右 16px 页面内边距 ≈ 328，取整到 `w-80` |
| 触控目标下限 | **24px** | WCAG 2.2 SC 2.5.8（AA）；门户等重要入口仍建议 44px（`max-md:size-11`） |
| 输入框字号下限 | **16px（触屏）** | iOS Safari 聚焦字号 < 16px 的输入框会放大整页且不回缩 |
| 触屏判定 | `pointer-coarse:` / `any-pointer-coarse:` | 不按宽度猜设备：平板 ≥ `md` 也是触屏 |

**变体优先级实测**（Tailwind v4 产物 CSS）：`pointer-coarse:*` / `any-pointer-coarse:*` 排在 `sm/md/lg` 断点之后，同特异性下能压过 `md:text-sm`、`opacity-0`；`group-hover:*` 特异性为 (0,2,0)，高于单类。所以兜底类直接追加即可，无需 `!important`。

## 3. 规则

规则统一挂在 ESLint 插件 `mobile`（`web/eslint-rules/mobile/index.mjs`），作用于 `web/src/**/*.{ts,tsx}`，全部 `error`。

### MOB-01 `mobile/no-viewport-vh`：视口高度用动态单位

- **问题**：移动浏览器的 `100vh` 按地址栏收起时的最大视口计算，地址栏展开时底部被裁（iOS 约 80px），整屏布局的底部按钮/输入框会被遮住。
- **规则**：禁止 `h-screen` / `min-h-screen` / `max-h-screen` / `size-screen`，以及任何字符串里的 `<数字>vh`（`max-h-[85vh]`、`calc(100vh-4rem)`、内联 style 的 `"100vh"`）。CSS 文件由单测扫描同一判据。
- **改法**：`h-dvh` / `min-h-dvh` / `max-h-[85dvh]`；需要稳定最小高度（不随地址栏抖动）时用 `svh`。

### MOB-02 `mobile/hover-reveal-needs-touch`：悬停显示必须有触屏兜底

- **问题**：Tailwind v4 的 `hover:` / `group-hover:` 包在 `@media (hover: hover)` 里，触屏永远不命中。「默认隐藏 + 悬停显示」的复制、编辑、删除按钮在手机/平板上等于不存在。
- **规则**：同一元素上同时出现「隐藏类」（`opacity-0` / `invisible` / `hidden`，可带断点或任意选择器变体）与「悬停显示类」（`hover:` / `group-hover[/name]:` / `peer-hover:` + `opacity-100` / `visible` / `block` / `flex` …）时，必须满足其一：
  - 有粗指针显示类：`pointer-coarse:opacity-100`（推荐，与 `code-block` 一致）；
  - 隐藏只作用于精确指针：`pointer-fine:opacity-0`。
- `focus-visible:` / `group-focus-within:` 只照顾键盘，**不算**触屏兜底；`md:opacity-0` 也不算（平板是 ≥ md 的触屏）。

### MOB-03 `mobile/input-font-size`：输入框在触屏上 ≥ 16px

- **规则**：
  - 原生 `input`（文本类 type）/ `textarea` / `select`，以及直接渲染原生元素的原语（`XxxPrimitive.Input`、`InputPrimitive`、`TextareaPrimitive`）：只要手机上生效的字号 < 16px（`text-sm`、`md:text-sm`、`text-[14px]`），必须同时带 `pointer-coarse:text-base`（或 `any-pointer-coarse:`）。
  - 业务封装组件（`Input`、`InputGroupInput`、`ComboboxInput`…）：tailwind-merge 不会用调用方的 `text-sm` 删掉封装内的 `pointer-coarse:text-base`，只拦截显式写小的粗指针字号（`pointer-coarse:text-sm`）。
- `placeholder:` / `file:` 等伪元素字号不参与判定。

### MOB-04 `mobile/no-fixed-overflow-width`：手机上的定宽 ≤ 320px

- **规则**：手机上生效（无变体，或只带 `max-*` / `dark` / 粗指针变体）的以下类，换算后 > 320px 即报错：
  - `w-*`：间距刻度（`w-96`）、容器刻度（`w-md`）、任意值（`w-[23.25rem]`）；同元素有流式上限 `max-w-full` / `max-w-[calc(100vw-2rem)]` / `max-w-[90vw]` 时放行；
  - `min-w-*`：没有豁免（`max-w` 压不回 `min-w`）；
  - `grid-cols-[…]`：定长轨道之和（`minmax` 取下限；含 `repeat()` 的放行）。
- **改法**：加断点前缀（`sm:w-[23.25rem]`）、改流式（`w-[min(23.25rem,85vw)]`）或配 `max-w-full`。
- 数据表确需横向滚动时，把定宽放进 `overflow-x-auto` 容器并 `eslint-disable-next-line mobile/no-fixed-overflow-width -- <理由>`。

### MOB-05 `mobile/min-touch-target`：可点元素高度 ≥ 24px

- **规则**：`button` / `a` / `summary` / `Button` 上手机生效的显式高度（`h-*`、`size-*`、`min-h-*`）< 24px 时，必须有其一：
  - 粗指针下 ≥ 24px 的尺寸：`pointer-coarse:h-6`；
  - 同元素的 `min-h-6` 及以上；
  - 伪元素热区扩展：`relative after:absolute after:-inset-2`（视觉不变，可点区域变大）。
- `Button` 的 size 变体（最小 `icon-xs` = 28px）已满足下限，不需要额外处理。

## 4. 已知盲区（按「宁漏报不误报」接受）

- 只分析 JSX 的 `className` / `xxxClassName` 属性里静态可见的字符串字面量；`cva()` 变体表、模块级类名常量、`mergeProps({ className })` 对象里的类名看不到（MOB-01 例外：它扫描全部字符串字面量）。
- 不做断点间的层叠推演：同一元素写了互相矛盾的类（`h-5 h-8`）按「任一命中即判定」处理。
- CSS 文件只由单测扫描 MOB-01；`:hover` 规则是否包在 `@media (hover: hover)` 内不检查。
- 例外一律走 `// eslint-disable-next-line mobile/<rule> -- <理由>`，理由必填，评审时逐条确认。

## 5. 规范内但需人工把关（不进 lint）

1. **形态切换优先于缩小**：窄屏改 `flex-col` / `flex-wrap` / 抽屉，不靠缩字号塞进去（见 Serena memory `web/portal-mobile-layout-2026-11-03`）。
2. **列表在窄屏渐进收列**：沿用 Progressive Columns（容器查询），主列可见宽度 ≥ ~110px。
3. **验收测量**：chrome-devtools `emulate({viewport: "390x844x3,mobile,touch"})`，按 320 → 390 → 768 → 1280 四档扫 `scrollWidth > clientWidth + 4`（排除装饰元素），目标 0。

## 6. 首次全量检查结果（2026-11-07）

接入规则后对 `web/src` 全量检查，共 **17 处违规 / 14 个文件**（MOB-01 ×6、MOB-02 ×6、MOB-03 ×3、MOB-04 ×1、MOB-05 ×1），均已在本分支修复，修复后 `npm run lint` 为 0：

| 规则 | 位置 | 问题 | 修复 |
| --- | --- | --- | --- |
| MOB-01 | `app/(dashboard)/layout.tsx` | 控制台外壳 `h-screen overflow-hidden`，iOS 地址栏展开时底部被裁 | `h-dvh` |
| MOB-01 | `app/callback/page.tsx` ×2、`components/permission-guard.tsx` ×2 | 居中页 `min-h-screen` | `min-h-dvh` |
| MOB-01 | `components/ui/alert-dialog.tsx` | 弹窗 `max-h-[85vh]` | `max-h-[85dvh]` |
| MOB-02 | `chat/user-message.tsx`、`chat/system-message.tsx`、`chat/assistant-message.tsx` | 会话详情 / 分享页的消息操作栏（复制等）仅悬停可见 | `pointer-coarse:opacity-100` |
| MOB-02 | `playground/chat-turn.tsx` ×2 | Playground 轮次操作（重试 / 复制）仅悬停可见 | 同上 |
| MOB-02 | `playground/session-sidebar.tsx` | Playground 会话列表的重命名 / 删除仅悬停可见 | 同上 |
| MOB-03 | `components/ui/input.tsx` | 全站 `Input` 基础字号 `text-sm`，iOS 聚焦放大 | `pointer-coarse:text-base` |
| MOB-03 | `components/ui/textarea.tsx` | `md:text-sm` 让 iPad 聚焦放大 | `pointer-coarse:text-base` |
| MOB-03 | `filter-bar/filter-bar.tsx` | 筛选栏 `ComboboxPrimitive.Input` 为 `text-sm` | `pointer-coarse:text-base` |
| MOB-04 | `portal/portal-feature-marquee.tsx` | 门户功能卡定宽 372px，375 视口下单卡比屏宽 | `w-[min(23.25rem,85vw)]` |
| MOB-05 | `dashboard/top-ranking-panel.tsx` | Top 排行「占比口径」切换按钮 `h-5`（20px） | `pointer-coarse:h-6` |

另：`components/ui/sidebar.tsx` 的 `SidebarMenuAction showOnHover` 也是悬停显示模式，但类名在 `mergeProps` 对象里（盲区），且全仓无调用方，暂不处理；启用前需补 `pointer-coarse:opacity-100`。

**产物级验证**（`next build` 产物 + chrome-headless-shell CDP，注入同款类名探针）：

| 视口 | 悬停兜底 opacity | Input / Textarea 字号 | `h-5` 按钮高 | 门户功能卡宽 | 文档横向溢出 |
| --- | --- | --- | --- | --- | --- |
| 1280 鼠标 | 0（不变） | 14 / 14px（不变） | 20px（不变） | 372px（不变） | 0 |
| 820 触屏（iPad） | 1 | 16 / 16px | 24px | 372px | 0 |
| 375 触屏 | 1 | 16 / 16px | 24px | 319px | 0 |
| 320 触屏 | 1 | 16 / 16px | 24px | 272px | 0 |

桌面端零视觉差异；触屏端兜底全部生效。

