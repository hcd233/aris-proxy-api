# i18n 布局稳定性 — Hybrid 策略

切换语言（en/zh/ja）时，因翻译字符长度与 CJK 字形度量差异，按钮、徽章、表格行、卡片等组件的宽高发生跳变（Switch Flicker）。本 ADR 确立一套跨组件的稳定化约定，而非逐个组件打补丁。

**决策：Hybrid 策略** — 按组件类别分流，外加 per-locale 字号调节与切换淡入兜底：

1. **Rigid 元素**（按钮、徽章、分页触发器）→ Category Reserve：按类别统一 `min-w`，溢出 `truncate`。侧边栏导航项不在此列（侧边栏 `w-64` 已定宽，容器吸收了位移）。
2. **Elastic 元素**（描述、对话框正文、表格描述列）→ Layout-Pattern Height Fix：表头 `whitespace-nowrap` + 单行单元格 `truncate` + `title=` 提示；卡片网格 `items-stretch` + 描述 `line-clamp-2`；对话框正文 `min-h` 按最长语言行数预留。关键内容永不截断。
3. **CJK 字号** → `:lang(zh)` / `:lang(ja)` 下等比下调字号（见下）。
4. **切换闪烁** → 以稳定化为主消除；仅在 dashboard 内容根与 share 页根加 ~150ms opacity 淡入兜底残留 reflow。不在 `width`/`height` 上加 CSS transition（layout thrash），不引入 View Transitions API（静态导出场景过重）。

**Why Hybrid, not all-Rigid.** 全部按最长翻译预留空间会让短语言（如英文按钮为容纳「キャンセル」而变宽）出现大量空白，且关键描述文本被截断的风险不可接受。结构性元素（按钮/徽章/分页）值得用空白换稳定，自由文本不值得。

**Why Hybrid, not all-Elastic.** 纯 flex/grid + truncate 无法消除相邻元素的位移（按钮组左右抖动依旧），也不解决 CJK 字号偏大。Elastic 只适合自由文本。

**Why Category Reserve, not per-key generated CSS.** 备选是构建脚本读 3 份 locale JSON 为每个 key 生成 `.i18n-w-{key}{min-width:Nch}`。`ch` 单位对比例字体不准（CJK 与拉丁字宽差异更大），且每条新翻译都要重建。Category Reserve 覆盖 ~90% 痛点表面，零构建工具，代价仅是可接受的少量空白。个别超长翻译手动加 `min-w`。

**Why Layout-Pattern Height Fix, not global min-h + line-clamp.** 全局统一会截断关键文本并制造大面积空白。高度跳变真正伤人的是结构性对齐（表格行重排、卡片网格错高），自由描述的高度变化是自然的。按布局类型分别约束即可。

**Why `:lang()` + Tailwind v4 `--text-*` 覆盖做 Font Scale.** `<html lang>` 已由 `HtmlLangUpdater` 设置，`:lang()` 零成本可用。CJK 字号下调有两种实现：
- (a) 改根 `font-size` → 缩放 `1rem`，连带缩放所有 rem 间距，整 UI 比例变形。否决。
- (b) 在 `:lang(zh)/:lang(ja)` 下覆盖 Tailwind v4 的 `--text-xs/--text-sm/--text-base/.../--text-5xl` 主题变量（已确认 `text-sm` 等 utility 编译为 `font-size: var(--text-sm)`）。仅作用于字号，间距不动，且走 Tailwind 自身变量机制，无源码顺序脆弱性。采纳。

目标缩放比：zh `0.92`、ja `0.88`（实现时按视觉微调）。仅覆盖实际在用的 9 档（xs/sm/base/lg/xl/2xl/3xl/4xl/5xl）。

**Why stabilize-first, not transition-everything.** 切换闪烁的根因是位移本身；尺寸预留后无物可闪，文字内容瞬时替换是预期行为。per-element `transition: width/height` 会触发布局抖动且多数元素无显式 width 可过渡；View Transitions API 在静态导出 + 多浏览器场景下过重。仅对内容根加 opacity 淡入作为自由文本区残留 reflow 的兜底。

**初始加载闪烁不在本 ADR 范围。** `I18nProvider` 当前 `useState("en")` + `useEffect` 探测，首次绘制为英文。修复需改 provider 初始化（`useState` 初始化器触发静态导出 HTML 的 hydration mismatch；阻塞 `<head>` 脚本增加复杂度）。属低频一次性事件，架构风险高，留作后续。

**Consequences:**
- 新增 Category Reserve 与 Layout-Pattern 约定需写入 `web/` 前端契约（`docs/agents/web-frontend.md`），新增按钮/徽章须按类别给 `min-w`，新增表格/卡片/对话框须遵循对应高度规则。
- `globals.css` 新增 `:lang(zh)/:lang(ja)` 下的 `--text-*` 覆盖块；Tailwind 升级时需复核 `--text-*` 变量命名是否稳定。
- 新增 `<LocaleFade>` 包裹 dashboard `<main>` 内 `max-w-6xl` 容器与 share 页根，监听 `locale` 变化做 ~150ms opacity 淡入。
- CJK 字号下调后，预留的 `min-w` 应以缩放后的 CJK 宽度为准重新核定。
- 初始加载闪烁保留，待后续单独处理。
