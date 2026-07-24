# Web 主题切换（Anthropic / Moonshot 深空）设计

> 日期：2026-07-24 ｜ 状态：待评审 ｜ 范围：`web/` 前端

## 背景与目标

管理后台前端当前只有一套 Anthropic 风格主题（暖色纸质，`--primary: #D97757`，衬线标题）。本需求新增第二套主题皮肤：参考 Moonshot AI 官网（moonshot.ai）的深空科技感——深空蓝黑背景、星空漂浮光点、蓝紫渐变光晕。

功能目标：

1. 两套完整主题（配色、字体、阴影、质感全部切换），应用在**所有界面**（login / dashboard / share 公开页）。
2. 左下角浮动切换器，点击即在两套主题间互换。
3. 选择持久化到 `localStorage`，刷新后保持。
4. 主题与页面组件联动：玻璃拟态透视、指针交互、组件辉光、切换过渡动效。

## 术语

**Theme（主题皮肤）**：整套视觉皮肤的命名集合，取值 `anthropic`（默认，暖色纸质）| `moonshot`（深空科技）。持久化于 `localStorage("theme")`，反映到 `<html data-theme>`，是所有 per-theme 行为的单一事实来源。与明暗模式（light/dark）正交且本需求不涉及：moonshot 固定深色，anthropic 保持现状。

## 方案对比

| 方案 | 机制 | 结论 |
|---|---|---|
| **A. CSS 变量覆盖 + `data-theme`** | `<html data-theme>` 切换，`globals.css` 新增 `[data-theme="moonshot"]` 块覆盖全套变量 | ✅ 采用。与现有 `.dark` 同模式，业务组件零改动 |
| B. Tailwind custom-variant（`moonshot:` 前缀） | 组件逐一添加 `moonshot:` 类 | ❌ 需改动全部组件，违背最小改动 |
| C. 双 CSS 文件动态切换 | 编译两份样式表切换 href | ❌ 静态导出下闪烁、构建链复杂 |

## 架构

```
localStorage("theme")  ←持久化──  ThemeSwitcher（fixed 左下角，点击互换）
        │                                │ 切换时
        ▼                                ▼
<head> blocking script ──读取──▶  <html data-theme="anthropic|moonshot">
                                        │
              ┌─────────────────────────┼──────────────────────────┐
              ▼                         ▼                          ▼
   [data-theme="moonshot"]       ParticleBackground          html.theme-transition
   覆盖全套 CSS 变量              MutationObserver 监听        （切换后 350ms 移除）
   （颜色/阴影/字体/玻璃/辉光）    仅 moonshot 时启动 rAF
```

- 不引入 React Context：切换器是唯一写入者，粒子组件用 `MutationObserver` 自行监听 `data-theme`（ponytail：避免为单一消费者建 Provider）。
- 防 FOUC：`layout.tsx` 的 `<head>` 内联 blocking script，hydration 前从 `localStorage` 读主题并设置 `data-theme`；非法值回退 `anthropic`。
- 优先级：`[data-theme="moonshot"]` 块置于 `.dark` 块之后，同特异性下后定义者胜，保证 moonshot 变量不受 `.dark` class 影响（moonshot 固定深色，无视明暗体系）。

## Moonshot 调色板（`[data-theme="moonshot"]` 变量块）

| 变量 | 值 | 说明 |
|---|---|---|
| `--background` | `#06070D` | 深空蓝黑 |
| `--foreground` | `#E6E8F0` | 冷白 |
| `--card` / `--popover` | `rgb(13 15 26 / 0.72)` / `rgb(15 17 28 / 0.85)` | 半透明（玻璃拟态前提） |
| `--primary` / `--ring` | `#7C8CFF` | 科技蓝紫 |
| `--secondary` / `--muted` | `rgb(22 26 42 / 0.8)` | 冷灰蓝 |
| `--muted-foreground` | `#8A90A8` | |
| `--accent` | `rgb(30 35 64 / 0.8)` | |
| `--border` / `--input` | `rgb(35 40 65 / 0.8)` | 半透明冷边 |
| `--destructive` | `#E06C75` | |
| `--chart-1..5` | `#7C8CFF` `#5E6EE0` `#9DB4FF` `#6FE3E0` `#2A3050` | 蓝紫青冷梯度 |
| `--sidebar*` | 同系深空色，`--sidebar` 用 `rgb(10 12 22 / 0.8)` | |
| `--shadow-*` | 蓝紫色调（`rgb(90 100 200 / α)`） | 组件阴影自动变发光感 |
| `--font-heading` / `--font-display` | 覆盖为 `var(--font-geist)` 无衬线栈 | 科技感，正文不变 |
| `::selection` / 滚动条 thumb | 蓝紫半透明 / 冷灰蓝 | |

## 粒子背景组件 `ParticleBackground`

新建 `web/src/components/theme/particle-background.tsx`（client component）：

- **星空 canvas**：约 70 个光点慢速漂移（速度向量 + 边界回绕），半径 0.6–1.8px，`globalAlpha` 正弦呼吸；颜色取蓝白系（`#BFCBFF`/`#8FA3FF`）。DPR 适配、resize 重设尺寸、`visibilitychange` 暂停 rAF。
- **渐变光晕**：2 个纯 CSS `div`（`radial-gradient` 蓝 `#3B4FD8` / 紫 `#7A4FD8`，`filter: blur(80px)`，约 40vw 直径），CSS animation 缓慢漂移（20s+ 周期），零 JS 开销。
- **指针交互**：监听 `pointermove`，指针 120px 半径内粒子受轻微吸引并增亮（每帧 lerp 平滑），移开后弹回原轨道。
- **滚动视差**：`window.addEventListener("scroll", …, { capture: true, passive: true })` 捕获所有滚动（含 dashboard `main` 内滚动），容器 `translateY(滚动增量 × 0.05)` 轻微视差。
- **显隐与启停**：容器默认 `opacity-0 pointer-events-none`，`[data-theme="moonshot"]` 下淡入（`opacity` transition 500ms）；JS 用 `MutationObserver` 监听 `<html>` 的 `data-theme`，仅 moonshot 时启动 rAF 循环，切回 anthropic 立即停止。
- **`prefers-reduced-motion`**：canvas 只渲染静态一帧，光晕 animation 关闭，指针/滚动交互不挂载。
- 层级：`fixed inset-0 -z-10`，内容自然在上；`aria-hidden`。

## 联动效果

### 1. 玻璃拟态透视

moonshot 变量块中 `--card` / `--popover` / `--sidebar` 为半透明色；`globals.css` 追加：

```css
[data-theme="moonshot"] :where(.bg-card, .bg-sidebar, .bg-popover) {
  backdrop-filter: blur(12px) saturate(1.4);
  -webkit-backdrop-filter: blur(12px) saturate(1.4);
}
```

`:where()` 零特异性，不覆盖组件自有类。弹窗 portal 到 body，`data-theme` 在 `<html>` 上依然命中。

### 2. 指针交互联动

见「粒子背景组件」指针交互部分。

### 3. 组件辉光联动

- `--shadow-*` 变量换蓝紫发光色调（全局自动生效）。
- moonshot 块追加：
  - `[data-theme="moonshot"] .bg-primary` → `box-shadow: 0 0 16px rgb(124 140 255 / 0.35)`，`:hover` 增强到 `0 0 24px …/0.5`（primary 按钮、logo 块、徽标发光）。
  - `[data-theme="moonshot"] .bg-sidebar-accent` → `box-shadow: inset 0 0 0 1px rgb(124 140 255 / 0.25)`（激活导航项光边）。

### 4. 切换过渡动效

- `globals.css` 追加：

```css
html.theme-transition,
html.theme-transition *,
html.theme-transition *::before,
html.theme-transition *::after {
  transition: background-color 350ms ease, border-color 350ms ease,
    color 350ms ease, box-shadow 350ms ease, fill 350ms ease, stroke 350ms ease !important;
}
```

- ThemeSwitcher 切换时给 `<html>` 加 `theme-transition`，400ms 后移除。只过渡颜色类属性（不动宽高），符合 i18n 布局稳定性契约。
- 粒子背景随 `opacity` 过渡淡入淡出。

## 切换器 `ThemeSwitcher`

新建 `web/src/components/theme/theme-switcher.tsx`（client component）：

- `fixed bottom-6 left-6 z-50`，`h-9 w-9` 圆形按钮：`border bg-popover/70 backdrop-blur-md`，`opacity-60 hover:opacity-100` 低存在感（避免遮挡 dashboard 侧边栏底部 UserBar）。
- lucide 图标：anthropic 态显示 `Sparkles`（点击切换到科技主题），moonshot 态显示 `Feather`（点击切回经典）。
- 点击逻辑：加 `theme-transition` class → 更新 `documentElement.dataset.theme` + `localStorage("theme")` → 400ms 后移除 class。
- `title` / `aria-label` 走 i18n：`locales/{en,zh,ja}.json` 各加 `theme.to_moonshot` / `theme.to_anthropic` 两条。
- 初始图标避免 hydration 不匹配：`useEffect` 内读 `documentElement.dataset.theme` 后渲染（mount 前渲染占位或默认图标，`suppressHydrationWarning` 已有）。

## 文件清单

| 文件 | 改动 |
|---|---|
| `web/src/app/globals.css` | +moonshot 变量块、玻璃/辉光规则、`theme-transition`（约 150 行） |
| `web/src/components/theme/particle-background.tsx` | 新建（约 180 行） |
| `web/src/components/theme/theme-switcher.tsx` | 新建（约 60 行） |
| `web/src/app/layout.tsx` | 挂载两个组件 + `<head>` 防 FOUC 内联脚本 |
| `web/src/locales/{en,zh,ja}.json` | 各 +2 条文案 |
| `web/CONTEXT.md` | +「Theme（主题皮肤）」术语 |

## 不做的事（YAGNI）

- 不做主题自定义（用户自选颜色）、不做第三套主题、不给 anthropic 补暗色入口。
- 不改 `markdown-lite.tsx` 代码块等少数硬编码暖色（moonshot 下呈暖色代码块，列为已知边界，后续需要再处理）。
- 不引入第三方粒子库（原生 Canvas 2D 足够）。

## 验证计划

1. `cd web && npm run lint && npm run build`：类型、lint、静态导出成功。
2. Chrome MCP 交互验证（本地 dev server）：
   - 点击左下角切换器 → moonshot 主题全页面生效（dashboard / sessions / login / share），星空 + 光晕出现，卡片玻璃透视。
   - 鼠标移动 → 指针附近粒子吸引增亮；滚动 → 轻微视差。
   - 刷新页面 → 主题保持（`localStorage("theme")` 正确读写）。
   - 切回 anthropic → 完全复原，粒子停止（rAF 已停）。
   - 切换瞬间颜色 350ms 平滑过渡，无布局跳变。
