# Warm Paper Web Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `web` 管理端升级为 Warm Paper Console 全局外壳，并加入受 Claude 启发的手写感标题字体。

**Architecture:** 保持现有 Next.js App Router、AuthProvider、API client 和页面数据流不变。通过 `globals.css` 配置本地字体栈、主题变量和基础质感，dashboard layout 与少量通用组件 class 统一外壳视觉。

**Tech Stack:** Next.js 16、React 19、TypeScript、Tailwind CSS 4、shadcn/base-ui 风格组件、`next/font/google`。

---

### Task 1: 全局字体与主题变量

**Files:**
- Modify: `web/src/app/layout.tsx`
- Modify: `web/src/app/globals.css`

- [ ] **Step 1: 注册手写感 display 字体**

在 `web/src/app/layout.tsx` 中把 `Kalam` 从 `next/font/google` 引入，并将变量加入 `<html>` class。

```tsx
import { Geist, Geist_Mono, Kalam } from "next/font/google";

const kalam = Kalam({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["400", "700"],
});

<html
  lang="en"
  className={`${geistSans.variable} ${geistMono.variable} ${kalam.variable} h-full antialiased`}
>
```

- [ ] **Step 2: 更新 Tailwind 字体和暖纸主题变量**

在 `web/src/app/globals.css` 的 `@theme inline` 中加入 display 字体映射，并把 `:root` / `.dark` 更新为 Warm Paper 色板。

```css
--font-display: var(--font-display);
--background: oklch(0.955 0.03 82);
--foreground: oklch(0.18 0.035 62);
--card: oklch(0.985 0.018 82);
--primary: oklch(0.62 0.13 72);
--border: oklch(0.79 0.055 78);
--sidebar: oklch(0.21 0.04 58);
```

- [ ] **Step 3: 增加全局纸张基础质感**

在 `@layer base` 中为 `body` 增加暖纸背景、选区和滚动条样式。保持 `font-sans` 为正文默认字体。

```css
body {
  @apply bg-background text-foreground;
  background-image:
    radial-gradient(circle at top left, oklch(0.98 0.035 88 / 70%), transparent 30rem),
    linear-gradient(135deg, oklch(0.97 0.025 86), oklch(0.92 0.04 78));
}
```

- [ ] **Step 4: 验证主题编译**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint`

Expected: ESLint 通过，或只暴露与本次改动无关的既有问题。

### Task 2: 后台外壳与导航

**Files:**
- Modify: `web/src/app/(dashboard)/layout.tsx`

- [ ] **Step 1: 改造桌面侧边栏**

将侧边栏从默认灰白卡片改为深咖色品牌栏，品牌标题使用 `font-display`，导航激活态使用暖纸底色。

```tsx
<aside className="hidden md:flex flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[0_24px_70px_rgba(62,38,16,0.22)] transition-[width] duration-200">
```

- [ ] **Step 2: 改造导航项状态**

在 `SidebarNav` 中保持现有 `isActive` 逻辑，仅替换 class：激活态为浅纸色胶囊，非激活态为低对比暖色 hover。

```tsx
isActive
  ? "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm"
  : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
```

- [ ] **Step 3: 改造用户信息区和移动端 Sheet**

`UserBar` 保留 logout 行为和 avatar 数据，只调整容器、Badge、按钮的视觉。移动端 `SheetContent` 使用同样深咖背景。

- [ ] **Step 4: 改造主内容容器**

主容器改为暖纸背景，`main` 增加最大宽度、居中和响应式 padding，避免内容贴边。

```tsx
<main className="flex-1 overflow-y-auto p-4 md:p-6 lg:p-8">
  <div className="mx-auto max-w-7xl">{children}</div>
</main>
```

- [ ] **Step 5: 验证导航行为**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint`

Expected: 不出现 React hook、TypeScript 或 JSX 语法错误。

### Task 3: 通用 UI 组件质感

**Files:**
- Modify: `web/src/components/ui/card.tsx`
- Modify: `web/src/components/ui/button.tsx`
- Modify: `web/src/components/ui/table.tsx`
- Modify: `web/src/components/ui/badge.tsx`
- Modify: `web/src/components/ui/skeleton.tsx`

- [ ] **Step 1: Card 纸张化**

保留组件 API，只在默认 class 中增加更大圆角、暖色边框和柔和阴影。

```tsx
"bg-card text-card-foreground flex flex-col gap-6 rounded-2xl border py-6 shadow-[0_18px_45px_rgba(92,62,29,0.08)]"
```

- [ ] **Step 2: Button 暖金主按钮**

保持 variant 名称不变，更新 `default`、`outline`、`ghost` 的 class，使主操作为蜂蜜金，outline 为纸面按钮。

- [ ] **Step 3: Table ledger 风格**

保持 `Table` API 不变，更新 header、row hover、cell 字号和边框色，使表格像操作 ledger。

- [ ] **Step 4: Badge/Skeleton 适配暖色主题**

Badge 保持语义 variant，默认更圆润；Skeleton 使用 `bg-muted/70` 和轻微透明度，避免冷灰色。

- [ ] **Step 5: 验证通用组件编译**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint`

Expected: UI 组件无 lint 错误。

### Task 4: 登录页与权限状态统一

**Files:**
- Modify: `web/src/app/login/page.tsx`
- Modify: `web/src/components/permission-guard.tsx`

- [ ] **Step 1: 登录页升级为品牌卡片**

保留 `login("github")`、`login("google")` 和 OAuth callback 逻辑，只替换 JSX 外观：暖纸背景、深咖登录卡、手写感品牌标题、清晰错误提示。

- [ ] **Step 2: Processing 状态统一**

`processing` 分支使用同一背景和卡片，不再显示裸文本。

- [ ] **Step 3: 权限守卫状态统一**

`Loading...`、`Access Pending`、`Access Denied` 使用统一的居中纸张卡片。重定向和权限判断逻辑不变。

- [ ] **Step 4: 验证登录页编译**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint`

Expected: 登录页和权限守卫无 lint 错误。

### Task 5: 页面标题和关键区域收口

**Files:**
- Modify: `web/src/app/(dashboard)/page.tsx`
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`
- Modify: `web/src/app/(dashboard)/sessions/detail/page.tsx`
- Modify: `web/src/app/(dashboard)/apikeys/page.tsx`
- Modify: `web/src/app/(dashboard)/endpoints/page.tsx`
- Modify: `web/src/app/(dashboard)/models/page.tsx`
- Modify: `web/src/app/(dashboard)/profile/page.tsx`

- [ ] **Step 1: 一级标题使用 display 字体**

将各页面 `h1` class 增加 `font-display`、更大的字号和暖色前景；不改变文案和数据逻辑。

```tsx
<h1 className="font-display text-4xl font-bold tracking-tight text-foreground">
```

- [ ] **Step 2: Dashboard 关键区域微调**

`StatCard` 和 Quick Actions 使用通用 Card/Button 新质感；必要时仅添加少量 class 提升层级，不重写数据获取。

- [ ] **Step 3: 会话详情保持可读性**

聊天气泡和 Tool block 适配暖色，不改变排序、展开、JSON 展示逻辑。

- [ ] **Step 4: 管理表格页保持高对比**

API Keys、Endpoints、Models、Sessions 的表格内容仍用 sans/mono 字体，避免手写字体进入密集信息区。

- [ ] **Step 5: 全量前端验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint && npm run build`

Expected: lint 和静态导出构建均通过。

## Execution Notes

- 本计划不包含 git commit 步骤；除非用户明确要求，否则不提交。
- 每个任务完成后优先运行聚焦的 `npm run lint`，最终运行 `npm run build`。
- 若发现样式变更导致业务逻辑文件出现无关问题，不做顺手重构，只修复本次改动引入的问题。
- 字体使用本地字体栈，不依赖构建期下载 Google Fonts。
