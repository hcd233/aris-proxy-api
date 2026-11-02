# Web 门户页与全局偏好控件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/web/` 变为公开门户页（方案 C），原仪表盘首页迁到 `/web/dashboard/`，主题/语言切换收敛为一个在所有页面可用的 `PreferenceControls`。

**Architecture:** 纯前端改动。仪表盘首页 `page.tsx` 下沉到 `(dashboard)/dashboard/`，根 `app/page.tsx` 成为路由组外的门户（无侧栏、无 PermissionGuard）。偏好控件合并为单一组件，带 `tone` 区分侧栏与普通页面配色，删除 FAB 及根布局挂载。

**Tech Stack:** Next.js 16 静态导出（basePath `/web`、trailingSlash）、React 19、Tailwind v4、Base UI/shadcn、vitest。

**Spec:** `docs/superpowers/specs/2026-11-02-web-portal-design.md`

## Global Constraints

- 工作目录：`.worktrees/web-portal`，分支 `feature/web-portal-2026-11-02`；`web/node_modules` 是指向主工作区的软链，禁止 `npm ci`。
- 构建只能用 `npx next build --webpack`（Turbopack 拒绝跨 root 软链）；构建后删除 `web/.next`、`web/out`（用 python `shutil.rmtree`，不要用 rm）。
- 其他管理页路由一律不变；`/login`、`/callback`、`/share` 路径不变；后端零改动。
- 只用主题 token（`bg-background`、`text-foreground`、`text-muted-foreground`、`border-border`、`bg-card`、`bg-primary` 等）和 `moonshot:` 变体，不写死颜色值。
- 新增文案全部走 i18n，en / zh / ja 三份同时补齐，key 前缀 `portal.*`。
- GitHub 地址常量：`https://github.com/hcd233/aris-proxy-api`，外链 `target="_blank" rel="noopener noreferrer"`。
- 门户不新增任何 API 请求，auth 状态只读 `useAuth()`。
- 截断文案遵守 `truncate-requires-tooltip` lint；提交前跑 `npm run format`（CI 有 `format:check`）。
- 提交不得使用 `--no-verify`。

## Review Focus

1. 认证加载中（`isLoading=true`）访问 `/web/`：主按钮应稳定显示「登录控制台」，加载完成且已登录后才换成「进入控制台」，不得闪成空白或错位 → Task 3 `portalPrimaryCta` 单测覆盖三态。
2. 日语/英语文案比中文长：390 宽手机下首屏按钮、顶栏不得横向溢出 → Task 4 无头断言 `scrollWidth <= innerWidth`，覆盖 3 语言 × 2 主题 × 2 视口。
3. 已登录用户的旧书签 `/web/` 现在落在门户：顶栏和首屏都必须有一跳进入 `/dashboard/` 的入口 → 由第 1 条单测加 Task 4 的 DOM 断言（`a[href="/web/dashboard/"]` 存在）覆盖。
4. 侧边栏折叠态仍能切换主题和语言 → Task 2 在折叠分支挂控件，Task 4 用 CDP 点击折叠按钮后断言控件可见。
5. 仪表盘导航高亮：`/dashboard/` 不应在访问 `/sessions/` 时被高亮，`/audit/model/` 与 `/audit/cron/` 不能互相串 → Task 1 `isNavItemActive` 单测。

---

### Task 1: 仪表盘首页迁到 `/dashboard/`

**Files:**
- Move: `web/src/app/(dashboard)/page.tsx` → `web/src/app/(dashboard)/dashboard/page.tsx`（内容不变，用 `git mv`）
- Create: `web/src/lib/nav.ts`
- Test: `web/src/lib/nav.test.ts`
- Create: `web/src/app/page.tsx`（临时门户骨架，Task 3 填充）
- Modify: `web/src/app/(dashboard)/layout.tsx:86`（nav href）、`:197-199`（删除本地 `isItemActive`，改用 `isNavItemActive`）
- Modify: `web/src/app/login/page.tsx:70,102,118`、`web/src/app/callback/page.tsx:42`

**Interfaces:**
- Produces: `export const DASHBOARD_HOME_PATH = "/dashboard/"`（Next 路由路径，不带 basePath）；`export const DASHBOARD_HOME_URL = "/web/dashboard/"`（`window.location.href` 用）；`export function isNavItemActive(pathname: string, href: string): boolean`，均在 `web/src/lib/nav.ts`。

- [ ] **Step 1: Write the failing test** `web/src/lib/nav.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { DASHBOARD_HOME_PATH, DASHBOARD_HOME_URL, isNavItemActive } from "./nav";

describe("isNavItemActive", () => {
  it("highlights dashboard on its own path with or without trailing slash", () => {
    expect(isNavItemActive("/dashboard/", DASHBOARD_HOME_PATH)).toBe(true);
    expect(isNavItemActive("/dashboard", DASHBOARD_HOME_PATH)).toBe(true);
  });
  it("does not highlight dashboard on other pages or the portal", () => {
    expect(isNavItemActive("/sessions/", DASHBOARD_HOME_PATH)).toBe(false);
    expect(isNavItemActive("/", DASHBOARD_HOME_PATH)).toBe(false);
  });
  it("keeps sibling audit tabs apart and matches nested pages", () => {
    expect(isNavItemActive("/audit/cron/", "/audit/model/")).toBe(false);
    expect(isNavItemActive("/sessions/detail/", "/sessions/")).toBe(true);
  });
  it("exposes the basePath-qualified url", () => {
    expect(DASHBOARD_HOME_URL).toBe("/web/dashboard/");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/lib/nav.test.ts`
Expected: FAIL，报 `Failed to resolve import "./nav"`

- [ ] **Step 3: Implement `web/src/lib/nav.ts`**

做法：pathname 和 href 都先补齐尾部 `/` 再做 `startsWith`。usePathname 在 trailingSlash 下可能带也可能不带尾斜杠。

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/lib/nav.test.ts`
Expected: PASS（4 tests）

- [ ] **Step 5: 迁移路由并替换引用**

- 执行 `git mv "web/src/app/(dashboard)/page.tsx" "web/src/app/(dashboard)/dashboard/page.tsx"`。
- 在 layout 里把「仪表盘」那一项的 `href` 改成 `DASHBOARD_HOME_PATH`。`isItemActive` 的两处调用（L202、L296）改成 `isNavItemActive`，然后删掉本地函数。
- login 的 3 处和 callback 的 1 处，把 `"/web/"` 改成 `DASHBOARD_HOME_URL`。
- 新建 `web/src/app/page.tsx`：`"use client"` 组件，只渲染品牌名加一个 `<Link href={DASHBOARD_HOME_PATH}>`，保证 `/` 在 Task 3 之前不会 404。

- [ ] **Step 6: Verify**

Run: `cd web && npm run lint && npx tsc --noEmit && npm run test`
Expected: 0 error，全部测试通过

- [ ] **Step 7: Commit**

```bash
git add web/src && git commit -m "refactor(web): 仪表盘首页迁至 /dashboard/，导航高亮抽为 isNavItemActive"
```

---

### Task 2: 统一 `PreferenceControls` 并铺到所有页面

**Files:**
- Create: `web/src/components/preference-controls.tsx`
- Modify: `web/src/components/theme/theme-switcher.tsx`（删除 FAB 分支、`FAB_PATHS`、`usePathname`；新增 `tone` prop）
- Modify: `web/src/components/language-switcher.tsx`（新增 `tone` prop，`sidebar` 档套用侧栏配色）
- Modify: `web/src/app/layout.tsx`（删除 `<ThemeSwitcher />` 及其 import）
- Modify: `web/src/app/login/page.tsx:155-158`
- Modify: `web/src/app/callback/page.tsx`（成功态和错误态都加右上角控件）
- Modify: `web/src/app/share/page.tsx`（控件放进 header 右侧、工具按钮左边，loading/error 视图放右上角 absolute）
- Modify: `web/src/app/(dashboard)/layout.tsx` 中 `UserBar`（展开态替换 L396-399；折叠态新增竖排控件）

**Interfaces:**
- Produces: `export type PreferenceTone = "page" | "sidebar"`；`export function PreferenceControls({ tone = "page", orientation = "horizontal" }: { tone?: PreferenceTone; orientation?: "horizontal" | "vertical" }): JSX.Element`；`ThemeSwitcher({ tone }: { tone?: PreferenceTone })`；`LanguageSwitcher({ tone, compact }: { tone?: PreferenceTone; compact?: boolean })`。`compact=true` 时触发器只显示语言代码大写（EN/中/日），给折叠侧栏用。

- [ ] **Step 1: 实现组件**

配色规则：`page` 档使用 `text-foreground/70 hover:text-foreground hover:bg-secondary`；`sidebar` 档沿用现有的 `text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground`。`vertical` 用 `flex-col`。Tooltip 的 `side` 规则：横排时为 `bottom`，竖排时为 `right`。

- [ ] **Step 2: 替换各页面挂载点**

- 登录页：`<PreferenceControls />`。
- 仪表盘展开态：语言放左侧，主题和登出按钮放右侧，保持现有布局。把两者都传 `tone="sidebar"`。
- 仪表盘折叠态：在 UserBar 里 `collapsed` 分支内的头像下方加 `<PreferenceControls tone="sidebar" orientation="vertical" />`，语言用 `compact` 模式。
- 全仓 grep `variant="inline"` 和 `ThemeSwitcher variant`，确认没有残留。

- [ ] **Step 3: Verify**

Run: `cd web && npm run lint && npx tsc --noEmit && npm run test`
Expected: 0 error。`grep -rn "FAB_PATHS\|variant=\"inline\"" web/src` 无输出。

- [ ] **Step 4: Commit**

```bash
git add web/src && git commit -m "feat(web): 主题与语言切换收敛为全局 PreferenceControls，覆盖登录/回调/分享/仪表盘折叠态"
```

---

### Task 3: 门户页（方案 C）

**Files:**
- Create: `web/src/lib/portal.ts`
- Test: `web/src/lib/portal.test.ts`
- Create: `web/src/components/portal/portal-header.tsx`、`portal-hero.tsx`、`portal-flow.tsx`、`portal-pillars.tsx`、`portal-footer.tsx`
- Modify: `web/src/app/page.tsx`（替换 Task 1 骨架，组合以上组件）
- Modify: `web/src/locales/en.json`、`zh.json`、`ja.json`

**Interfaces:**
- Consumes: `DASHBOARD_HOME_PATH`（Task 1）、`PreferenceControls`（Task 2）、`useAuth()` 的 `{ user, isLoading }`。
- Produces（`web/src/lib/portal.ts`）：
  - `export const GITHUB_REPO_URL = "https://github.com/hcd233/aris-proxy-api"`
  - `export const LOGIN_PATH = "/login/"`
  - `export function portalPrimaryCta(state: { isLoading: boolean; hasUser: boolean }): { href: string; labelKey: "portal.cta.login" | "portal.cta.console" }`
  - `export const PORTAL_PILLARS: readonly { key: "access" | "curation" | "governance"; items: readonly string[] }[]`。items 是 i18n key，每组 4 项，对应 spec §3.1 第 4 点。
  - `export const PORTAL_FLOW: { clients: readonly string[]; upstreams: readonly string[] }`。值为 i18n key 或产品专名字面量。专名（Claude Code、Codex、OpenCode、OpenAI、Anthropic）不翻译。

- [ ] **Step 1: Write the failing test** `web/src/lib/portal.test.ts`

```ts
import { describe, expect, it } from "vitest";
import en from "@/locales/en.json";
import zh from "@/locales/zh.json";
import ja from "@/locales/ja.json";
import { GITHUB_REPO_URL, PORTAL_PILLARS, portalPrimaryCta } from "./portal";

describe("portalPrimaryCta", () => {
  it("shows login while auth is loading even if a stale user exists", () => {
    expect(portalPrimaryCta({ isLoading: true, hasUser: true })).toEqual({ href: "/login/", labelKey: "portal.cta.login" });
  });
  it("shows login for anonymous visitors", () => {
    expect(portalPrimaryCta({ isLoading: false, hasUser: false })).toEqual({ href: "/login/", labelKey: "portal.cta.login" });
  });
  it("routes signed-in users to the dashboard", () => {
    expect(portalPrimaryCta({ isLoading: false, hasUser: true })).toEqual({ href: "/dashboard/", labelKey: "portal.cta.console" });
  });
});

describe("portal copy", () => {
  it("points at the public repository", () => {
    expect(GITHUB_REPO_URL).toBe("https://github.com/hcd233/aris-proxy-api");
  });
  it("has three pillars of four items each", () => {
    expect(PORTAL_PILLARS.map((p) => p.key)).toEqual(["access", "curation", "governance"]);
    PORTAL_PILLARS.forEach((p) => expect(p.items).toHaveLength(4));
  });
  it("defines every portal.* key in en, zh and ja", () => {
    const keys = (d: Record<string, string>) => Object.keys(d).filter((k) => k.startsWith("portal.")).sort();
    expect(keys(en).length).toBeGreaterThan(0);
    expect(keys(zh)).toEqual(keys(en));
    expect(keys(ja)).toEqual(keys(en));
    for (const p of PORTAL_PILLARS) for (const k of p.items) expect(en).toHaveProperty([k]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/lib/portal.test.ts`
Expected: FAIL，报 `Failed to resolve import "./portal"`

- [ ] **Step 3: 实现 `portal.ts` 并补齐三语 `portal.*` 文案**

需要的 key：
- `portal.tagline`、`portal.subtitle`
- `portal.cta.login`、`portal.cta.console`、`portal.cta.github`
- `portal.flow.clients`、`portal.flow.gateway`、`portal.flow.gateway_desc`、`portal.flow.upstreams`、`portal.flow.maas`、`portal.flow.subscription`
- `portal.pillar.{access,curation,governance}.title`，以及每组 4 个 `.item_N`
- `portal.footer.source`

中文参考：标语「一个地址，接入所有大模型」，副标题「模型聚合网关 + 使用数据沉淀平台」。其他文案按 README Core Capabilities 提炼。

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/lib/portal.test.ts`
Expected: PASS（6 tests）

- [ ] **Step 5: 实现门户组件与 `app/page.tsx`**

- **header**：左侧是 A 字标加「Aris」（样式参照登录页 L163-168）。右侧依次是 `PreferenceControls`、GitHub 图标链接（`ProviderIcon` 里有 GitHub 图标就复用，没有就用 lucide `Github`）、主按钮。
- **hero**：居中放 `font-display` 标语和副标题。主按钮用 `bg-foreground text-background`，同项目登录页风格；次按钮用 outline 加外链箭头。
- **flow**：三列。窄于 `md` 时改为纵向堆叠，箭头随之旋转 90°。中间节点用 `border-primary` 高亮。
- **pillars**：`md:grid-cols-3`，每张卡顶部 2px 色带，三组分别用 `--primary`、`--chart-2`、`--chart-3`。
- **footer**：产品名加 GitHub 链接。
- 整页套用 `page-surface` 和 `stagger-rise` 入场。

- [ ] **Step 6: Verify**

Run: `cd web && npm run format && npm run lint && npx tsc --noEmit && npm run test`
Expected: 0 error，全部测试通过

- [ ] **Step 7: Commit**

```bash
git add web/src && git commit -m "feat(web): 新增门户页（链路示意 + 接入/沉淀/治理三支柱，三语）"
```

---

### Task 4: 构建产物验证与文档同步

**Files:**
- Modify: `docs/agents/web-frontend.md`（路由组描述：`/` 为门户，`(dashboard)/dashboard/` 为仪表盘首页）
- Temp: `/tmp/portal-check/`（无头验证脚本，不入库）

- [ ] **Step 1: 构建**

Run: `cd web && npx next build --webpack && ls out/index.html out/dashboard/index.html`
Expected: 构建成功，两个文件都存在。

- [ ] **Step 2: 无头断言矩阵**

在 `out/` 下执行 `ln -s . web`，然后用 `python3 -m http.server` 托管。用 chrome-headless-shell 加注入脚本做 DOM 断言，localStorage 预置 `theme` 和 `locale`。覆盖 {anthropic, moonshot} × {en, zh, ja} × {1440×900, 390×844}，共 12 组。

每组断言：
- `document.documentElement.scrollWidth <= innerWidth`
- 页面文本中不出现 `portal.` 字面量
- `a[href="https://github.com/hcd233/aris-proxy-api"]` 至少 2 个，且都带 `rel` 含 `noopener`
- 主按钮 `a[href="/web/login/"]` 存在

另外，在 `/web/login/`、`/web/share/`、`/web/dashboard/`（未登录会跳转，只验证 login 和 share 即可）上，断言存在主题按钮（`aria-label` 为 `theme.to_*` 的翻译）和语言触发器。

Expected: 12/12 门户组合通过，login 和 share 断言通过。

- [ ] **Step 3: 手工路径（需要用户本地登录态，交付时列给用户复验）**

- 已登录访问 `/web/`，按钮显示「进入控制台」。
- 侧栏「仪表盘」能正确高亮。
- 折叠侧栏后能切换主题和语言。
- `/web/sessions/` 正常打开。

- [ ] **Step 4: 清理与文档**

用 python `shutil.rmtree` 删除 `web/.next` 和 `web/out`，再更新 `docs/agents/web-frontend.md`。

- [ ] **Step 5: Commit**

```bash
git add docs/agents/web-frontend.md && git commit -m "docs(agents): 同步门户页与仪表盘路由结构"
```
