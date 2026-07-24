# Web 主题切换（Anthropic / Moonshot 深空）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 web 端新增 Moonshot 深空科技主题（星空粒子 + 蓝紫光晕 + 玻璃拟态 + 辉光联动），左下角浮动切换器在两套主题间切换并持久化记忆，全部页面生效。

**Architecture:** `<html data-theme="anthropic|moonshot">` 为单一事实来源；`globals.css` 新增 `[data-theme="moonshot"]` 块覆盖全套 CSS 变量（业务组件零改动）；粒子背景为全局 fixed 组件，MutationObserver 监听主题启停；根布局 `<body>` 顶端内联 blocking script 防 FOUC；`localStorage("theme")` 持久化。

**Tech Stack:** Next.js 16（App Router, 静态导出）+ React 19 + Tailwind v4 + 原生 Canvas 2D + lucide-react。

**Spec:** `docs/superpowers/specs/2026-07-24-theme-switch-design.md`

## Global Constraints

- 分支规范：`feature/theme-switch-moonshot-2026-07-24`，在 `.worktrees/` 下开发，禁止直接在主工作区开发。
- `web/` 为独立 npm 工程（`output: "export"`、`basePath: "/web"`），禁止引入需 SSR/Edge 的依赖，禁止改 basePath。
- 样式仅 Tailwind v4 + 主题 CSS 变量；新增全局 CSS 放 `globals.css` 末尾**且不得包在 `@layer` 中**（Tailwind v4 的 utilities 层优先级高于 `@layer base`，裸写规则才能压过 `shadow-*` 等 utility）。
- 图标仅 `lucide-react`；禁止 `alert/confirm`；过渡只作用颜色类属性（禁止 width/height transition，见 i18n 布局契约）。
- moonshot 主题固定深色，变量块置于 `.dark` 块之后，不受 `.dark` class 影响。
- 前端无单测框架；验证 = `npm run lint && npm run build` + 浏览器交互验证。
- 未经用户明确要求，不执行 git commit/push。

---

### Task 1: 创建 worktree 与分支，安装依赖

**Files:**
- 无（环境准备）

**Interfaces:**
- Produces: 工作目录 `.worktrees/theme-switch-moonshot-2026-07-24/`，后续所有任务在此目录内操作。

- [ ] **Step 1: 创建 worktree + 分支**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
git worktree add .worktrees/theme-switch-moonshot-2026-07-24 -b feature/theme-switch-moonshot-2026-07-24
```

Expected: 输出 `Preparing worktree (new branch 'feature/theme-switch-moonshot-2026-07-24')`

- [ ] **Step 2: 安装 web 依赖（worktree 内无 node_modules）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-switch-moonshot-2026-07-24/web && npm ci
```

Expected: 安装成功，无 ERR。

- [ ] **Step 3: 验证基线可构建**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-switch-moonshot-2026-07-24/web && npm run build`
Expected: 构建成功（未改动代码的基线绿）。

---

### Task 2: globals.css — moonshot 变量块、联动规则、粒子容器样式 + 根容器 `page-surface` 标记

**Files:**
- Modify: `web/src/app/globals.css`（文件末尾追加）
- Modify: `web/src/app/(dashboard)/layout.tsx:204`
- Modify: `web/src/app/login/page.tsx:66,80`
- Modify: `web/src/app/callback/page.tsx:56,72`
- Modify: `web/src/app/share/page.tsx:64`
- Modify: `web/src/components/permission-guard.tsx:20`
- Modify: `web/src/components/shared/reading-layout.tsx:46,112`

**Interfaces:**
- Produces: `[data-theme="moonshot"]` 全套 CSS 变量；`.particle-bg` / `.aura-*`（Task 3 的组件使用）；`page-surface` 类语义 =「整页根容器，moonshot 下背景透明以透出星空」；`html.theme-transition` 过渡规则（Task 4 的切换器使用）。

**注意**：整页根容器（`min-h-screen`/`h-screen`/`100dvh` + `bg-background`）共 8 处需加 `page-surface` 类，使 moonshot 下透明透出星空；小组件（switch 滑块、tabs、tooltip 等）上的 `bg-background` **不动**。

- [ ] **Step 1: globals.css 末尾追加 moonshot 主题块（裸写，不加 `@layer`）**

```css
/* === Moonshot deep-space theme (fixed dark; independent of .dark) === */
[data-theme="moonshot"] {
  --background: #06070d;
  --foreground: #e6e8f0;
  --card: rgb(13 15 26 / 0.72);
  --card-foreground: #e6e8f0;
  --popover: rgb(15 17 28 / 0.85);
  --popover-foreground: #e6e8f0;
  --primary: #7c8cff;
  --primary-foreground: #ffffff;
  --primary-hover: #93a2ff;
  --secondary: rgb(22 26 42 / 0.8);
  --secondary-foreground: #e6e8f0;
  --muted: rgb(22 26 42 / 0.8);
  --muted-foreground: #8a90a8;
  --accent: rgb(30 35 64 / 0.8);
  --accent-foreground: #e6e8f0;
  --destructive: #e06c75;
  --border: rgb(35 40 65 / 0.8);
  --input: rgb(35 40 65 / 0.8);
  --ring: #7c8cff;
  --chart-1: #7c8cff;
  --chart-2: #5e6ee0;
  --chart-3: #9db4ff;
  --chart-4: #6fe3e0;
  --chart-5: #2a3050;
  --sidebar: rgb(10 12 22 / 0.8);
  --sidebar-foreground: #e6e8f0;
  --sidebar-primary: #7c8cff;
  --sidebar-primary-foreground: #ffffff;
  --sidebar-accent: rgb(30 35 64 / 0.65);
  --sidebar-accent-foreground: #e6e8f0;
  --sidebar-border: rgb(35 40 65 / 0.8);
  --sidebar-ring: #7c8cff;
  --font-heading: var(--font-geist), "Avenir Next", "Noto Sans SC", "Noto Sans JP", ui-sans-serif, system-ui, sans-serif;
  --font-display: var(--font-geist), "Avenir Next", "Noto Sans SC", "Noto Sans JP", ui-sans-serif, system-ui, sans-serif;
  --shadow-2xs: 0 1px 2px 0 rgb(90 100 200 / 0.1);
  --shadow-xs: 0 1px 3px 0 rgb(90 100 200 / 0.14);
  --shadow-sm: 0 1px 3px 0 rgb(90 100 200 / 0.16), 0 2px 8px -1px rgb(90 100 200 / 0.12);
  --shadow-md: 0 2px 6px -1px rgb(90 100 200 / 0.18), 0 6px 16px -4px rgb(90 100 200 / 0.14);
  --shadow-lg: 0 4px 10px -2px rgb(90 100 200 / 0.2), 0 12px 28px -8px rgb(90 100 200 / 0.16);
  --shadow-xl: 0 8px 18px -4px rgb(90 100 200 / 0.22), 0 20px 44px -12px rgb(90 100 200 / 0.2);
}

/* Moonshot: page roots become transparent so the starfield shows through.
 * Only full-page containers carry the marker class; small widgets with
 * bg-background (switch knob, tabs, tooltips) stay opaque. */
[data-theme="moonshot"] :where(.page-surface) {
  background-color: transparent;
}

/* Moonshot: glassmorphism on cards / sidebar / popovers */
[data-theme="moonshot"] :where(.bg-card, .bg-sidebar, .bg-popover) {
  backdrop-filter: blur(12px) saturate(1.4);
  -webkit-backdrop-filter: blur(12px) saturate(1.4);
}

/* Moonshot: glow on primary surfaces and active nav items */
[data-theme="moonshot"] :where(.bg-primary) {
  box-shadow: 0 0 16px rgb(124 140 255 / 0.35);
}
[data-theme="moonshot"] :where(.bg-primary):hover {
  box-shadow: 0 0 24px rgb(124 140 255 / 0.5);
}
[data-theme="moonshot"] :where(.bg-sidebar-accent) {
  box-shadow: inset 0 0 0 1px rgb(124 140 255 / 0.25);
}

/* Moonshot: cool-tinted selection & scrollbar */
[data-theme="moonshot"] ::selection {
  background: rgb(124 140 255 / 0.3);
}
[data-theme="moonshot"] ::-webkit-scrollbar-thumb {
  background: rgb(124 140 255 / 0.2);
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: content-box;
}

/* Starfield container + gradient auras (used by ParticleBackground) */
.particle-bg {
  position: fixed;
  inset: 0;
  z-index: -10;
  pointer-events: none;
  overflow: hidden;
  opacity: 0;
  transition: opacity 500ms ease;
}
[data-theme="moonshot"] .particle-bg {
  opacity: 1;
}
.particle-bg canvas {
  position: absolute;
  inset: 0;
}
.particle-bg .aura {
  position: absolute;
  border-radius: 9999px;
  filter: blur(80px);
}
.particle-bg .aura-blue {
  width: 42vw;
  height: 42vw;
  left: -12vw;
  top: -14vh;
  background: radial-gradient(circle, rgb(59 79 216 / 0.38), transparent 70%);
  animation: aura-drift-a 26s ease-in-out infinite alternate;
}
.particle-bg .aura-violet {
  width: 38vw;
  height: 38vw;
  right: -10vw;
  bottom: -16vh;
  background: radial-gradient(circle, rgb(122 79 216 / 0.32), transparent 70%);
  animation: aura-drift-b 32s ease-in-out infinite alternate;
}
@keyframes aura-drift-a {
  from { transform: translate(0, 0) scale(1); }
  to { transform: translate(7vw, 5vh) scale(1.12); }
}
@keyframes aura-drift-b {
  from { transform: translate(0, 0) scale(1.08); }
  to { transform: translate(-6vw, -7vh) scale(1); }
}
@media (prefers-reduced-motion: reduce) {
  .particle-bg .aura {
    animation: none;
  }
}

/* Theme switch transition: color properties only, never width/height */
html.theme-transition,
html.theme-transition *,
html.theme-transition *::before,
html.theme-transition *::after {
  transition:
    background-color 350ms ease,
    border-color 350ms ease,
    color 350ms ease,
    box-shadow 350ms ease,
    fill 350ms ease,
    stroke 350ms ease !important;
}
```

- [ ] **Step 2: 8 处整页根容器加 `page-surface` 类（仅追加类名，不动其他）**

`web/src/app/(dashboard)/layout.tsx:204`：
```tsx
<div className="page-surface flex h-screen overflow-hidden bg-background text-foreground">
```

`web/src/app/login/page.tsx:66` 与 `:80` 两处：
```tsx
<div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
<div className="page-surface flex min-h-screen items-center justify-center bg-background px-4 py-10">
```

`web/src/app/callback/page.tsx:56` 与 `:72` 两处：
```tsx
<div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
<div className="page-surface flex min-h-screen items-center justify-center bg-background">
```

`web/src/app/share/page.tsx:64`：
```tsx
<div className="page-surface flex min-h-[100dvh] items-center justify-center bg-background px-4">
```

`web/src/components/permission-guard.tsx:20`：
```tsx
<div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
```

`web/src/components/shared/reading-layout.tsx:46` 与 `:112` 两处：
```tsx
<div className="page-surface -mx-4 -mt-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
<div className="page-surface -mx-4 -mt-4 -mb-4 flex h-[100dvh] overflow-hidden bg-background md:-mx-8 md:-mt-8 md:-mb-8 lg:-mx-10 lg:-mt-10 lg:-mb-10">
```

- [ ] **Step 3: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功（CSS 语法合法、类名无 TS 影响）。

---

### Task 3: ParticleBackground 组件（星空 canvas + 指针/滚动交互）

**Files:**
- Create: `web/src/components/theme/particle-background.tsx`

**Interfaces:**
- Consumes: `.particle-bg` / `.aura-blue` / `.aura-violet` 样式（Task 2）；`<html data-theme>`（Task 4 的 blocking script 与切换器写入）。
- Produces: `<ParticleBackground />`，挂于根布局（Task 4）。

行为契约：
- 仅 `data-theme="moonshot"` 时运行 rAF；切走立即停止并清屏。
- `prefers-reduced-motion: reduce`：moonshot 下只画静态一帧，不挂交互监听。
- 星星用归一化坐标（0..1）存储，resize 不重置分布。
- 指针 120px 内吸引增亮；任何滚动（capture）带来轻微视差（≤40px 循环）。

- [ ] **Step 1: 创建组件**

```tsx
"use client";

import { useEffect, useRef } from "react";

const STAR_COUNT = 70;
const POINTER_RADIUS = 120;
const STAR_COLORS = ["#BFCBFF", "#8FA3FF", "#E6E8F0"];
const PARALLAX_RANGE = 40;

interface Star {
  nx: number; // normalized x, 0..1
  ny: number; // normalized y, 0..1
  r: number;
  vx: number; // px per frame
  vy: number;
  phase: number;
  color: string;
}

export function ParticleBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;

    const root = document.documentElement;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);

    let raf = 0;
    let width = window.innerWidth;
    let height = window.innerHeight;
    let running = false;
    const pointer = { x: -9999, y: -9999 };

    const stars: Star[] = [];
    for (let i = 0; i < STAR_COUNT; i++) {
      stars.push({
        nx: Math.random(),
        ny: Math.random(),
        r: 0.6 + Math.random() * 1.2,
        vx: (Math.random() - 0.5) * 0.15,
        vy: (Math.random() - 0.5) * 0.15,
        phase: Math.random() * Math.PI * 2,
        color: STAR_COLORS[i % STAR_COLORS.length],
      });
    }

    const resize = () => {
      width = window.innerWidth;
      height = window.innerHeight;
      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(height * dpr);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener("resize", resize);

    const drawStar = (x: number, y: number, s: Star, alpha: number, glow: number) => {
      ctx.globalAlpha = Math.min(alpha, 1);
      ctx.fillStyle = s.color;
      ctx.beginPath();
      ctx.arc(x, y, s.r * glow, 0, Math.PI * 2);
      ctx.fill();
    };

    const frame = (t: number) => {
      ctx.clearRect(0, 0, width, height);
      for (const s of stars) {
        s.nx += s.vx / width;
        s.ny += s.vy / height;
        if (s.nx < -0.01) s.nx = 1.01;
        else if (s.nx > 1.01) s.nx = -0.01;
        if (s.ny < -0.01) s.ny = 1.01;
        else if (s.ny > 1.01) s.ny = -0.01;

        let x = s.nx * width;
        let y = s.ny * height;
        let boost = 0;
        const dx = pointer.x - x;
        const dy = pointer.y - y;
        const dist = Math.hypot(dx, dy);
        if (dist < POINTER_RADIUS && dist > 0.1) {
          const f = 1 - dist / POINTER_RADIUS;
          s.nx += (dx * f * 0.02) / width;
          s.ny += (dy * f * 0.02) / height;
          x = s.nx * width;
          y = s.ny * height;
          boost = f;
        }
        const twinkle = Math.sin(t / 900 + s.phase);
        drawStar(x, y, s, 0.35 + 0.4 * twinkle * twinkle + boost * 0.25, 1 + boost * 0.8);
      }
      ctx.globalAlpha = 1;
      raf = requestAnimationFrame(frame);
    };

    const drawStatic = () => {
      ctx.clearRect(0, 0, width, height);
      for (const s of stars) {
        drawStar(s.nx * width, s.ny * height, s, 0.75, 1);
      }
      ctx.globalAlpha = 1;
    };

    const onPointerMove = (e: PointerEvent) => {
      pointer.x = e.clientX;
      pointer.y = e.clientY;
    };
    const onPointerOut = () => {
      pointer.x = -9999;
      pointer.y = -9999;
    };
    const onScroll = () => {
      const y = document.scrollingElement?.scrollTop ?? 0;
      canvas.style.transform = `translateY(${-(y * 0.05) % PARALLAX_RANGE}px)`;
    };
    const onVisibility = () => {
      if (document.hidden) {
        cancelAnimationFrame(raf);
        running = false;
      } else if (root.dataset.theme === "moonshot" && !reduced && !running) {
        raf = requestAnimationFrame(frame);
        running = true;
      }
    };

    const stop = () => {
      cancelAnimationFrame(raf);
      running = false;
      window.removeEventListener("pointermove", onPointerMove);
      document.removeEventListener("pointerout", onPointerOut);
      window.removeEventListener("scroll", onScroll, true);
      canvas.style.transform = "";
      ctx.clearRect(0, 0, width, height);
    };

    const sync = () => {
      stop();
      if (root.dataset.theme !== "moonshot") return;
      if (reduced) {
        drawStatic();
        return;
      }
      window.addEventListener("pointermove", onPointerMove, { passive: true });
      document.addEventListener("pointerout", onPointerOut);
      window.addEventListener("scroll", onScroll, { capture: true, passive: true });
      raf = requestAnimationFrame(frame);
      running = true;
    };

    const observer = new MutationObserver(sync);
    observer.observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    document.addEventListener("visibilitychange", onVisibility);
    sync();

    return () => {
      observer.disconnect();
      document.removeEventListener("visibilitychange", onVisibility);
      stop();
      window.removeEventListener("resize", resize);
    };
  }, []);

  return (
    <div aria-hidden="true" className="particle-bg">
      <div className="aura aura-blue" />
      <div className="aura aura-violet" />
      <canvas ref={canvasRef} />
    </div>
  );
}
```

- [ ] **Step 2: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无 error。

---

### Task 4: ThemeSwitcher + 根布局挂载 + 防 FOUC + i18n 文案

**Files:**
- Create: `web/src/components/theme/theme-switcher.tsx`
- Modify: `web/src/app/layout.tsx`
- Modify: `web/src/locales/en.json`、`web/src/locales/zh.json`、`web/src/locales/ja.json`

**Interfaces:**
- Consumes: `html.theme-transition` 规则（Task 2）；`useT()`（`@/lib/i18n`）。
- Produces: `<ThemeSwitcher />`（fixed 左下角，读写 `localStorage("theme")` 与 `<html data-theme>`）。

- [ ] **Step 1: 创建 ThemeSwitcher**

```tsx
"use client";

import { useEffect, useState } from "react";
import { Feather, Sparkles } from "lucide-react";
import { useT } from "@/lib/i18n";

type Theme = "anthropic" | "moonshot";

export function ThemeSwitcher() {
  const t = useT();
  const [theme, setTheme] = useState<Theme | null>(null);

  useEffect(() => {
    setTheme(document.documentElement.dataset.theme === "moonshot" ? "moonshot" : "anthropic");
  }, []);

  const toggle = () => {
    if (theme === null) return;
    const next: Theme = theme === "moonshot" ? "anthropic" : "moonshot";
    const root = document.documentElement;
    root.classList.add("theme-transition");
    root.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {
      // private mode etc.: theme still applies for this session
    }
    setTheme(next);
    window.setTimeout(() => root.classList.remove("theme-transition"), 400);
  };

  const label = theme === "moonshot" ? t("theme.to_anthropic") : t("theme.to_moonshot");

  return (
    <button
      type="button"
      onClick={toggle}
      title={label}
      aria-label={label}
      className="fixed bottom-6 left-6 z-50 flex h-9 w-9 items-center justify-center rounded-full border border-border bg-popover/70 text-foreground/70 opacity-60 backdrop-blur-md transition-opacity hover:text-foreground hover:opacity-100"
    >
      {theme === null ? (
        <span className="size-4" />
      ) : theme === "moonshot" ? (
        <Feather className="size-4" />
      ) : (
        <Sparkles className="size-4" />
      )}
    </button>
  );
}
```

- [ ] **Step 2: 根布局 `web/src/app/layout.tsx` 挂载**

改动点（完整文件结构不变，仅下列三处）：

1. 顶部 import 追加：
```tsx
import Script from "next/script";
import { ParticleBackground } from "@/components/theme/particle-background";
import { ThemeSwitcher } from "@/components/theme/theme-switcher";
```

2. `metadata` 之后追加常量：
```tsx
const themeScript = `(function(){try{var t=localStorage.getItem("theme");if(t!=="moonshot")t="anthropic";document.documentElement.dataset.theme=t;}catch(e){document.documentElement.dataset.theme="anthropic";}})();`;
```

3. `<body>` 内改为：
```tsx
<body className="min-h-full flex flex-col">
  <Script id="theme-init" strategy="beforeInteractive" dangerouslySetInnerHTML={{ __html: themeScript }} />
  <I18nProvider>
    <HtmlLangUpdater />
    <AuthProvider>{children}</AuthProvider>
    <ParticleBackground />
    <ThemeSwitcher />
    <Toaster />
  </I18nProvider>
</body>
```

- [ ] **Step 3: i18n 文案（追加到各 json 末尾，保持 key 排序风格）**

`en.json`：
```json
"theme.to_moonshot": "Switch to Moonshot theme",
"theme.to_anthropic": "Switch to Anthropic theme"
```

`zh.json`：
```json
"theme.to_moonshot": "切换为 Moonshot 深空主题",
"theme.to_anthropic": "切换为 Anthropic 经典主题"
```

`ja.json`：
```json
"theme.to_moonshot": "Moonshot テーマに切り替え",
"theme.to_anthropic": "Anthropic テーマに切り替え"
```

- [ ] **Step 4: 类型检查 + lint**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无 error。

---

### Task 5: CONTEXT.md 术语回写

**Files:**
- Modify: `web/CONTEXT.md`（末尾追加新章节）

- [ ] **Step 1: 追加术语**

```markdown
## Theme（主题皮肤）

**Theme（主题皮肤）**:
整套视觉皮肤的命名集合，取值 `anthropic`（默认，暖色纸质）| `moonshot`（深空科技，固定深色）。持久化于 `localStorage("theme")`，由根布局内联脚本在 hydration 前反映到 `<html data-theme>`，是所有 per-theme 行为的单一事实来源。与明暗模式正交：moonshot 主题不受 `.dark` class 影响。整页根容器携带 `page-surface` 标记类，moonshot 下背景透明以透出星空；小组件不携带。
_Avoid_: color scheme, dark mode toggle
```

---

### Task 6: lint + build 全量验证

- [ ] **Step 1: lint**

Run: `cd web && npm run lint`
Expected: 无 error（warning 若来自既有代码，不处理）。

- [ ] **Step 2: 静态导出构建**

Run: `cd web && npm run build`
Expected: `✓ Compiled successfully`，`out/` 生成成功。

---

### Task 7: 浏览器交互验证

- [ ] **Step 1: 启动 dev server**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-switch-moonshot-2026-07-24/web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev
```

- [ ] **Step 2: preview_url 打开 `http://localhost:3000/web/login/`（免登录页）验证**

检查项：
1. 默认 anthropic 主题正常，左下角有半透明圆形切换钮（Sparkles 图标）。
2. 点击切换 → 350ms 颜色平滑过渡为深空主题；星空粒子出现并漂移；两团蓝紫光晕缓慢移动；登录卡片玻璃透视（星空透出）；primary 按钮带辉光。
3. 鼠标移动 → 附近粒子吸引增亮；滚动 → 轻微视差。
4. 刷新页面 → 仍为 moonshot（记忆生效，`localStorage("theme")==="moonshot"`）；首次 paint 无 anthropic 闪烁（blocking script 生效）。
5. 再点切换钮（Feather 图标）→ 平滑切回 anthropic，粒子停止（canvas 清屏），刷新后为 anthropic。
6. dashboard 内联动（侧边栏玻璃/激活导航光边）需登录态，请用户本地起后端后人工确认。

---

## Self-Review 记录

- Spec 覆盖：调色板/字体/阴影（Task 2）、玻璃/辉光/滚动条/selection（Task 2）、粒子+光晕+指针+滚动视差+reduced-motion（Task 2/3）、切换器+记忆+防FOUC+过渡（Task 2/4）、CONTEXT.md（Task 5）、验证（Task 6/7）——全覆盖。
- 已发现并修正两处设计落点：①联动 CSS 不能放 `@layer base`（utilities 层会覆盖）；②整页根容器需 `page-surface` 标记类，避免小组件 `bg-background` 被误透明化。
- 类型一致性：`data-theme` 值域 `anthropic|moonshot` 在 blocking script、ThemeSwitcher、ParticleBackground 三处一致。
