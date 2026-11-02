# Portal 长页重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `/web/` 门户页从 4 个区块扩成一张 9 个区块的长页，参考 Lusmu 的结构与动效，两个主题都要达到定稿视觉稿的观感。

**Architecture:**
- **数据层**：演示数据和文案 key 全部集中在 `src/lib/portal.ts`。纯函数（曲线路径、熔断脚本帧、循环是否运行）放在 `src/lib/portal-motion.ts`，用 vitest 覆盖。
- **组件层**：区块组件放在 `src/components/portal/` 下，按区块拆文件，并复用 web 端已有组件（`ProviderIcon`、`ScoreDots`、`Bubble`、`MetricNumber`、`Progress`、`Badge`、`Button`、`SplitText`、`PreferenceControls`）。
- **样式层**：门户专属样式全部写在 `src/styles/portal.css`，并按 motion tier 降级。其中包括跑马灯、滚动揭示、星空、柔光、呼吸光点。

**Tech Stack:** Next.js 16.3（静态导出，`basePath: "/web"`），React 19，Tailwind v4，shadcn（base-nova + `@base-ui/react`），`@lobehub/icons`，vitest。

**Spec:** `docs/superpowers/specs/2026-11-02-portal-longpage-design.md`
**视觉稿：** `docs/superpowers/specs/2026-11-02-portal-longpage-mockup.html`。它是视觉真值，实现前先用浏览器打开对照。

## Global Constraints

- **工作目录**：所有命令都在 worktree `.worktrees/portal-longpage/web` 下执行。`node_modules` 先用软链接入：`ln -s ../../../web/node_modules node_modules`，禁止 `npm ci`。构建用 `npx next build --webpack`。
- **不加依赖**：不新增 npm 依赖，不引入动画库，不用 canvas 或 WebGL。
- **样式**：
  - 只用 Tailwind v4 + `cn()` 和 `globals.css` 里的语义 token，不写死 hex。
  - 逐元素的 CSS 变量（如 `--i`、`--delay`）可以通过 `style` 传入，这是 `SplitText` 的先例。除此之外不写内联定值样式。
  - 用 `gap-*` 而不是 `space-*`；宽高相等时用 `size-*`；组件内的图标不加尺寸类；按钮里的图标用 `data-icon`。
- **图标**：厂商和客户端图标只走 `ProviderIcon`，其他图标用 `lucide-react`。按钮文字后面不手工拼「→」。
- **动效档位**：
  - 档位只读 `useMotionTier()`（`@/lib/motion/tier-store`），类型是 `MotionTier = "immersive" | "lite" | "still"`。
  - CSS 侧用 `html[data-motion-tier="…"]` 命中。
  - 入场动画用 `backwards` 填充。
  - moonshot 下不要对包含玻璃卡片的祖先元素做 `filter` 动画。
- **i18n**：所有可见文案都走 `useT()`（`@/lib/i18n`），key 用 `portal.*` 前缀，en / zh / ja 三份必须齐全。中文文案以 spec 为准。en / ja 的小标签不用全大写字母。
- **演示数据中的模型名一律小写**：
  - claude-sonnet-5-5、claude-opus-5-5
  - gpt-6.1-sol
  - deepseek-v4-pro
  - qwen3.8-max
  - minimax-m3.1-flash
  - glm-5.3
  - hy4-preview
- **名称写法固定**：
  - 客户端是 Claude Code、Codex、OpenCode、Pi、DeepSeek Harness；DeepSeek Harness 用 deepseek 图标。
  - 厂商是 OpenAI、Anthropic、DeepSeek、Qwen、Hy、MiniMax、GLM；Hy 用 hunyuan 图标，GLM 用 zai 图标。
  - 订阅类上游是「Codex 订阅」和「Claude Code 订阅」。
- **验收命令**：`npm run lint && npm run test && npm run format:check && npx next build --webpack`。

## Review Focus

1. **390px 窄屏**：请求链路三栏必须改为纵向排列，虚线要按新布局重新计算，拼图网格和步骤区改成单栏，页面不出现横向滚动。由 Task 6 / Task 7 / Task 10 的程序化断言覆盖。
2. **en / ja 长文案**：切换语言后，功能卡、能力卡和导航不能溢出，也不能让布局跳动。由 Task 10 在三种语言下跑溢出断言覆盖。
3. **still 档 / 系统开启「减弱动态」**：所有自动轮播和打字都要停下，内容直接以完整态可见，不能卡在透明度为 0 的状态。由 Task 3 的 `shouldRunLoop` 单测和 Task 10 的 still 档断言覆盖。
4. **标签页切到后台**：JS 驱动的循环要暂停，切回来后能继续，也不会一次补跑大量积压的帧。由 Task 3 的 `shouldRunLoop` 单测覆盖。
5. **字体加载、语言切换或主题切换后节点位置改变**：链路虚线必须跟着重算，不能错位。由 Task 6 用 `ResizeObserver` 监听容器解决，并在 Task 10 断言 14 条路径的端点都落在节点边缘 ±2px 内。

---

### Task 1: ProviderIcon 补客户端图标

**Files:**
- Modify: `web/src/components/provider-icon.tsx`（import 列表和 `providerMap`）
- Create: `web/src/components/provider-icon.test.ts`

**Interfaces:**
- Produces:
  - `findProviderKey(protocol: string): string | undefined` 改为 `export`，供测试使用；
  - `providerMap` 新增 key：`claudecode`（ClaudeCode）、`codex`（Codex）、`opencode`（OpenCode）、`pi`（Pi）。
  - 后续任务都通过 `<ProviderIcon protocol="claudecode" />` 这种写法取图标。

- [ ] **Step 1：写失败测试 `provider-icon.test.ts`**

```ts
import { describe, expect, it } from "vitest";
import { findProviderKey } from "./provider-icon";

describe("findProviderKey", () => {
  it.each([
    ["claudecode", "claudecode"], ["codex", "codex"], ["opencode", "opencode"], ["pi", "pi"],
    ["claude", "claude"], ["deepseek", "deepseek"], ["hunyuan", "hunyuan"], ["zai", "zai"],
  ])("%s -> %s", (input, key) => expect(findProviderKey(input)).toBe(key));
  it("does not let 'claude' swallow 'claudecode'", () => expect(findProviderKey("claudecode")).not.toBe("claude"));
  it("still maps model ids by prefix", () => expect(findProviderKey("deepseek-v4-pro")).toBe("deepseek"));
});
```

- [ ] **Step 2：运行测试，确认失败**
  - 命令：`npx vitest run src/components/provider-icon.test.ts`
  - 预期：FAIL，原因是 `findProviderKey` 没有被导出，或者 `claudecode` 被解析成 `claude`。
  - 注意：`providerMap` 的 key 顺序决定 `find` 的结果。`claudecode` 要放在 `claude` **之前**；`pi` 不能误匹配 `pixtral` 这类名字（现有前缀规则要求紧跟的字符不是小写字母，满足这一点）。
- [ ] **Step 3：实现。** 从 `@lobehub/icons` 导入 `ClaudeCode, Codex, OpenCode, Pi`，加入 `providerMap`，并 `export function findProviderKey`。
- [ ] **Step 4：重新运行测试，确认 PASS**
- [ ] **Step 5：提交**：`git commit -m "feat(web): ProviderIcon 支持 Claude Code / Codex / OpenCode / Pi 客户端图标"`

---

### Task 2: 门户数据模型与三语文案

**Files:**
- Modify: `web/src/lib/portal.ts`（删除 `PORTAL_PILLARS` 和 `PORTAL_FLOW`，换成下面的导出）
- Modify: `web/src/lib/portal.test.ts`
- Modify: `web/src/locales/{zh,en,ja}.json`（删掉旧的 `portal.pillar*`、`portal.flow.maas`、`portal.flow.subscription`，新增本任务列出的 key）

**Interfaces:**
- Produces（全部 `export`，以 `as const` 定义为只读字面量）：

```ts
export type PortalEntity = { id: string; icon: string; label: string; labelKey?: string };
// label 是专名，原样展示；labelKey 存在时优先用 t(labelKey)（如「Codex 订阅」）
export const PORTAL_CLIENTS: readonly PortalEntity[];   // 5 个：claudecode/codex/opencode/pi/deepseek-harness(icon deepseek)
export const PORTAL_VENDORS: readonly PortalEntity[];   // 7 个：openai/anthropic/deepseek/qwen/hy(icon hunyuan)/minimax/glm(icon zai)
export const PORTAL_UPSTREAMS: readonly PortalEntity[]; // 9 个 = PORTAL_VENDORS + codex-sub(icon codex) + claudecode-sub(icon claudecode)
export const PORTAL_GATEWAY_STAGES: readonly string[];  // 4 个 i18n key：auth/route/breaker/audit
export const PORTAL_MODELS: Readonly<Record<"sonnet"|"opus"|"gpt"|"deepseek"|"qwen"|"minimax"|"glm"|"hy", string>>;
export type PortalFeatureKey = "single_endpoint" | "sessions" | "export_cli" | "usage" | "self_heal" | "dataset";
export const PORTAL_FEATURES: readonly PortalFeatureKey[]; // 顺序同 spec §5.3
export type PortalAuditRow = { icon: string; model: string; tokens: number; firstTokenMs: number | null; status: number };
export const PORTAL_AUDIT_ROWS: readonly PortalAuditRow[]; // 8 行，模型名取自 PORTAL_MODELS，其中 minimax 那行 status 502、firstTokenMs null
export type PortalRouteTier = { priority: 1 | 2; endpoints: readonly { icon: string; label: string; labelKey?: string; weight: number }[] };
export const PORTAL_ROUTE: { alias: "sonnet"; tiers: readonly PortalRouteTier[] }; // P1: anthropic 70 / claudecode-sub 30；P2: deepseek 100 / glm 100
export type PortalStepKey = "login" | "upstream" | "install" | "use";
export const PORTAL_STEPS: readonly PortalStepKey[];
export const PORTAL_TTY_LINES: readonly { kind: "cmd" | "out" | "ask" | "ok" | "blank"; text: string; ms: number }[];
// 终端内容逐行对应 spec §5.7 第 03 步（专名、路径、命令原样展示，不翻译）
export const PORTAL_SECTION_IDS: { flow: "flow"; capabilities: "capabilities"; getStarted: "get-started" };
```

- 保留不变：`GITHUB_REPO_URL`、`LOGIN_PATH`、`portalPrimaryCta`、`PortalCtaLabelKey`。
- 文案 key 规划，三语都要写：
  - `portal.nav.{flow,capabilities,get_started,github}`
  - `portal.feature.<key>.{title,desc}`
  - `portal.eco.title`
  - `portal.flow.{eyebrow,title,subtitle,clients,upstreams,stage.*}`
  - `portal.cap.<access|curation|governance|resilience>.{eyebrow,title,desc,chip_1..n}` 以及各示例面板的标签 key
  - `portal.steps.{eyebrow,title,subtitle}`、`portal.step.<key>.{title,desc,chip_1..n,window}`
  - `portal.upstream.codex_sub`、`portal.upstream.claudecode_sub`
  - `portal.cta.{title,subtitle}`
  - `portal.footer.{product,resources,open_source,...}`
  - 已有的 `portal.kicker/tagline/subtitle/cta.*` 保留。

- [ ] **Step 1：在 `portal.test.ts` 中把旧的「three pillars」用例换成下面这组失败测试**

```ts
it("lists the five export targets as clients", () =>
  expect(PORTAL_CLIENTS.map((c) => c.label)).toEqual(["Claude Code", "Codex", "OpenCode", "Pi", "DeepSeek Harness"]));
it("uses deepseek icon for DeepSeek Harness", () =>
  expect(PORTAL_CLIENTS.find((c) => c.id === "deepseek-harness")?.icon).toBe("deepseek"));
it("expands nine upstreams including both subscriptions", () => {
  expect(PORTAL_UPSTREAMS).toHaveLength(9);
  expect(PORTAL_UPSTREAMS.map((u) => u.labelKey).filter(Boolean)).toEqual(["portal.upstream.codex_sub", "portal.upstream.claudecode_sub"]);
  expect(PORTAL_VENDORS.map((v) => v.label)).toEqual(["OpenAI", "Anthropic", "DeepSeek", "Qwen", "Hy", "MiniMax", "GLM"]);
});
it("keeps every demo model id lowercase", () => {
  const ids = [...Object.values(PORTAL_MODELS), ...PORTAL_AUDIT_ROWS.map((r) => r.model)];
  ids.forEach((m) => expect(m).toBe(m.toLowerCase()));
  expect(PORTAL_MODELS.minimax).toBe("minimax-m3.1-flash");
});
it("never says --target in the export demo", () =>
  expect(PORTAL_TTY_LINES.some((l) => l.text.includes("--target"))).toBe(false));
it("keeps implementation jargon out of feature cards", () => {
  for (const k of PORTAL_FEATURES) for (const f of ["title", "desc"] as const)
    expect(zh[`portal.feature.${k}.${f}` as keyof typeof zh]).not.toMatch(/checksum|内容寻址|去重/);
});
```

  同时保留已有的「defines every portal.* key in en, zh and ja」用例，并把其中遍历 `PORTAL_PILLARS` 的那一行，改成遍历 `PORTAL_FEATURES` 和 `PORTAL_STEPS` 拼出来的 key。

- [ ] **Step 2：运行 `npx vitest run src/lib/portal.test.ts`**，预期 FAIL。
- [ ] **Step 3：实现 `portal.ts` 的导出，并补齐三语 locale。** 中文照抄 spec §5；en / ja 意译，句式简短，不写成营销腔。
- [ ] **Step 4：运行 `npx vitest run src/lib/portal.test.ts`**，预期 PASS。此时旧组件会因为引用了已删除的导出而编译失败，这一点 Task 9 会处理。这一步只验证单测，不跑 build。
- [ ] **Step 5：提交**：`git commit -m "feat(web): 门户长页演示数据与三语文案"`

---

### Task 3: 门户动效基础（纯函数、hook、样式层）

**Files:**
- Create: `web/src/lib/portal-motion.ts`、`web/src/lib/portal-motion.test.ts`
- Create: `web/src/components/portal/use-portal-loop.ts`
- Create: `web/src/components/portal/reveal.tsx`
- Create: `web/src/styles/portal.css`，并在 `web/src/app/globals.css` 的 `@import "../styles/motion-signature.css";` 之后加上 `@import "../styles/portal.css";`

**Interfaces:**
- Produces:

```ts
// portal-motion.ts
export function shouldRunLoop(tier: MotionTier, hidden: boolean): boolean; // still 或 hidden → false
export function curvePath(x1: number, y1: number, x2: number, y2: number): string; // "M x1 y1 C mx y1 mx y2 x2 y2"，mx=(x1+x2)/2
export type BreakerFrame = { state: "closed" | "open" | "half_open"; cells: readonly ("idle"|"ok"|"err"|"rejected"|"probe")[]; errorPct: number | null; captionKey: string; holdMs: number };
export function breakerFrames(): readonly BreakerFrame[]; // 按 spec §5.6 容错脚本生成，图案 "ggrgrrgrrr"，第 10 格时 errorPct=60 并切到 open
// use-portal-loop.ts
export function usePortalLoop(step: () => void, intervalMs: number): void; // 内部用 useMotionTier + visibilitychange；shouldRunLoop 为 false 时不调度；卸载时清理
// reveal.tsx
export function Reveal(props: { as?: "div" | "section"; className?: string; children: React.ReactNode; id?: string }): JSX.Element;
// 首次进入视口（IntersectionObserver，threshold 0.12）时加上 data-revealed，之后不再观察；still 档初始即为 revealed
```

- `portal.css` 提供以下类，后续任务直接使用：
  - `.portal-root`：门户根容器。
  - `.portal-stars`：只在 `html[data-theme="moonshot"] .portal-root` 下生效，6 层 `radial-gradient` 白色点阵（透明度 .35–.6，平铺 600px），静态；`immersive` 档下 120s 线性缓慢平移。
  - `.portal-glow`：Hero 柔光，anthropic 用 `--chart-3` 与 `--accent` 的混色，moonshot 用 `--chart-4` / `--chart-3`；`immersive` 和 `lite` 下 12s 来回漂移，`still` 静止。
  - `.portal-eyebrow`：小号文字，前面带 6px 圆点，2.4s 呼吸 ping；moonshot 下额外加描边胶囊。
  - `.portal-marquee`：外层容器，带左右渐隐遮罩。
  - `.portal-marquee-track`：`width:max-content`，`translateX(-50%)` 无限动画，时长取 `--marquee-duration`，`data-reverse` 时反向；外层 `:hover` 或 `:focus-within` 时暂停；`still` 档下 `animation:none`，外层改为 `overflow-x:auto`。
  - `[data-reveal]` / `[data-revealed]`：透明度加 `translateY(28px)` 揭示，`immersive` 档额外加 `blur(4px)`，`lite` 档不加 blur。moonshot 下 blur 只作用于 `[data-reveal] > *`，不作用在根元素上。
  - `.portal-hero-title .split-char`：逐字升起（`translateY(110%) rotate(3deg)` → none，步进 `calc(var(--i) * 40ms)`，`backwards` 填充）。
  - `.portal-dash`：SVG 虚线，`stroke-dasharray: 4 6`，`dashoffset` 循环动画；`still` 档静止。
- 所有颜色只引用 `var(--…)` token。

- [ ] **Step 1：写 `portal-motion.test.ts` 失败测试**

```ts
it("stops loops in still tier or when hidden", () => {
  expect(shouldRunLoop("immersive", false)).toBe(true);
  expect(shouldRunLoop("lite", false)).toBe(true);
  expect(shouldRunLoop("still", false)).toBe(false);
  expect(shouldRunLoop("immersive", true)).toBe(false);
});
it("builds a horizontal-tangent cubic", () => expect(curvePath(0, 10, 100, 50)).toBe("M0 10C50 10 50 50 100 50"));
it("breaker script opens at 60% and recovers via a single probe", () => {
  const f = breakerFrames();
  const open = f.findIndex((x) => x.state === "open");
  expect(f[open - 1].errorPct).toBe(60);
  expect(f.slice(open).find((x) => x.state === "half_open")?.cells.filter((c) => c === "probe")).toHaveLength(1);
  expect(f.at(-1)?.state).toBe("closed");
  f.forEach((x) => expect(x.cells).toHaveLength(10));
});
```

- [ ] **Step 2：运行 `npx vitest run src/lib/portal-motion.test.ts`**，预期 FAIL。
- [ ] **Step 3：实现 `portal-motion.ts`、`usePortalLoop`、`Reveal` 和 `portal.css`。**
  - `usePortalLoop` 用 `setTimeout` 链而不是 `setInterval`，每一轮都重新检查 `shouldRunLoop`。这样切回前台时不会一次补跑积压的回调（对应 Review Focus 第 4 条）。
- [ ] **Step 4：运行 `npx vitest run src/lib/portal-motion.test.ts`**，预期 PASS。
- [ ] **Step 5：提交**：`git commit -m "feat(web): 门户动效基础（循环守卫、揭示、跑马灯、星空样式层）"`

---

### Task 4: 悬浮导航 + Hero + 页面骨架

**Files:**
- Modify（重写）: `web/src/components/portal/portal-hero.tsx`，导出 `PortalHeader`、`PortalHero`
- Modify: `web/src/app/page.tsx`

**Interfaces:**
- Consumes：
  - `portalPrimaryCta`、`PORTAL_SECTION_IDS`（Task 2）
  - `.portal-root`、`.portal-stars`、`.portal-glow`、`.portal-eyebrow`、`.portal-hero-title`（Task 3）
  - `SplitText`、`PreferenceControls`、`Button`、`ProviderIcon`
- Produces：
  - `PortalHeader({ cta })` 和 `PortalHero({ cta })`，签名不变；
  - 新导出 `PortalEyebrow({ children })`，供各区块的小标签复用；
  - 新导出 `PortalSectionHead({ eyebrowKey, titleKey, subtitleKey? })`，渲染「eyebrow → h2 → 副文案」三层，供 Task 5–8 使用。

- [ ] **Step 1：`page.tsx` 改为最终的区块顺序。** 暂时只挂 `PortalHeader` 和 `PortalHero`，其余区块在后续任务中依次接入。根节点加 `portal-root` 类，并在内部放一个 `<div aria-hidden className="portal-stars" />`。
- [ ] **Step 2：实现悬浮药丸导航（spec §5.1）。**
  - 结构：`sticky top-3.5` 外层居中，内层 `max-w-[720px] rounded-full`，背景 `bg-card/80` + `backdrop-blur-md` + `shadow-lg`。
  - 锚点用 `<a href="#flow">` 等。这是同页锚点，不经过 `next/link`，也不会被拼上 basePath。
  - `md` 以下隐藏锚点链接。
- [ ] **Step 3：实现 Hero（spec §5.2）。**
  - h1 用 `<SplitText text={t("portal.tagline")} className="portal-hero-title" />`，字号 `font-display text-[clamp(2.75rem,7vw,5.5rem)] leading-[1.04] tracking-[-0.045em] font-normal`；moonshot 用文字渐变类。
  - 柔光 `div.portal-glow` 绝对定位在 Hero 区块内部。
  - 按钮：主 CTA 加 `ArrowRight data-icon="inline-end"`；GitHub 按钮加 `ArrowUpRight data-icon="inline-end"`。
- [ ] **Step 4：验证。** 运行 `npm run lint`，只允许出现「其他门户旧文件引用已删除导出」这一类错误，Task 9 会清理它们。
- [ ] **Step 5：提交**：`git commit -m "feat(web): 门户悬浮导航与 Hero"`

---

### Task 5: 功能卡跑马灯 + 兼容生态两行跑马灯

**Files:**
- Create: `web/src/components/portal/portal-feature-marquee.tsx`（导出 `PortalFeatureMarquee`）
- Create: `web/src/components/portal/feature-art.tsx`：6 个迷你插图组件，统一签名 `(props: {}) => JSX.Element`，按 `PortalFeatureKey` 映射为 `FEATURE_ART`
- Create: `web/src/components/portal/portal-ecosystem.tsx`（导出 `PortalEcosystem`）
- Modify: `web/src/app/page.tsx`，在 Hero 之后依次接入这两个区块

**Interfaces:**
- Consumes：`PORTAL_FEATURES`、`PORTAL_CLIENTS`、`PORTAL_VENDORS`、`PORTAL_MODELS`（Task 2），`.portal-marquee*`（Task 3），`ScoreDots`、`Bubble`/`BubbleContent`、`MetricNumber`、`Progress`、`Badge`、`ProviderIcon`、`formatTokenCount`、`formatCost`、`formatPct`。
- **跑马灯规则**：内容渲染两份。第二份整体加 `aria-hidden` 和 `inert`，保证读屏器和键盘焦点都只经过一份。
- **6 张卡的插图**（spec §5.3）：

| 卡 | 插图要点 |
|---|---|
| sessions | `ScoreDots` 用本地 `useState` 管理分数（初始 4），`scoring={false}` |
| usage | 用 `MetricNumber` 显示三个数（Token 126.4k、估算费用、成功率），曲线用内联 SVG `path`，颜色引用 `var(--primary)` |
| dataset | 用 `Progress` 循环递增，靠 `usePortalLoop` 驱动 |
| self_heal | 一个红点脉冲 |

- 卡片宽 `w-[372px] shrink-0`，悬停上浮 `-translate-y-1.5`。上浮只用 transform，不改变布局尺寸。

- [ ] **Step 1：实现两个区块，并在 `page.tsx` 接入。**
- [ ] **Step 2：运行 `npm run lint && npx tsc --noEmit -p .`**，预期这两个新文件没有错误。
- [ ] **Step 3：提交**：`git commit -m "feat(web): 门户功能卡跑马灯与兼容生态条"`

---

### Task 6: 请求链路

**Files:**
- Create: `web/src/components/portal/portal-flow.tsx`（覆盖旧文件，导出 `PortalFlow`）

**Interfaces:**
- Consumes：`PORTAL_CLIENTS`、`PORTAL_UPSTREAMS`、`PORTAL_GATEWAY_STAGES`、`PORTAL_SECTION_IDS.flow`（Task 2），`curvePath`、`usePortalLoop`、`.portal-dash`（Task 3），`Reveal`、`PortalSectionHead`（Task 3、Task 4）。
- **DOM 标记约定**（Task 10 的断言依赖它们）：
  - 每个节点加 `data-flow-node="client|upstream"`；
  - 核心加 `data-flow-core`；
  - SVG 路径加 `data-flow-path="in|out"`。

- [ ] **Step 1：实现三栏布局。**
  - `lg` 及以上：`grid-cols-[14rem_1fr_14rem]`；
  - `lg` 以下：纵向排列，客户端一行（换行网格）→ 核心 → 上游一行（换行网格），虚线改成纵向连线。
- [ ] **Step 2：虚线几何。**
  - 用 `useLayoutEffect` 和 `ResizeObserver` 监听容器，按节点和核心的 `getBoundingClientRect` 计算端点，再用 `curvePath` 生成 `d` 写入 state。
  - 同时监听 `document.fonts.ready` 和 locale 变化，触发重算（对应 Review Focus 第 5 条）。
  - 入线端点按客户端序号在核心左边沿上分散排布，出线在右边沿同理。
- [ ] **Step 3：请求演示。**
  - 用 `usePortalLoop` 驱动一个状态机：`idle → in(i) → stages(0..3) → out(j) → idle`，`i` 和 `j` 随机。
  - 光点是一个 SVG `circle`，用 rAF 配合 `getPointAtLength` 沿路径移动，时长 720ms。
  - `lite` 档不显示光点，只保留虚线流动；`still` 档只显示静态虚线。
  - 整个 SVG 加 `aria-hidden`；节点名字是普通文本，读屏器可读。
- [ ] **Step 4：在 `page.tsx` 接入，运行 `npm run lint`。**
- [ ] **Step 5：提交**：`git commit -m "feat(web): 门户请求链路（全量上游 + 动态虚线）"`

---

### Task 7: 四大能力拼图网格

**Files:**
- Create: `web/src/components/portal/portal-capabilities.tsx`（导出 `PortalCapabilities`）
- Create: `web/src/components/portal/capability-demos.tsx`（导出 `RouteDemo`、`ProtocolDemo`、`SessionDemo`、`DatasetDemo`、`AuditStreamDemo`、`PermissionLadder`、`TriggerActions`、`BreakerDemo`、`BulkheadDemo`）
- Delete: `web/src/components/portal/portal-pillars.tsx`，其中的 `PortalFooter` 由 Task 9 重建

**Interfaces:**
- Consumes：`PORTAL_ROUTE`、`PORTAL_AUDIT_ROWS`、`PORTAL_MODELS`（Task 2），`breakerFrames`、`usePortalLoop`（Task 3），`ScoreDots`、`Progress`、`Badge`、`ProviderIcon`、`formatTokenCount`、`formatDurationMs`。
- **布局**：`grid-cols-12`。卡片按「接入 7 / 沉淀 5」「治理 5 / 容错 7」排列；`lg` 以下每张卡都占满 12 列。卡片 `rounded-3xl bg-card shadow-sm p-7`，内部示例面板 `rounded-2xl bg-muted/50 p-4`。
- **各演示规格**（全部取自 spec §5.6）：

| 演示 | 规格 |
|---|---|
| `RouteDemo` | 按权重序列 `[0,0,1,0,0,1,0]` 轮换高亮，步进 1700ms |
| `AuditStreamDemo` | 每 1900ms 在顶部插入一行，最多保留 4 行；status ≥ 500 的行用 `text-destructive` |
| `BreakerDemo` | 逐帧播放 `breakerFrames()`，每帧停留 `holdMs` |
| `BulkheadDemo` | 32 格占用条，每 900ms 随机游走一次（取值钳制在 3–32） |

- `still` 档下，各演示停在 spec §9 规定的代表帧：

| 演示 | still 档停留画面 |
|---|---|
| 路由 | 选中第 1 个端点 |
| 审计 | 显示前 4 行 |
| 熔断 | 停在 OPEN 帧 |
| 并发 | 占用 9 格 |

- 要点标签用 `Badge variant="outline"`。

- [ ] **Step 1：实现，并在 `page.tsx` 接入。**
- [ ] **Step 2：运行 `npm run lint && npx vitest run`**，预期全部通过。
- [ ] **Step 3：提交**：`git commit -m "feat(web): 门户四大能力图解（路由/协议/会话/导出/审计/权限/触发词/熔断/隔离）"`

---

### Task 8: 接入四步

**Files:**
- Create: `web/src/components/portal/portal-get-started.tsx`（导出 `PortalGetStarted`）
- Create: `web/src/components/portal/step-scenes.tsx`（导出 `ApiKeyScene`、`UpstreamScene`、`TerminalScene`、`SessionScene`）

**Interfaces:**
- Consumes：`PORTAL_STEPS`、`PORTAL_TTY_LINES`、`PORTAL_ROUTE`、`PORTAL_MODELS`、`PORTAL_SECTION_IDS.getStarted`（Task 2），`usePortalLoop`（Task 3），`Bubble`、`ScoreDots`、`Badge`、`Progress`、`ProviderIcon`。
- **步骤列表**：
  - 左栏用 `<ol>`，每一步是一个 `<button aria-expanded>`。
  - 当前步骤展开说明和标签，底部有一条 7s 进度条（CSS 动画，`still` 档隐藏）。
  - 自动切换由 `usePortalLoop(next, 7000)` 驱动。用户点击任意一步后，设置 `userPicked = true`，从此停止自动切换（spec §11.1）。
- **右栏窗口**：`lg` 以上 `sticky top-24`，窗口栏标题取 `t("portal.step.<key>.window")`。
- **`TerminalScene`**：
  - 进入第 03 步时按 `PORTAL_TTY_LINES[i].ms` 逐行显示，末尾闪烁光标；
  - `still` 档直接显示全部行；
  - 离开该步时清理定时器。

- [ ] **Step 1：实现，并在 `page.tsx` 接入。**
- [ ] **Step 2：运行 `npm run lint`。**
- [ ] **Step 3：提交**：`git commit -m "feat(web): 门户接入四步（手风琴 + 演示窗口 + 终端回放）"`

---

### Task 9: 收尾 CTA、大页脚、清理旧代码

**Files:**
- Create: `web/src/components/portal/portal-closing.tsx`（导出 `PortalCta({ cta })`、`PortalFooter()`）
- Modify: `web/src/app/page.tsx`，完成最终组装
- 确认已删除旧代码：`portal-pillars.tsx`，以及旧 `portal-flow.tsx` 里不再使用的 `FlowColumn` / `FlowArrow`；locale 里不留无人引用的 `portal.*` key

**Interfaces:**
- Consumes：`portalPrimaryCta` 的返回值，`GITHUB_REPO_URL`，`PORTAL_SECTION_IDS`。

- [ ] **Step 1：实现收尾 CTA（spec §5.8）。** 卡片 `rounded-[28px] p-14 min-h-[340px]`，渐变卡用 `portal.css` 新增的 `.portal-cta-surface`，两个主题各写一套。
- [ ] **Step 2：实现页脚（spec §5.9）。**
  - 四栏链接：控制台（指向 cta.href）、页内锚点、GitHub。
  - 底部超大号「Aris」字标用 `aria-hidden`。
- [ ] **Step 3：清理 locale 中无人引用的 key。** 在三份 locale 中搜索每个 `portal.*` key 是否在 `src` 里出现，删掉没有出现的。`portal.test.ts` 的三语一致性用例必须仍然通过。
- [ ] **Step 4：跑全量门禁**：`npm run lint && npm run test && npm run format && npm run format:check && npx next build --webpack`，预期全部通过。
- [ ] **Step 5：提交**：`git commit -m "feat(web): 门户收尾 CTA 与大页脚，移除旧支柱区块"`

---

### Task 10: 程序化视觉验收与契约回写

**Files:**
- Create（不提交）：`/tmp/portal-verify/check.mjs`，基于 chrome-headless-shell CDP，按 memory 中已验证的链路实现
- Modify: `docs/agents/web-frontend.md`，在动效契约「页面背景保持纯色」一条后追加 spec §10 第 3 条的例外说明

**Interfaces:**
- Consumes：Task 6 的 DOM 标记 `data-flow-node`、`data-flow-core`、`data-flow-path`。

- [ ] **Step 1：本地托管构建产物。** 在 `out/` 下执行 `ln -s . web`，再用 `python3 -m http.server` 起服务，访问 `http://localhost:<port>/web/`。
- [ ] **Step 2：跑断言矩阵。**
  - 组合：宽度 {1440, 1280, 390} × 主题 {anthropic, anthropic dark, moonshot} × 语言 {zh, en, ja}。
  - 主题和语言通过 localStorage 预置，或通过首帧脚本读取的 key 注入。
  - 每个组合都要断言：
    - `errors.length === 0`
    - `document.documentElement.scrollWidth <= innerWidth`
    - 卡片和面板内没有元素的 `scrollWidth > clientWidth + 1`（排除 SVG 和跑马灯轨道）
    - `[data-flow-path]` 数量为 14，且每条路径的起点和终点都在对应节点或核心的边沿 ±2px 以内
    - 每个 `ProviderIcon` 都渲染出了 `svg`
- [ ] **Step 3：档位断言。**
  - 用 `--force-prefers-reduced-motion` 启动，或预置 `data-motion-tier="still"`。等 3 秒后断言：
    - 跑马灯轨道的 `getAnimations().length === 0`；
    - 所有 `[data-reveal]` 元素的透明度为 1；
    - `document.getAnimations()` 结束后没有元素残留非 none 的 transform；
    - 步骤区 7 秒后仍停在 01 步。
- [ ] **Step 4：运行设计检测。** 执行 `/Users/centonhuang/Desktop/code/aris-proxy-api/.codebuddy/skills/impeccable/scripts/impeccable detect --json web/src/components/portal web/src/styles/portal.css`，修掉报告中的问题。修完后重跑 Step 2–3。
- [ ] **Step 5：回写契约，并清理构建产物。** 删除 `web/.next` 和 `web/out`。
- [ ] **Step 6：提交**：`git commit -m "docs(agents): 门户 moonshot 星空例外写入动效契约"`
- [ ] **Step 7：请用户在浏览器里验收。** 运行 `npm run dev` 后访问 `http://localhost:3000/web/`，等用户认可观感后，再询问是提 PR 还是直接合并到 master。
