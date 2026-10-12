# 主题动效「双世界」PR1：地基 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭好「双世界」动效的地基：动效档位判定与降级阶梯、注意力信号、环境脉冲总线、帧率调速器，以及承载 atelier（纸与墨）/ lunar（月光与引力）两个 WebGL 场景的 Ambient Stage，替换现有的 ParticleBackground。

**Architecture:** `<html data-motion-tier>` 由首帧前的内联脚本写入，是所有降级行为的唯一事实来源，运行时只降不升。`AmbientStage` 是根布局里的单个 fixed 背景层：`immersive` 档用手写的极简 WebGL2 渲染器（全屏三角形 + 每场景一个 fragment program）绘制当前主题的场景；`lite / still` 档退回 CSS 静态纹理或 Canvas 2D 星空。纯逻辑（档位、调速器、注意力、脉冲）全部是可在 node 下单测的纯函数模块。

**Tech Stack:** Next.js 16（App Router，`output: "export"`）、React 19、TypeScript、Tailwind v4、WebGL2 / GLSL ES 3.00、vitest、chrome-headless-shell（swiftshader，用于 shader 编译与运行时断言）。

**Spec:** `docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md`（本计划只覆盖 §10 的 PR1）

## Global Constraints

- 静态导出不变：`output: "export"`、`basePath: "/web"`，不引入任何需要 SSR / Edge 的依赖。
- PR1 **不新增 npm 依赖**（`motion` 在 PR2 引入）。
- 档位唯一事实来源是 `<html data-motion-tier>`，取值 `immersive | lite | still`；运行时只降不升，恢复需刷新页面。
- 降级条件：`still ← prefers-reduced-motion: reduce`；`lite ← navigator.connection.saveData || navigator.deviceMemory ≤ 4 || navigator.hardwareConcurrency ≤ 4 || (pointer: coarse) || WebGL2 不可用`；其余 `immersive`。
- 渲染分辨率 0.75× DPR（DPR 上限 2）；环境态 30fps 封顶，脉冲与 intro 期间临时 60fps。
- 帧率调速器：滚动 3s 平均 < 45fps → 渲染分辨率降到 0.5× → 再持续 3s 仍 < 45fps → 降到 `lite`。
- 注意力：`keydown / input / wheel / scroll / pointerdown` 视为操作中，presence 在 300ms 内降到 0.35；空闲 8s 后用 2s 回升到 1；`pointermove` 不算操作。
- 颜色严格对齐 token：atelier 底色 = `--background #faf8f5`，墨色取 `--primary #d97757`；lunar 亮面 `#FFEED9`、星云 `#A8BDD8`，月球最高亮度封顶 0.22。
- 可读性：atelier 纸面光照亮度起伏钳制在 ±3%。
- 只动画 `transform / opacity / filter / clip-path / mask`，不给 width / height 加过渡。
- 样式走 `globals.css`；禁止内联 `style` 写定值（canvas 尺寸由 CSS 控制）。
- WebGL 初始化失败 / shader 编译错误 / context lost → `console.warn` 一次（前缀 `[ambient]`）并降到 `lite`。
- 包体积：GL 渲染器 + 两套 shader 的 gzip 增量 ≤ 12KB。
- 每个任务结束前至少通过该任务列出的验证命令；PR 收尾跑 `npm run lint && npm run test && npm run build && npm run format:check`。
- Worktree 内禁止 `npm ci`，软链主工作区 `web/node_modules`。
- 命令中的路径以仓库根为基准：`WT=/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds`。

## 与 spec 的实现层偏差（Task 12 回写 spec）

1. **内联脚本**：spec §4.2 写「共用同一份逻辑的字符串化版本」。生产构建会压缩函数体，`Function.prototype.toString` 不可靠，改为 `resolveMotionTier` 的手写 ES5 镜像，由 `tier.test.ts` 用 96 种信号组合对拍保证一致。
2. **文件拆分**：`tier.ts` 只放纯逻辑；DOM 存储与 React hook 放到 `tier-store.ts`，保证 `tier.ts` 可在 node 下单测。
3. **route 脉冲来源**：spec §7.4 写「导航点击 → emitAmbientPulse」。改为由 `AmbientStage` 监听 `pathname` 变化，在 1s 内最后一次 `pointerdown` 的位置发出（无近期点击则取视口中心）。这样导航、表格链接、按钮跳转全部自动覆盖，PR3 无需再接线。
4. **uniform 增补**：新增 `uDrift`（漂移时钟，操作中速度减半、聚焦时再减速），避免直接缩放 `uTime` 造成画面跳变。
5. **atelier 降级纸纹的挂载点**：spec §5.3 写「`body` 叠一层静态纸纹」。改为挂在 `.ambient-stage` 上：页面底板透明后透出的正是舞台层，纹理和 GL 场景处于同一层级，切换档位时不会出现两层叠加。

## File Structure

| 路径 | 职责 |
|---|---|
| Create `web/src/lib/motion/tier.ts` | `MotionTier` 类型、`resolveMotionTier`、`lowerTier`、`isMotionTier`、`MOTION_TIER_INIT_SCRIPT` |
| Create `web/src/lib/motion/tier.test.ts` | 档位判定、降档、内联脚本对拍 |
| Create `web/src/lib/motion/tier-store.ts` | `useMotionTier()`、`downgradeMotionTier()`、减弱动态运行时监听 |
| Create `web/src/lib/motion/fps-governor.ts` / `.test.ts` | 帧率调速器状态机 |
| Create `web/src/lib/motion/attention.ts` / `.test.ts` | 注意力 reducer + 事件绑定 |
| Create `web/src/lib/motion/pulse.ts` / `.test.ts` | 环境脉冲总线 |
| Create `web/src/components/ambient/scenes/common.ts` | 顶点 shader、fragment 公共前缀（uniform 声明 + 噪声函数）、`AmbientScene` 类型、脉冲类型码 |
| Create `web/src/components/ambient/scenes/atelier.ts` | 纸与墨场景 |
| Create `web/src/components/ambient/scenes/lunar.ts` | 月光与引力场景 |
| Create `web/src/components/ambient/gl.ts` | 极简 WebGL2 渲染器 |
| Create `web/src/components/ambient/fallback-2d.ts` | Canvas 2D 星空（由 ParticleBackground 迁移） |
| Create `web/src/components/ambient/ambient-stage.tsx` | 舞台组件：档位联动、渲染循环、事件接线 |
| Delete `web/src/components/theme/particle-background.tsx` | 被 AmbientStage 取代 |
| Modify `web/src/app/layout.tsx` | 内联脚本追加档位判定；挂载 AmbientStage |
| Modify `web/src/app/globals.css` | `.page-surface` 两主题透明；`.particle-bg` 规则替换为 `.ambient-stage` 及降级纹理 |
| Modify `web/CONTEXT.md`、`docs/agents/web-frontend.md`、spec | 术语与契约同步 |

---

### Task 0: Worktree 准备与包体积基线

**Files:** 无代码改动（worktree `.worktrees/theme-motion-worlds` 已在 brainstorming 阶段创建，分支 `feature/theme-motion-worlds-2026-10-12`）

- [ ] **Step 1: 软链依赖并确认分支**

```bash
WT=/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds
ln -s ../../../web/node_modules "$WT/web/node_modules"
cd "$WT" && git branch --show-current && git status --short
```

Expected: 输出 `feature/theme-motion-worlds-2026-10-12`，工作区干净。

- [ ] **Step 2: 构建并记录 chunks gzip 基线**

```bash
cd "$WT/web" && env -u NODE_OPTIONS npm run build > /tmp/aris-build-baseline.log 2>&1; echo "exit=$?"
for f in $(find out/_next/static/chunks -name '*.js'); do gzip -9c "$f" | wc -c; done | awk '{s+=$1} END {print s}' > /tmp/aris-bundle-baseline.txt
cat /tmp/aris-bundle-baseline.txt
```

Expected: `exit=0`，打印一个字节数（基线）。`env -u NODE_OPTIONS` 用于绕过本机 IDE safe-delete shim 在清理 `.next` 时的批量删除拦截。

---

### Task 1: 档位判定纯逻辑 + 内联脚本

**Files:**
- Create: `web/src/lib/motion/tier.ts`
- Test: `web/src/lib/motion/tier.test.ts`

**Interfaces:**
- Produces:
  - `type MotionTier = "immersive" | "lite" | "still"`
  - `interface MotionSignals { reducedMotion: boolean; saveData: boolean; coarsePointer: boolean; deviceMemory: number | undefined; hardwareConcurrency: number | undefined }`
  - `resolveMotionTier(s: MotionSignals): MotionTier`
  - `lowerTier(current: MotionTier, proposed: MotionTier): MotionTier`
  - `isMotionTier(x: unknown): x is MotionTier`
  - `MOTION_TIER_INIT_SCRIPT: string`

- [ ] **Step 1: 写失败测试**

`web/src/lib/motion/tier.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import {
  MOTION_TIER_INIT_SCRIPT,
  isMotionTier,
  lowerTier,
  resolveMotionTier,
  type MotionSignals,
} from "./tier";

const BASE: MotionSignals = {
  reducedMotion: false,
  saveData: false,
  coarsePointer: false,
  deviceMemory: 8,
  hardwareConcurrency: 8,
};

function runInitScript(s: MotionSignals, opts: { throwOnMatchMedia?: boolean } = {}) {
  const dataset: Record<string, string> = {};
  const doc = { documentElement: { dataset } };
  const nav = {
    connection: { saveData: s.saveData },
    deviceMemory: s.deviceMemory,
    hardwareConcurrency: s.hardwareConcurrency,
  };
  const win = {
    matchMedia: (q: string) => {
      if (opts.throwOnMatchMedia) throw new Error("no matchMedia");
      if (q === "(prefers-reduced-motion: reduce)") return { matches: s.reducedMotion };
      if (q === "(pointer: coarse)") return { matches: s.coarsePointer };
      return { matches: false };
    },
  };
  new Function("document", "navigator", "window", MOTION_TIER_INIT_SCRIPT)(doc, nav, win);
  return dataset.motionTier;
}

describe("resolveMotionTier", () => {
  it("defaults to immersive on a capable desktop", () => {
    expect(resolveMotionTier(BASE)).toBe("immersive");
  });

  it("reduced motion wins over every other signal", () => {
    expect(resolveMotionTier({ ...BASE, reducedMotion: true, saveData: true })).toBe("still");
  });

  it.each([
    ["saveData", { saveData: true }],
    ["coarse pointer", { coarsePointer: true }],
    ["4GB memory", { deviceMemory: 4 }],
    ["4 cores", { hardwareConcurrency: 4 }],
  ])("drops to lite on %s", (_label, patch) => {
    expect(resolveMotionTier({ ...BASE, ...patch })).toBe("lite");
  });

  it("treats unsupported deviceMemory / hardwareConcurrency as capable", () => {
    expect(
      resolveMotionTier({ ...BASE, deviceMemory: undefined, hardwareConcurrency: undefined }),
    ).toBe("immersive");
  });
});

describe("lowerTier", () => {
  it("only ever moves down", () => {
    expect(lowerTier("immersive", "lite")).toBe("lite");
    expect(lowerTier("lite", "immersive")).toBe("lite");
    expect(lowerTier("still", "lite")).toBe("still");
    expect(lowerTier("lite", "still")).toBe("still");
  });
});

describe("isMotionTier", () => {
  it("accepts only the three tier names", () => {
    expect(isMotionTier("immersive")).toBe(true);
    expect(isMotionTier("still")).toBe(true);
    expect(isMotionTier("full")).toBe(false);
    expect(isMotionTier(undefined)).toBe(false);
  });
});

describe("MOTION_TIER_INIT_SCRIPT", () => {
  it("matches resolveMotionTier across the full signal matrix", () => {
    for (const reducedMotion of [false, true])
      for (const saveData of [false, true])
        for (const coarsePointer of [false, true])
          for (const deviceMemory of [undefined, 2, 4, 8])
            for (const hardwareConcurrency of [undefined, 4, 8]) {
              const s = { reducedMotion, saveData, coarsePointer, deviceMemory, hardwareConcurrency };
              expect(runInitScript(s), JSON.stringify(s)).toBe(resolveMotionTier(s));
            }
  });

  it("falls back to lite when detection throws", () => {
    expect(runInitScript(BASE, { throwOnMatchMedia: true })).toBe("lite");
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/tier.test.ts`
Expected: FAIL，报错 `Failed to resolve import "./tier"`。

- [ ] **Step 3: 实现**

`web/src/lib/motion/tier.ts`：

```ts
/* Motion Tier: the single source of truth for every degradation decision
 * (web/CONTEXT.md「Motion」). Pure logic only — the DOM-backed store and the
 * React hook live in tier-store.ts so this module stays testable in node. */

export type MotionTier = "immersive" | "lite" | "still";

export interface MotionSignals {
  reducedMotion: boolean;
  saveData: boolean;
  coarsePointer: boolean;
  /** navigator.deviceMemory in GB; undefined where unsupported (Safari / Firefox). */
  deviceMemory: number | undefined;
  hardwareConcurrency: number | undefined;
}

const LOW_END_MEMORY_GB = 4;
const LOW_END_CORES = 4;

export function resolveMotionTier(s: MotionSignals): MotionTier {
  if (s.reducedMotion) return "still";
  if (s.saveData || s.coarsePointer) return "lite";
  if (s.deviceMemory !== undefined && s.deviceMemory <= LOW_END_MEMORY_GB) return "lite";
  if (s.hardwareConcurrency !== undefined && s.hardwareConcurrency <= LOW_END_CORES) return "lite";
  return "immersive";
}

const TIER_RANK: Record<MotionTier, number> = { still: 0, lite: 1, immersive: 2 };

/** Runtime tier changes only ever go down: returns the lower of the two. */
export function lowerTier(current: MotionTier, proposed: MotionTier): MotionTier {
  return TIER_RANK[proposed] < TIER_RANK[current] ? proposed : current;
}

export function isMotionTier(x: unknown): x is MotionTier {
  return x === "immersive" || x === "lite" || x === "still";
}

/* Inline init script (FOUC prevention, same pattern as THEME_INIT_SCRIPT):
 * runs synchronously before first paint and writes <html data-motion-tier>.
 * A hand-written ES5 mirror of resolveMotionTier — production minification
 * makes Function.prototype.toString unreliable — kept in lockstep by the
 * full-matrix parity test in tier.test.ts. WebGL2 is probed later by the
 * Ambient Stage (too costly to create a context before first paint). */
export const MOTION_TIER_INIT_SCRIPT =
  "(function(){var d=document.documentElement;try{var n=navigator,c=n.connection," +
  "m=function(q){return window.matchMedia(q).matches};" +
  "d.dataset.motionTier=m('(prefers-reduced-motion: reduce)')?'still':" +
  "((c&&c.saveData)||m('(pointer: coarse)')||" +
  "(n.deviceMemory!==undefined&&n.deviceMemory<=4)||" +
  "(n.hardwareConcurrency!==undefined&&n.hardwareConcurrency<=4))?'lite':'immersive';" +
  "}catch(e){d.dataset.motionTier='lite';}})();";
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/tier.test.ts`
Expected: PASS（所有用例通过）。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/lib/motion/tier.ts web/src/lib/motion/tier.test.ts
git add web/src/lib/motion/tier.ts web/src/lib/motion/tier.test.ts
git commit -m "feat(web): 动效档位判定纯逻辑与首帧前内联脚本"
```

---

### Task 2: 帧率调速器

**Files:**
- Create: `web/src/lib/motion/fps-governor.ts`
- Test: `web/src/lib/motion/fps-governor.test.ts`

**Interfaces:**
- Produces:
  - `type GovernorAction = "none" | "reduce-scale" | "downgrade"`
  - `interface GovernorState { windowStart: number; lastFrame: number; frames: number; scaleReduced: boolean; done: boolean }`
  - `createGovernor(now: number): GovernorState`
  - `governorTick(s: GovernorState, now: number): { state: GovernorState; action: GovernorAction }`
  - `resetGovernorWindow(s: GovernorState, now: number): GovernorState`

- [ ] **Step 1: 写失败测试**

`web/src/lib/motion/fps-governor.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import {
  createGovernor,
  governorTick,
  resetGovernorWindow,
  type GovernorAction,
  type GovernorState,
} from "./fps-governor";

function simulate(state: GovernorState, start: number, fps: number, ms: number) {
  const step = 1000 / fps;
  const actions: GovernorAction[] = [];
  let s = state;
  let t = start;
  const frames = Math.floor(ms / step);
  for (let i = 1; i <= frames; i++) {
    t = start + i * step;
    const r = governorTick(s, t);
    s = r.state;
    if (r.action !== "none") actions.push(r.action);
  }
  return { state: s, actions, end: t };
}

describe("fps governor", () => {
  it("leaves a healthy 60fps stage alone", () => {
    expect(simulate(createGovernor(0), 0, 60, 9000).actions).toEqual([]);
  });

  it("first reduces render scale, then downgrades, then stops judging", () => {
    const a = simulate(createGovernor(0), 0, 30, 3100);
    expect(a.actions).toEqual(["reduce-scale"]);
    const b = simulate(a.state, a.end, 30, 3100);
    expect(b.actions).toEqual(["downgrade"]);
    const c = simulate(b.state, b.end, 20, 9000);
    expect(c.actions).toEqual([]);
  });

  it("never recovers the reduced scale when the frame rate improves", () => {
    const a = simulate(createGovernor(0), 0, 30, 3100);
    const b = simulate(a.state, a.end, 60, 9000);
    expect(b.actions).toEqual([]);
    expect(b.state.scaleReduced).toBe(true);
  });

  it("treats a long stall as a fresh window, not as slowness", () => {
    const a = simulate(createGovernor(0), 0, 60, 2000);
    const resumed = governorTick(a.state, a.end + 1500);
    expect(resumed.action).toBe("none");
    const b = simulate(resumed.state, a.end + 1500, 30, 2900);
    expect(b.actions).toEqual([]);
  });

  it("resetGovernorWindow keeps past degradation", () => {
    const a = simulate(createGovernor(0), 0, 30, 3100);
    const s = resetGovernorWindow(a.state, 10_000);
    expect(s.scaleReduced).toBe(true);
    expect(s.frames).toBe(0);
    expect(s.windowStart).toBe(10_000);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/fps-governor.test.ts`
Expected: FAIL，`Failed to resolve import "./fps-governor"`。

- [ ] **Step 3: 实现**

`web/src/lib/motion/fps-governor.ts`：

```ts
/* Frame-rate governor (spec §8.1): watches the browser's rAF cadence —
 * independent of the stage's own 30fps render cap — and asks for
 * degradation, never recovery. */

export type GovernorAction = "none" | "reduce-scale" | "downgrade";

export const GOVERNOR_WINDOW_MS = 3000;
export const GOVERNOR_MIN_FPS = 45;
/** One gap longer than this (tab switch, debugger, GC storm) restarts the
 * window instead of being counted as slowness. */
export const GOVERNOR_STALL_MS = 1000;

export interface GovernorState {
  windowStart: number;
  lastFrame: number;
  frames: number;
  scaleReduced: boolean;
  done: boolean;
}

export function createGovernor(now: number): GovernorState {
  return { windowStart: now, lastFrame: now, frames: 0, scaleReduced: false, done: false };
}

export function resetGovernorWindow(s: GovernorState, now: number): GovernorState {
  return { ...s, windowStart: now, lastFrame: now, frames: 0 };
}

export function governorTick(
  s: GovernorState,
  now: number,
): { state: GovernorState; action: GovernorAction } {
  if (s.done) return { state: s, action: "none" };
  if (now - s.lastFrame > GOVERNOR_STALL_MS) {
    return { state: resetGovernorWindow(s, now), action: "none" };
  }
  const frames = s.frames + 1;
  const elapsed = now - s.windowStart;
  if (elapsed < GOVERNOR_WINDOW_MS) {
    return { state: { ...s, lastFrame: now, frames }, action: "none" };
  }
  const fps = (frames * 1000) / elapsed;
  const next = resetGovernorWindow(s, now);
  if (fps >= GOVERNOR_MIN_FPS) return { state: next, action: "none" };
  if (!s.scaleReduced) return { state: { ...next, scaleReduced: true }, action: "reduce-scale" };
  return { state: { ...next, done: true }, action: "downgrade" };
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/fps-governor.test.ts`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/lib/motion/fps-governor.ts web/src/lib/motion/fps-governor.test.ts
git add web/src/lib/motion/fps-governor.ts web/src/lib/motion/fps-governor.test.ts
git commit -m "feat(web): 环境舞台帧率调速器（只降不升）"
```

---

### Task 3: 注意力信号

**Files:**
- Create: `web/src/lib/motion/attention.ts`
- Test: `web/src/lib/motion/attention.test.ts`

**Interfaces:**
- Produces:
  - 常量 `PRESENCE_BUSY = 0.35`、`BUSY_FALL_MS = 300`、`IDLE_DELAY_MS = 8000`、`IDLE_RISE_MS = 2000`
  - `interface AttentionState { presence: number; lastActiveAt: number }`
  - `createAttention(): AttentionState`
  - `markActive(s: AttentionState, now: number): AttentionState`
  - `stepAttention(s: AttentionState, now: number, dtMs: number): AttentionState`
  - `bindAttention(onActive: () => void): () => void`（返回解绑函数）

- [ ] **Step 1: 写失败测试**

`web/src/lib/motion/attention.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import {
  BUSY_FALL_MS,
  IDLE_DELAY_MS,
  IDLE_RISE_MS,
  PRESENCE_BUSY,
  createAttention,
  markActive,
  stepAttention,
  type AttentionState,
} from "./attention";

const FRAME_MS = 16;

function advance(s: AttentionState, from: number, to: number): AttentionState {
  let state = s;
  for (let t = from + FRAME_MS; t <= to; t += FRAME_MS) state = stepAttention(state, t, FRAME_MS);
  return state;
}

describe("attention", () => {
  it("starts fully present and stays present while idle", () => {
    expect(advance(createAttention(), 0, 20_000).presence).toBe(1);
  });

  it("falls linearly during the busy fall", () => {
    const s = advance(markActive(createAttention(), 0), 0, 144);
    expect(s.presence).toBeCloseTo(1 - (1 - PRESENCE_BUSY) * (144 / BUSY_FALL_MS), 5);
  });

  it("reaches the busy floor within BUSY_FALL_MS of activity", () => {
    const s = advance(markActive(createAttention(), 0), 0, BUSY_FALL_MS + FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });

  it("holds the floor until IDLE_DELAY_MS has passed", () => {
    const s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS - FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });

  it("rises back to 1 over IDLE_RISE_MS after the idle delay", () => {
    const s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS + IDLE_RISE_MS + FRAME_MS);
    expect(s.presence).toBe(1);
  });

  it("new activity during the rise pushes it back down", () => {
    let s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS + 1000);
    expect(s.presence).toBeGreaterThan(PRESENCE_BUSY);
    const at = IDLE_DELAY_MS + 1000;
    s = advance(markActive(s, at), at, at + BUSY_FALL_MS + FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/attention.test.ts`
Expected: FAIL，`Failed to resolve import "./attention"`。

- [ ] **Step 3: 实现**

`web/src/lib/motion/attention.ts`：

```ts
/* Attention signal → Ambient Presence (spec §4.3). Active input pushes the
 * ambient layer back within BUSY_FALL_MS; after IDLE_DELAY_MS of calm it
 * returns over IDLE_RISE_MS. pointermove is deliberately NOT activity: it
 * drives the lamp / parallax and must not dim the scene it animates.
 * PRESENCE_BUSY is mirrored in scenes/common.ts presenceFactor(). */

export const PRESENCE_BUSY = 0.35;
export const BUSY_FALL_MS = 300;
export const IDLE_DELAY_MS = 8000;
export const IDLE_RISE_MS = 2000;

export interface AttentionState {
  presence: number;
  lastActiveAt: number;
}

export function createAttention(): AttentionState {
  return { presence: 1, lastActiveAt: Number.NEGATIVE_INFINITY };
}

export function markActive(s: AttentionState, now: number): AttentionState {
  return { ...s, lastActiveAt: now };
}

export function stepAttention(s: AttentionState, now: number, dtMs: number): AttentionState {
  const span = 1 - PRESENCE_BUSY;
  const busy = now - s.lastActiveAt < IDLE_DELAY_MS;
  const presence = busy
    ? Math.max(PRESENCE_BUSY, s.presence - (span * dtMs) / BUSY_FALL_MS)
    : Math.min(1, s.presence + (span * dtMs) / IDLE_RISE_MS);
  return presence === s.presence ? s : { ...s, presence };
}

const ACTIVITY_EVENTS = ["keydown", "input", "wheel", "scroll", "pointerdown"] as const;

/** Capture-phase listeners so scrolls inside nested containers (dashboard
 * <main>) count too. Returns an unbind function. */
export function bindAttention(onActive: () => void): () => void {
  const opts: AddEventListenerOptions = { capture: true, passive: true };
  for (const type of ACTIVITY_EVENTS) window.addEventListener(type, onActive, opts);
  return () => {
    for (const type of ACTIVITY_EVENTS) window.removeEventListener(type, onActive, opts);
  };
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/attention.test.ts`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/lib/motion/attention.ts web/src/lib/motion/attention.test.ts
git add web/src/lib/motion/attention.ts web/src/lib/motion/attention.test.ts
git commit -m "feat(web): 环境舞台注意力信号（操作时退后、空闲时浮现）"
```

---

### Task 4: 环境脉冲总线

**Files:**
- Create: `web/src/lib/motion/pulse.ts`
- Test: `web/src/lib/motion/pulse.test.ts`

**Interfaces:**
- Produces:
  - `type AmbientPulseKind = "primary" | "route" | "focus-on" | "focus-off" | "intro"`
  - `interface AmbientPulse { kind: AmbientPulseKind; x?: number; y?: number }`（视口 CSS px；省略表示视口中心）
  - `emitAmbientPulse(pulse: AmbientPulse): void`
  - `onAmbientPulse(listener: (pulse: AmbientPulse) => void): () => void`

- [ ] **Step 1: 写失败测试**

`web/src/lib/motion/pulse.test.ts`：

```ts
import { describe, expect, it, vi } from "vitest";
import { emitAmbientPulse, onAmbientPulse } from "./pulse";

describe("ambient pulse bus", () => {
  it("delivers pulses to every subscriber until it unsubscribes", () => {
    const a = vi.fn();
    const b = vi.fn();
    const offA = onAmbientPulse(a);
    const offB = onAmbientPulse(b);

    emitAmbientPulse({ kind: "primary", x: 10, y: 20 });
    expect(a).toHaveBeenCalledWith({ kind: "primary", x: 10, y: 20 });
    expect(b).toHaveBeenCalledTimes(1);

    offA();
    emitAmbientPulse({ kind: "route" });
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenLastCalledWith({ kind: "route" });
    offB();
  });

  it("is a no-op without subscribers", () => {
    expect(() => emitAmbientPulse({ kind: "intro" })).not.toThrow();
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/pulse.test.ts`
Expected: FAIL，`Failed to resolve import "./pulse"`。

- [ ] **Step 3: 实现**

`web/src/lib/motion/pulse.ts`：

```ts
/* Ambient Pulse bus (spec §4.3): UI events request one-shot effects from the
 * Ambient Stage without a React context. Coordinates are viewport CSS px;
 * omitted coordinates mean "viewport center". Each scene maps kinds to its
 * own effect (atelier: ink drops; lunar: gravity ripple / warp). */

export type AmbientPulseKind = "primary" | "route" | "focus-on" | "focus-off" | "intro";

export interface AmbientPulse {
  kind: AmbientPulseKind;
  x?: number;
  y?: number;
}

type Listener = (pulse: AmbientPulse) => void;

const listeners = new Set<Listener>();

export function emitAmbientPulse(pulse: AmbientPulse): void {
  for (const listener of listeners) listener(pulse);
}

export function onAmbientPulse(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd "$WT/web" && npx vitest run src/lib/motion/pulse.test.ts`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/lib/motion/pulse.ts web/src/lib/motion/pulse.test.ts
git add web/src/lib/motion/pulse.ts web/src/lib/motion/pulse.test.ts
git commit -m "feat(web): 环境脉冲总线"
```

---

### Task 5: 档位运行时存储 + 首帧前写入

**Files:**
- Create: `web/src/lib/motion/tier-store.ts`
- Modify: `web/src/app/layout.tsx`（内联脚本那一行与 import）

**Interfaces:**
- Consumes: `isMotionTier`、`lowerTier`、`MotionTier`、`MOTION_TIER_INIT_SCRIPT`（Task 1）
- Produces:
  - `useMotionTier(): MotionTier`（SSR 快照为 `"lite"`）
  - `downgradeMotionTier(proposed: MotionTier): void`（只降不升；同步写 `<html data-motion-tier>` 并通知订阅者）

- [ ] **Step 1: 实现存储与 hook**

`web/src/lib/motion/tier-store.ts`：

```ts
import { useSyncExternalStore } from "react";
import { isMotionTier, lowerTier, type MotionTier } from "./tier";

/* DOM-backed Motion Tier store. <html data-motion-tier> (written before first
 * paint by MOTION_TIER_INIT_SCRIPT) is the source of truth: CSS reads the
 * attribute directly, React reads it through useMotionTier(). */

const SERVER_TIER: MotionTier = "lite";
const listeners = new Set<() => void>();
let reducedMotionBound = false;

function readTier(): MotionTier {
  const v = document.documentElement.dataset.motionTier;
  return isMotionTier(v) ? v : SERVER_TIER;
}

/** Lower the tier (never raises — recovery needs a reload). */
export function downgradeMotionTier(proposed: MotionTier): void {
  const root = document.documentElement;
  const current = readTier();
  const next = lowerTier(current, proposed);
  if (root.dataset.motionTier === next) return;
  root.dataset.motionTier = next;
  for (const listener of listeners) listener();
}

function bindReducedMotion(): void {
  if (reducedMotionBound) return;
  reducedMotionBound = true;
  window.matchMedia("(prefers-reduced-motion: reduce)").addEventListener("change", (e) => {
    if (e.matches) downgradeMotionTier("still");
  });
}

function subscribe(listener: () => void): () => void {
  bindReducedMotion();
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useMotionTier(): MotionTier {
  return useSyncExternalStore(subscribe, readTier, () => SERVER_TIER);
}
```

- [ ] **Step 2: 内联脚本追加档位判定**

`web/src/app/layout.tsx`：

```tsx
// import 区（第 14 行附近）新增一行
import { MOTION_TIER_INIT_SCRIPT } from "@/lib/motion/tier";
```

```tsx
// 原：<script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
<script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT + MOTION_TIER_INIT_SCRIPT }} />
```

并在该 `<script>` 上方的注释末尾补一句：`The same script also writes [data-motion-tier] (lib/motion/tier.ts).`

- [ ] **Step 3: 验证类型、lint 与产物**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npx eslint src/lib/motion src/app/layout.tsx
env -u NODE_OPTIONS npm run build > /tmp/aris-build.log 2>&1; echo "exit=$?"
grep -c "motionTier" out/login/index.html
```

Expected: tsc / eslint 无输出；`exit=0`；grep 输出 `1`（内联脚本已进入静态 HTML）。

- [ ] **Step 4: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/lib/motion/tier-store.ts web/src/app/layout.tsx
git add web/src/lib/motion/tier-store.ts web/src/app/layout.tsx
git commit -m "feat(web): 动效档位运行时存储与首帧前写入 data-motion-tier"
```

---

### Task 6: 极简 WebGL2 渲染器 + shader 公共前缀

**Files:**
- Create: `web/src/components/ambient/scenes/common.ts`
- Create: `web/src/components/ambient/gl.ts`
- 临时验证脚手架（不提交）：`/tmp/aris-shader-check/entry.ts`、`/tmp/aris-shader-check/run.sh`

**约束：** `components/ambient/` 下的模块只用同目录相对导入（`./`），不用 `@/` 别名——验证脚手架用 esbuild 直接打包，不解析别名。

**Interfaces:**
- Produces（`scenes/common.ts`）：
  - `type SceneId = "atelier" | "lunar"`
  - `interface AmbientScene { id: SceneId; fragment: string; introMs: number; pointerTauMs: number }`
  - `PULSE_SLOTS = 4`、`PULSE_KIND_CODE = { primary: 1, route: 2 } as const`
  - `VERTEX_SOURCE: string`、`FRAGMENT_PRELUDE: string`
  - GLSL 公共函数：`hash12`、`hash22`、`vnoise`、`fbm`、`presenceN()`、`ditherNoise(vec2)`
  - GLSL uniform：`uResolution`、`uTime`、`uDrift`、`uPointer`、`uScroll`、`uPresence`、`uFocus`、`uIntro`、`uPulse[4]`
- Produces（`gl.ts`）：
  - `interface SceneProgram { set1f(name: string, v: number): void; set2f(name: string, x: number, y: number): void; set4fv(name: string, v: Float32Array): void }`
  - `interface Renderer { use(id: SceneId, fragment: string): SceneProgram; resize(width: number, height: number): void; draw(): void; dispose(): void }`
  - `createRenderer(canvas: HTMLCanvasElement): Renderer | null`（无 WebGL2 返回 `null`；编译 / 链接失败时 `use()` 抛错）

- [ ] **Step 1: 写验证脚手架（会先失败）**

`/tmp/aris-shader-check/run.sh`：

```bash
#!/bin/bash
set -euo pipefail
WT=/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds
H=/tmp/aris-shader-check
CH=~/Library/Caches/ms-playwright/chromium_headless_shell-1228/chrome-headless-shell-mac-arm64/chrome-headless-shell
"$WT/web/node_modules/.bin/esbuild" "$H/entry.ts" --bundle --format=iife --outfile="$H/bundle.js" --log-level=warning
printf '<!doctype html><html><body><pre id="__m__">pending</pre><script src="bundle.js"></script></body></html>' > "$H/index.html"
"$CH" --headless --use-angle=swiftshader --enable-unsafe-swiftshader --user-data-dir="$(mktemp -d)" \
  --virtual-time-budget=3000 --dump-dom "file://$H/index.html" 2>/dev/null | grep -o '<pre id="__m__">[^<]*'
```

`/tmp/aris-shader-check/entry.ts`：

```ts
import { createRenderer } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/gl";
import {
  FRAGMENT_PRELUDE,
  PULSE_SLOTS,
} from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/common";

const out: Record<string, unknown> = {};
try {
  const canvas = document.createElement("canvas");
  const r = createRenderer(canvas);
  if (!r) throw new Error("no webgl2");
  r.resize(64, 64);
  const p = r.use(
    "atelier",
    FRAGMENT_PRELUDE +
      "void main(){ float n = fbm(gl_FragCoord.xy * 0.05) * presenceN() + hash22(gl_FragCoord.xy).x * 0.1;" +
      " outColor = vec4(vec3(n) + ditherNoise(gl_FragCoord.xy), 1.0); }",
  );
  p.set2f("uResolution", 64, 64);
  p.set1f("uPresence", 1);
  p.set4fv("uPulse", new Float32Array(PULSE_SLOTS * 4));
  r.draw();
  const gl = canvas.getContext("webgl2");
  const px = new Uint8Array(64 * 64 * 4);
  gl?.readPixels(0, 0, 64, 64, gl.RGBA, gl.UNSIGNED_BYTE, px);
  let sum = 0;
  for (let i = 0; i < px.length; i += 4) sum += px[i];
  out.probeMean = Math.round(sum / (64 * 64));
  out.pass = (out.probeMean as number) > 10;
  r.dispose();
} catch (e) {
  out.pass = false;
  out.error = String(e);
}
document.getElementById("__m__")!.textContent = JSON.stringify(out);
```

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: esbuild 报错 `Could not resolve ".../components/ambient/gl"`（模块还不存在）。

- [ ] **Step 2: 实现公共前缀**

`web/src/components/ambient/scenes/common.ts`：

```ts
/* Shared GLSL for Ambient Stage scenes (spec §4.3). Every scene fragment is
 * FRAGMENT_PRELUDE + its own body; the stage sets the uniforms below each
 * rendered frame. Keep this module free of `@/` imports (see gl.ts). */

export type SceneId = "atelier" | "lunar";

export interface AmbientScene {
  id: SceneId;
  /** Full fragment source: FRAGMENT_PRELUDE + scene body. */
  fragment: string;
  /** Intro duration (atelier lamp-on / lunar moonrise), ms. */
  introMs: number;
  /** Pointer smoothing time constant, ms (atelier: lazy lamp; lunar: snappy lens). */
  pointerTauMs: number;
}

export const PULSE_SLOTS = 4;

/** uPulse[i].w codes; 0 marks an empty slot. */
export const PULSE_KIND_CODE = { primary: 1, route: 2 } as const;

/** Fullscreen triangle from gl_VertexID — no vertex buffers. */
export const VERTEX_SOURCE = `#version 300 es
void main() {
  vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}
`;

export const FRAGMENT_PRELUDE = `#version 300 es
precision highp float;

uniform vec2 uResolution;           // drawing-buffer px
uniform float uTime;                // s since stage start
uniform float uDrift;               // s, presence/focus-scaled drift clock
uniform vec2 uPointer;              // smoothed pointer, -1..1, y up
uniform float uScroll;              // scrollTop / viewport height
uniform float uPresence;            // 0.35 (busy) .. 1 (idle)
uniform float uFocus;               // 0..1, dialog focus
uniform float uIntro;               // 0..1, eased intro progress
uniform vec4 uPulse[${PULSE_SLOTS}]; // xy uv (y up), z start time (s), w kind (1 primary, 2 route)

out vec4 outColor;

float hash12(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * 0.1031);
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.x + p3.y) * p3.z);
}

vec2 hash22(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * vec3(0.1031, 0.1030, 0.0973));
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.xx + p3.yz) * p3.zy);
}

float vnoise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash12(i), hash12(i + vec2(1.0, 0.0)), u.x),
             mix(hash12(i + vec2(0.0, 1.0)), hash12(i + vec2(1.0, 1.0)), u.x), u.y);
}

float fbm(vec2 p) {
  float v = 0.0;
  float a = 0.5;
  mat2 r = mat2(0.8, 0.6, -0.6, 0.8);
  for (int i = 0; i < 5; i++) {
    v += a * vnoise(p);
    p = r * p * 2.02;
    a *= 0.5;
  }
  return v;
}

// 0.35 mirrors PRESENCE_BUSY in lib/motion/attention.ts.
float presenceN() {
  return clamp((uPresence - 0.35) / 0.65, 0.0, 1.0);
}

float ditherNoise(vec2 fc) {
  return (hash12(fc) - 0.5) / 255.0;
}
`;
```

- [ ] **Step 3: 实现渲染器**

`web/src/components/ambient/gl.ts`：

```ts
import { VERTEX_SOURCE, type SceneId } from "./scenes/common";

/* Minimal WebGL2 renderer for the Ambient Stage: one fullscreen triangle,
 * one cached program per scene. Relative imports only — the shader check
 * harness bundles this module with esbuild, without the `@/` alias. */

export interface SceneProgram {
  set1f(name: string, v: number): void;
  set2f(name: string, x: number, y: number): void;
  set4fv(name: string, v: Float32Array): void;
}

export interface Renderer {
  /** Compile (once, cached) and bind a scene program. Throws on compile/link errors. */
  use(id: SceneId, fragment: string): SceneProgram;
  resize(width: number, height: number): void;
  draw(): void;
  dispose(): void;
}

function compile(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error("createShader failed");
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(shader) ?? "";
    gl.deleteShader(shader);
    throw new Error(`shader compile failed: ${log}`);
  }
  return shader;
}

function link(gl: WebGL2RenderingContext, fragment: string): WebGLProgram {
  const vs = compile(gl, gl.VERTEX_SHADER, VERTEX_SOURCE);
  const fs = compile(gl, gl.FRAGMENT_SHADER, fragment);
  const program = gl.createProgram();
  if (!program) throw new Error("createProgram failed");
  gl.attachShader(program, vs);
  gl.attachShader(program, fs);
  gl.linkProgram(program);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(program) ?? "";
    gl.deleteProgram(program);
    throw new Error(`program link failed: ${log}`);
  }
  return program;
}

function programApi(gl: WebGL2RenderingContext, program: WebGLProgram): SceneProgram {
  const locations = new Map<string, WebGLUniformLocation | null>();
  const loc = (name: string) => {
    if (!locations.has(name)) locations.set(name, gl.getUniformLocation(program, name));
    return locations.get(name) ?? null;
  };
  return {
    set1f: (name, v) => gl.uniform1f(loc(name), v),
    set2f: (name, x, y) => gl.uniform2f(loc(name), x, y),
    set4fv: (name, v) => gl.uniform4fv(loc(name), v),
  };
}

export function createRenderer(canvas: HTMLCanvasElement): Renderer | null {
  const gl = canvas.getContext("webgl2", {
    alpha: false,
    antialias: false,
    depth: false,
    stencil: false,
    powerPreference: "low-power",
    preserveDrawingBuffer: false,
  });
  if (!gl) return null;

  const vao = gl.createVertexArray();
  gl.bindVertexArray(vao);
  const programs = new Map<SceneId, { program: WebGLProgram; api: SceneProgram }>();

  return {
    use(id, fragment) {
      let entry = programs.get(id);
      if (!entry) {
        const program = link(gl, fragment);
        entry = { program, api: programApi(gl, program) };
        programs.set(id, entry);
      }
      gl.useProgram(entry.program);
      return entry.api;
    },
    resize(width, height) {
      canvas.width = width;
      canvas.height = height;
      gl.viewport(0, 0, width, height);
    },
    draw() {
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    },
    dispose() {
      for (const { program } of programs.values()) gl.deleteProgram(program);
      programs.clear();
      gl.deleteVertexArray(vao);
      // Fires `webglcontextlost`: callers must detach their listener first.
      gl.getExtension("WEBGL_lose_context")?.loseContext();
    },
  };
}
```

- [ ] **Step 4: 运行脚手架确认通过**

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: `<pre id="__m__">{"probeMean":<大于 10 的整数>,"pass":true}`。

- [ ] **Step 5: lint 并提交**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npx eslint src/components/ambient
cd "$WT" && npx --prefix web prettier --write web/src/components/ambient
git add web/src/components/ambient/gl.ts web/src/components/ambient/scenes/common.ts
git commit -m "feat(web): 环境舞台极简 WebGL2 渲染器与 shader 公共前缀"
```

---

### Task 7: atelier 场景（纸与墨）

**Files:**
- Create: `web/src/components/ambient/scenes/atelier.ts`
- 临时：重写 `/tmp/aris-shader-check/entry.ts`

**Interfaces:**
- Consumes: `FRAGMENT_PRELUDE`、`AmbientScene`、`PULSE_SLOTS`（Task 6）；`createRenderer`（Task 6）
- Produces: `atelierScene: AmbientScene`（`id: "atelier"`、`introMs: 800`、`pointerTauMs: 1200`）

- [ ] **Step 1: 写场景断言（会先失败）**

`/tmp/aris-shader-check/entry.ts` 整体替换为：

```ts
import { createRenderer } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/gl";
import {
  PULSE_SLOTS,
  type AmbientScene,
} from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/common";
import { atelierScene } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/atelier";

const W = 128;
const H = 72;

function render(scene: AmbientScene, intro: number): Uint8Array {
  const canvas = document.createElement("canvas");
  const r = createRenderer(canvas);
  if (!r) throw new Error("no webgl2");
  r.resize(W, H);
  const p = r.use(scene.id, scene.fragment);
  const pulses = new Float32Array(PULSE_SLOTS * 4);
  for (let i = 0; i < PULSE_SLOTS; i++) pulses[i * 4 + 2] = -1000;
  p.set2f("uResolution", W, H);
  p.set1f("uTime", 10);
  p.set1f("uDrift", 10);
  p.set2f("uPointer", 0, 0);
  p.set1f("uScroll", 0);
  p.set1f("uPresence", 1);
  p.set1f("uFocus", 0);
  p.set1f("uIntro", intro);
  p.set4fv("uPulse", pulses);
  r.draw();
  const gl = canvas.getContext("webgl2");
  const px = new Uint8Array(W * H * 4);
  gl?.readPixels(0, 0, W, H, gl.RGBA, gl.UNSIGNED_BYTE, px);
  r.dispose();
  return px;
}

/** Mean luminance in a uv box (readPixels origin is bottom-left, y up). */
function boxMean(px: Uint8Array, x0: number, x1: number, y0: number, y1: number): number {
  let sum = 0;
  let n = 0;
  for (let y = Math.floor(y0 * H); y < Math.floor(y1 * H); y++)
    for (let x = Math.floor(x0 * W); x < Math.floor(x1 * W); x++) {
      const o = (y * W + x) * 4;
      sum += (px[o] + px[o + 1] + px[o + 2]) / 3;
      n++;
    }
  return sum / n;
}

function minChannel(px: Uint8Array): number {
  let m = 255;
  for (let i = 0; i < px.length; i += 4) m = Math.min(m, px[i], px[i + 1], px[i + 2]);
  return m;
}

const out: Record<string, unknown> = {};
try {
  // Paper #faf8f5 → mean luminance 247.7; ±3% lighting clamp → [240.2, 255].
  const lit = render(atelierScene, 1);
  const center = boxMean(lit, 0.45, 0.55, 0.45, 0.55);
  const min = minChannel(lit); // worst corner: blue 245 × 0.97 (light) × 0.94 (vignette) ≈ 223
  out.atelier = { center: Math.round(center), min };
  out.pass = center >= 240 && center <= 255 && min >= 220;
} catch (e) {
  out.pass = false;
  out.error = String(e);
}
document.getElementById("__m__")!.textContent = JSON.stringify(out);
```

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: esbuild 报错 `Could not resolve ".../scenes/atelier"`。

- [ ] **Step 2: 实现场景**

`web/src/components/ambient/scenes/atelier.ts`：

```ts
import { FRAGMENT_PRELUDE, type AmbientScene } from "./common";

/* 纸与墨 · Atelier (spec §5.3): fibrous paper lit by a raking desk lamp that
 * lazily follows the pointer, ink drops diffusing where the user acts.
 * Lighting is clamped to ±3% so text on bare paper keeps its contrast. */

const ATELIER_BODY = `
const vec3 PAPER = vec3(0.9804, 0.9725, 0.9608);  // --background #faf8f5
const vec3 INK = vec3(0.8510, 0.4667, 0.3412);    // --primary #d97757
const vec3 SEPIA = vec3(0.4275, 0.3333, 0.2549);
const float LIGHT_RANGE = 0.03;

float paperHeight(vec2 p) {
  float fibers = fbm(p * vec2(0.9, 5.0));          // stretched along x
  float grain = vnoise(p * 9.0);
  return fibers * 0.75 + grain * 0.25;
}

// One ink drop: diffuses for 1.4s, dries to a faint stain, gone by 7.4s.
float inkDrop(vec2 s, vec4 pulse, float idx, float aspect) {
  float age = uTime - pulse.z;
  if (pulse.w < 0.5 || age < 0.0 || age > 7.4) return 0.0;
  vec2 c = vec2(pulse.x * aspect, pulse.y);
  float size = pulse.w > 1.5 ? 0.06 : 0.09;        // route drops are smaller
  float r = size * (1.0 - exp(-age * 2.2));
  float d = length(s - c) + (fbm((s - c) * 14.0 + idx * 7.3) - 0.5) * 0.025;
  float wet = 1.0 - smoothstep(r * 0.6, r, d);
  float rim = smoothstep(r * 0.55, r * 0.95, d) * (1.0 - smoothstep(r * 0.95, r * 1.05, d));
  float peak = 0.06 * smoothstep(0.0, 0.2, age)
             * mix(1.0, 0.25, smoothstep(1.4, 2.2, age))
             * (1.0 - smoothstep(2.2, 7.4, age));
  return clamp(wet * 0.7 + rim * 0.6, 0.0, 1.0) * peak;
}

void main() {
  vec2 fc = gl_FragCoord.xy;
  vec2 uv = fc / uResolution;
  float aspect = uResolution.x / uResolution.y;
  vec2 s = vec2(uv.x * aspect, uv.y);              // screen-height units
  vec2 p = s * 6.0;

  // Height → normal via screen-space derivatives (one fbm per pixel).
  float h = paperHeight(p);
  vec2 g = vec2(dFdx(h), dFdy(h)) * (uResolution.y / 6.0);
  vec3 n = normalize(vec3(-g * 0.08, 1.0));

  float sway = sin(uDrift * 0.21) * 0.035;          // ~±2° idle breathing
  vec3 L = normalize(vec3(uPointer * 0.8 + vec2(-0.4 + sway, 0.5), 0.6));
  float shade = (dot(n, L) - L.z) * 0.15;
  vec2 lampUV = uPointer * 0.5 + 0.5;
  float pool = (1.0 - smoothstep(0.0, 1.2, distance(uv, lampUV))) * 0.015;
  float light = clamp((shade + pool) * uIntro, -LIGHT_RANGE, LIGHT_RANGE);
  vec3 col = PAPER * (1.0 + light);

  float vig = smoothstep(0.6, 1.4, length((uv - 0.5) * 2.0)) * 0.06;
  col *= 1.0 - vig * vec3(0.85, 0.95, 1.0);         // warm: blue darkens most

  float ink = 0.0;
  for (int i = 0; i < 4; i++) ink = max(ink, inkDrop(s, uPulse[i], float(i), aspect));
  ink *= mix(0.5, 1.0, presenceN());
  col = mix(col, mix(INK, SEPIA, 0.5), ink);

  outColor = vec4(col + ditherNoise(fc), 1.0);
}
`;

export const atelierScene: AmbientScene = {
  id: "atelier",
  fragment: FRAGMENT_PRELUDE + ATELIER_BODY,
  introMs: 800,
  pointerTauMs: 1200,
};
```

- [ ] **Step 3: 运行脚手架确认通过**

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: `{"atelier":{"center":<240..255>,"min":<≥220>},"pass":true}`。若 `pass:false` 且有 `error`，按 shader 编译日志修正后重跑。

- [ ] **Step 4: lint 并提交**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npx eslint src/components/ambient
cd "$WT" && npx --prefix web prettier --write web/src/components/ambient/scenes/atelier.ts
git add web/src/components/ambient/scenes/atelier.ts
git commit -m "feat(web): atelier 纸与墨环境场景（纸纤维 + 台灯斜光 + 墨滴）"
```

---

### Task 8: lunar 场景（月光与引力）

**Files:**
- Create: `web/src/components/ambient/scenes/lunar.ts`
- 临时：重写 `/tmp/aris-shader-check/entry.ts`

**Interfaces:**
- Consumes: 同 Task 7
- Produces: `lunarScene: AmbientScene`（`id: "lunar"`、`introMs: 1200`、`pointerTauMs: 150`）

- [ ] **Step 1: 写场景断言（会先失败）**

`/tmp/aris-shader-check/entry.ts` 整体替换为（`render` / `boxMean` / `minChannel` 与 Task 7 相同，此处完整重复）：

```ts
import { createRenderer } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/gl";
import {
  PULSE_SLOTS,
  type AmbientScene,
} from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/common";
import { atelierScene } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/atelier";
import { lunarScene } from "/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/theme-motion-worlds/web/src/components/ambient/scenes/lunar";

const W = 128;
const H = 72;

function render(scene: AmbientScene, intro: number): Uint8Array {
  const canvas = document.createElement("canvas");
  const r = createRenderer(canvas);
  if (!r) throw new Error("no webgl2");
  r.resize(W, H);
  const p = r.use(scene.id, scene.fragment);
  const pulses = new Float32Array(PULSE_SLOTS * 4);
  for (let i = 0; i < PULSE_SLOTS; i++) pulses[i * 4 + 2] = -1000;
  p.set2f("uResolution", W, H);
  p.set1f("uTime", 10);
  p.set1f("uDrift", 10);
  p.set2f("uPointer", 0, 0);
  p.set1f("uScroll", 0);
  p.set1f("uPresence", 1);
  p.set1f("uFocus", 0);
  p.set1f("uIntro", intro);
  p.set4fv("uPulse", pulses);
  r.draw();
  const gl = canvas.getContext("webgl2");
  const px = new Uint8Array(W * H * 4);
  gl?.readPixels(0, 0, W, H, gl.RGBA, gl.UNSIGNED_BYTE, px);
  r.dispose();
  return px;
}

function boxMean(px: Uint8Array, x0: number, x1: number, y0: number, y1: number): number {
  let sum = 0;
  let n = 0;
  for (let y = Math.floor(y0 * H); y < Math.floor(y1 * H); y++)
    for (let x = Math.floor(x0 * W); x < Math.floor(x1 * W); x++) {
      const o = (y * W + x) * 4;
      sum += (px[o] + px[o + 1] + px[o + 2]) / 3;
      n++;
    }
  return sum / n;
}

function minChannel(px: Uint8Array): number {
  let m = 255;
  for (let i = 0; i < px.length; i += 4) m = Math.min(m, px[i], px[i + 1], px[i + 2]);
  return m;
}

const out: Record<string, unknown> = {};
try {
  const lit = render(atelierScene, 1);
  const center = boxMean(lit, 0.45, 0.55, 0.45, 0.55);
  const min = minChannel(lit);
  out.atelier = { center: Math.round(center), min };
  const atelierOk = center >= 240 && center <= 255 && min >= 220;

  // Moon sits bottom-right; 0.22 brightness cap → mean ≤ 60. Before the
  // intro it is below the viewport, so the same box must be darker.
  const risen = render(lunarScene, 1);
  const sunk = render(lunarScene, 0);
  const moonRisen = boxMean(risen, 0.8, 0.95, 0.1, 0.3);
  const moonSunk = boxMean(sunk, 0.8, 0.95, 0.1, 0.3);
  const titleArea = boxMean(risen, 0.0, 0.25, 0.8, 1.0); // top-left stays near-black
  out.lunar = {
    moonRisen: Math.round(moonRisen),
    moonSunk: Math.round(moonSunk),
    titleArea: Math.round(titleArea),
  };
  const lunarOk = moonRisen > 2 && moonRisen <= 60 && moonSunk < moonRisen && titleArea < 12;

  out.pass = atelierOk && lunarOk;
} catch (e) {
  out.pass = false;
  out.error = String(e);
}
document.getElementById("__m__")!.textContent = JSON.stringify(out);
```

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: esbuild 报错 `Could not resolve ".../scenes/lunar"`。

- [ ] **Step 2: 实现场景**

`web/src/components/ambient/scenes/lunar.ts`：

```ts
import { FRAGMENT_PRELUDE, type AmbientScene } from "./common";

/* 月光与引力 · Lunar Orbit (spec §6.3): a huge cropped moon in the
 * bottom-right corner (analytic sphere, crater normals, drifting sun),
 * three parallax star layers with a gravitational lens at the pointer,
 * faint nebula. Route pulses warp the stars, primary pulses ripple them. */

const LUNAR_BODY = `
const vec3 MOONLIGHT = vec3(1.0, 0.9333, 0.8510);  // #FFEED9
const vec3 ICE = vec3(0.6588, 0.7412, 0.8471);     // #A8BDD8
const float MOON_RADIUS = 0.9;                     // screen heights
const vec2 MOON_OFFSET = vec2(-0.3, 0.05);         // from the right edge / bottom
const float MOON_MAX = 0.22;                       // brightness cap

vec2 craterField(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 acc = vec2(0.0);
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      vec2 g = vec2(float(x), float(y));
      vec2 o = hash22(i + g);
      float rad = 0.25 + 0.3 * hash12(i + g + 17.0);
      vec2 v = g + o - f;                          // pixel → crater center
      float len = length(v);
      float t = len / rad;
      if (t < 1.2) {
        vec2 dir = v / max(len, 1e-4);
        float bowl = smoothstep(0.0, 0.85, t) * (1.0 - smoothstep(0.85, 1.0, t));
        float rimOut = smoothstep(0.85, 1.0, t) * (1.0 - smoothstep(1.0, 1.2, t));
        acc += dir * (bowl * 0.8 - rimOut * 0.6);
      }
    }
  }
  return acc;
}

vec3 moon(vec2 s, float aspect, out float cover) {
  float rise = 1.0 - uIntro;                       // below the viewport before the intro
  float lift = min(uScroll * 0.08, 0.15);
  vec2 c = vec2(aspect + MOON_OFFSET.x, MOON_OFFSET.y - rise + lift);
  vec2 d = (s - c) / MOON_RADIUS;
  float r2 = dot(d, d);
  float px = 1.0 / (uResolution.y * MOON_RADIUS);
  cover = 1.0 - smoothstep(1.0 - 3.0 * px, 1.0, r2);
  if (cover <= 0.0) return vec3(0.0);

  vec3 nrm = vec3(d, sqrt(max(1.0 - r2, 0.0)));
  vec2 tc = nrm.xy / (1.0 + nrm.z) * 2.5;
  float maria = smoothstep(0.45, 0.75, fbm(tc * 1.3 + 3.1));
  vec2 cr = craterField(tc * 6.0) * 0.35 + craterField(tc * 14.0) * 0.15;
  vec3 nn = normalize(nrm + vec3(cr, 0.0));

  float a = 0.9 + sin(uDrift * 6.2832 / 90.0) * 0.5 + uPointer.x * 0.15;
  vec3 sun = normalize(vec3(cos(a), sin(a) * 0.6 + 0.3, 0.55));
  float albedo = mix(0.92, 0.55, maria) * (0.9 + 0.2 * vnoise(tc * 30.0));
  float term = smoothstep(-0.05, 0.25, dot(nrm, sun));
  float diff = max(dot(nn, sun), 0.0);
  float earthshine = 0.025 * (1.0 - term);
  float rim = pow(1.0 - nrm.z, 3.0) * 0.25 * term;
  float lum = min(albedo * diff * term + earthshine + rim, 1.0);

  float pres = presenceN();
  float breathe = 1.0 + 0.04 * sin(uDrift * 0.5) * pres;
  float dim = mix(0.7, 1.0, pres) * (1.0 - 0.4 * uFocus) * breathe;
  vec3 tint = mix(vec3(0.86, 0.86, 0.88), MOONLIGHT, term);
  return tint * lum * MOON_MAX * dim;
}

// Gravitational lens: sample toward the pointer so stars appear pushed out.
vec2 lens(vec2 s, float aspect) {
  vec2 ptr = vec2((uPointer.x * 0.5 + 0.5) * aspect, uPointer.y * 0.5 + 0.5);
  vec2 dv = s - ptr;
  float d = length(dv);
  float R = 0.15;
  float k = 0.012 * R / (d + R) * (1.0 - smoothstep(R, 2.0 * R, d)) * presenceN();
  return s - dv / max(d, 1e-4) * k;
}

// Primary pulses: an expanding ring that displaces stars and glows faintly.
vec2 ripple(vec2 s, float aspect, out float glow) {
  glow = 0.0;
  vec2 off = vec2(0.0);
  for (int i = 0; i < 4; i++) {
    vec4 pu = uPulse[i];
    float age = uTime - pu.z;
    if (pu.w < 0.5 || pu.w > 1.5 || age < 0.0 || age > 0.6) continue;
    vec2 dv = s - vec2(pu.x * aspect, pu.y);
    float d = length(dv);
    float x = (d - age * 0.5) / 0.02;
    float band = exp(-x * x) * (1.0 - age / 0.6);
    off += dv / max(d, 1e-4) * band * 0.006;
    glow += band * 0.03;
  }
  return off;
}

// Route pulses: stars stretch radially for 450ms, then relax.
float warpAmount() {
  float w = 0.0;
  for (int i = 0; i < 4; i++) {
    vec4 pu = uPulse[i];
    float age = uTime - pu.z;
    if (pu.w > 1.5 && age >= 0.0 && age <= 0.45) w = max(w, sin(3.14159 * age / 0.45));
  }
  return w;
}

float starLayer(vec2 s, float scale, float depth, float seed, float chance, vec2 center, float warp) {
  vec2 par = uPointer * depth * 0.012 + vec2(0.0, uScroll * depth * 0.05);
  vec2 p = (s + par) * scale;
  vec2 i = floor(p);
  vec2 f = fract(p);
  float h = hash12(i + seed);
  if (h > chance) return 0.0;
  vec2 pos = hash22(i + seed * 1.7) * 0.8 + 0.1;
  vec2 dv = (f - pos) * (uResolution.y / scale);   // buffer px
  vec2 radial = normalize(s - center + 1e-5);
  float dr = dot(dv, radial);
  float dt = dot(dv, vec2(-radial.y, radial.x));
  float d2 = dr * dr / (1.0 + warp * 40.0) + dt * dt;
  float rad = (0.6 + 1.2 * hash12(i + seed + 3.3)) * uResolution.y / 1200.0 + 0.4;
  float tw = 0.55 + 0.45 * sin(uDrift * (0.6 + h * 4.0) + h * 40.0);
  return exp(-d2 / (rad * rad)) * tw;
}

vec3 nebula(vec2 s) {
  float a = smoothstep(0.45, 0.85, fbm(s * 1.6 + vec2(uDrift * 0.004, uDrift * 0.002)));
  float b = smoothstep(0.55, 0.9, fbm(s * 2.3 + 7.0 - uDrift * 0.003));
  return ICE * a * 0.04 + MOONLIGHT * b * 0.03;
}

void main() {
  vec2 fc = gl_FragCoord.xy;
  vec2 uv = fc / uResolution;
  float aspect = uResolution.x / uResolution.y;
  vec2 s = vec2(uv.x * aspect, uv.y);
  float pres = presenceN();

  float rippleGlow;
  vec2 ss = lens(s, aspect) - ripple(s, aspect, rippleGlow);
  float warp = warpAmount();
  vec2 center = vec2(aspect * 0.5, 0.5);
  float stars = starLayer(ss, 6.0, 1.0, 11.0, 0.25, center, warp)
              + starLayer(ss, 10.0, 0.5, 23.0, 0.30, center, warp) * 0.7
              + starLayer(ss, 16.0, 0.25, 37.0, 0.35, center, warp) * 0.45;

  vec3 col = nebula(s) + vec3(stars * (1.0 + warp * 0.6) * 0.85 + rippleGlow);
  col *= smoothstep(0.0, 0.3, uIntro) * (1.0 - 0.3 * uFocus) * mix(0.75, 1.0, pres);

  float cover;
  vec3 m = moon(s, aspect, cover);
  col = mix(col, m, cover);                        // the moon occludes the stars

  float lit = step(0.002, max(col.r, max(col.g, col.b)));
  outColor = vec4(col + ditherNoise(fc) * lit, 1.0);
}
`;

export const lunarScene: AmbientScene = {
  id: "lunar",
  fragment: FRAGMENT_PRELUDE + LUNAR_BODY,
  introMs: 1200,
  pointerTauMs: 150,
};
```

- [ ] **Step 3: 运行脚手架确认通过**

Run: `bash /tmp/aris-shader-check/run.sh`
Expected: `{"atelier":{...},"lunar":{"moonRisen":<3..60>,"moonSunk":<小于 moonRisen>,"titleArea":<小于 12>},"pass":true}`。

- [ ] **Step 4: lint 并提交**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npx eslint src/components/ambient
cd "$WT" && npx --prefix web prettier --write web/src/components/ambient/scenes/lunar.ts
git add web/src/components/ambient/scenes/lunar.ts
git commit -m "feat(web): lunar 月光与引力环境场景（月球 + 视差星空 + 引力透镜）"
```

---

### Task 9: Canvas 2D 星空降级形态（迁移 ParticleBackground）

**Files:**
- Create: `web/src/components/ambient/fallback-2d.ts`

**Interfaces:**
- Produces: `startStarfield2D(canvas: HTMLCanvasElement, animate: boolean): () => void`（`animate=false` 只画一帧静态画面；首帧绘制后写 `canvas.dataset.ready = "1"`；返回 stop 函数）

行为与现有 `components/theme/particle-background.tsx` 一致：70 颗白星、指针 120px 吸引增亮、滚动视差 40px、标签页隐藏时暂停。差异只有两点：主题判断移到调用方；canvas 尺寸交给 CSS（删除 `style.width/height` 赋值）。

- [ ] **Step 1: 实现**

`web/src/components/ambient/fallback-2d.ts`：

```ts
/* Canvas 2D starfield: lunar's `lite` (animated) and `still` (one static
 * frame) form, migrated from the former ParticleBackground. The caller
 * decides when it runs; canvas size comes from CSS (.ambient-stage canvas). */

const STAR_COUNT = 70;
const POINTER_RADIUS = 120;
const PARALLAX_RANGE = 40;
const MAX_DPR = 2;

interface Star {
  nx: number; // normalized x, 0..1
  ny: number; // normalized y, 0..1
  r: number;
  vx: number; // px per frame
  vy: number;
  phase: number;
}

export function startStarfield2D(canvas: HTMLCanvasElement, animate: boolean): () => void {
  const ctx = canvas.getContext("2d");
  if (!ctx) return () => {};

  const dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
  let width = window.innerWidth;
  let height = window.innerHeight;
  let raf = 0;
  let running = false;
  const pointer = { x: -9999, y: -9999 };

  const stars: Star[] = Array.from({ length: STAR_COUNT }, () => ({
    nx: Math.random(),
    ny: Math.random(),
    r: 0.6 + Math.random() * 1.2,
    vx: (Math.random() - 0.5) * 0.15,
    vy: (Math.random() - 0.5) * 0.15,
    phase: Math.random() * Math.PI * 2,
  }));

  const markReady = () => {
    if (!canvas.dataset.ready) canvas.dataset.ready = "1";
  };

  const drawStar = (x: number, y: number, s: Star, alpha: number, glow: number) => {
    ctx.globalAlpha = Math.min(alpha, 1);
    ctx.fillStyle = "#FFFFFF";
    ctx.beginPath();
    ctx.arc(x, y, s.r * glow, 0, Math.PI * 2);
    ctx.fill();
  };

  const drawStatic = () => {
    ctx.clearRect(0, 0, width, height);
    for (const s of stars) drawStar(s.nx * width, s.ny * height, s, 0.75, 1);
    ctx.globalAlpha = 1;
    markReady();
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
    markReady();
    raf = requestAnimationFrame(frame);
  };

  const resize = () => {
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    if (!animate) drawStatic();
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
    } else if (animate && !running) {
      raf = requestAnimationFrame(frame);
      running = true;
    }
  };

  resize();
  window.addEventListener("resize", resize);
  document.addEventListener("visibilitychange", onVisibility);
  if (animate) {
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    document.addEventListener("pointerout", onPointerOut);
    window.addEventListener("scroll", onScroll, { capture: true, passive: true });
    raf = requestAnimationFrame(frame);
    running = true;
  }

  return () => {
    cancelAnimationFrame(raf);
    running = false;
    window.removeEventListener("resize", resize);
    document.removeEventListener("visibilitychange", onVisibility);
    window.removeEventListener("pointermove", onPointerMove);
    document.removeEventListener("pointerout", onPointerOut);
    window.removeEventListener("scroll", onScroll, true);
    canvas.style.transform = "";
    ctx.clearRect(0, 0, width, height);
  };
}
```

- [ ] **Step 2: 验证并提交**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npx eslint src/components/ambient
cd "$WT" && npx --prefix web prettier --write web/src/components/ambient/fallback-2d.ts
git add web/src/components/ambient/fallback-2d.ts
git commit -m "refactor(web): Canvas 2D 星空迁移为环境舞台的降级形态"
```

Expected: tsc / eslint 无输出。运行时行为在 Task 11 的 `lite / still` 矩阵中验证。

---

### Task 10: AmbientStage 组件接线 + 样式 + 替换 ParticleBackground

**Files:**
- Create: `web/src/components/ambient/ambient-stage.tsx`
- Delete: `web/src/components/theme/particle-background.tsx`
- Modify: `web/src/app/layout.tsx`（import 与挂载点）
- Modify: `web/src/app/globals.css:297-302`（`.page-surface` 规则）、`web/src/app/globals.css:333-350`（`.particle-bg` 块）

**Interfaces:**
- Consumes:
  - `useMotionTier`、`downgradeMotionTier`（Task 5）
  - `bindAttention`、`createAttention`、`markActive`、`stepAttention`、`PRESENCE_BUSY`（Task 3）
  - `createGovernor`、`governorTick`、`resetGovernorWindow`（Task 2）
  - `emitAmbientPulse`、`onAmbientPulse`、`AmbientPulse`（Task 4）
  - `createRenderer`、`SceneProgram`（Task 6）；`atelierScene`（Task 7）；`lunarScene`（Task 8）；`startStarfield2D`（Task 9）
  - `PULSE_SLOTS`、`PULSE_KIND_CODE`、`AmbientScene`、`SceneId`（Task 6）
  - `useTheme`（`@/lib/theme`，已有）
- Produces: `AmbientStage()`（根布局挂载一次；DOM：`div.ambient-stage[aria-hidden] > canvas`，canvas 以 `key={mode}` 在 gl / fallback 之间重建）

- [ ] **Step 1: 实现组件**

`web/src/components/ambient/ambient-stage.tsx`：

```tsx
"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { useTheme } from "@/lib/theme";
import { downgradeMotionTier, useMotionTier } from "@/lib/motion/tier-store";
import {
  PRESENCE_BUSY,
  bindAttention,
  createAttention,
  markActive,
  stepAttention,
} from "@/lib/motion/attention";
import { createGovernor, governorTick, resetGovernorWindow } from "@/lib/motion/fps-governor";
import { emitAmbientPulse, onAmbientPulse, type AmbientPulse } from "@/lib/motion/pulse";
import { createRenderer, type SceneProgram } from "./gl";
import { startStarfield2D } from "./fallback-2d";
import { atelierScene } from "./scenes/atelier";
import { lunarScene } from "./scenes/lunar";
import { PULSE_KIND_CODE, PULSE_SLOTS, type AmbientScene, type SceneId } from "./scenes/common";

const SCENES: Record<SceneId, AmbientScene> = { atelier: atelierScene, lunar: lunarScene };
const RENDER_SCALE = 0.75;
const REDUCED_SCALE = 0.5;
const MAX_DPR = 2;
const AMBIENT_FRAME_MS = 1000 / 30;
/** Pulses keep the stage at full frame rate this long (atelier's wet phase). */
const PULSE_HOT_S = 2.2;
const FOCUS_TAU_MS = 300;
/** A press this recent anchors the route pulse; otherwise viewport center. */
const ROUTE_PULSE_WINDOW_MS = 1000;
const EMPTY_PULSE_START = -1000;

function sceneOf(theme: string | undefined): SceneId {
  return theme === "moonshot" ? "lunar" : "atelier";
}

/** Expo-out, close to --ease-drift cubic-bezier(.16,1,.3,1). */
function easeDrift(p: number): number {
  return p >= 1 ? 1 : 1 - Math.pow(2, -10 * p);
}

/**
 * Ambient Stage (web/CONTEXT.md「Motion」): the single fixed backdrop under
 * all content. `immersive` renders the active skin's WebGL scene; `lite` /
 * `still` fall back to CSS textures (atelier) or the Canvas 2D starfield
 * (lunar). Any GL failure downgrades the Motion Tier, which remounts the
 * canvas (key={mode}) for the fallback path.
 */
export function AmbientStage() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const { theme } = useTheme();
  const tier = useMotionTier();
  const pathname = usePathname();
  const mode = tier === "immersive" ? "gl" : "fallback";
  const sceneRef = useRef<SceneId>("atelier");
  const prevPathRef = useRef(pathname);
  const lastDownRef = useRef({ x: 0, y: 0, t: Number.NEGATIVE_INFINITY });

  // Follow the skin via <html data-theme> (written before setState), so the
  // very first effect already sees the real skin, not the hydration default.
  useEffect(() => {
    sceneRef.current = sceneOf(document.documentElement.dataset.theme);
  }, [theme]);

  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      lastDownRef.current = { x: e.clientX, y: e.clientY, t: performance.now() };
    };
    window.addEventListener("pointerdown", onDown, { capture: true, passive: true });
    return () => window.removeEventListener("pointerdown", onDown, { capture: true });
  }, []);

  // Every route change pulses the stage at the press that caused it.
  useEffect(() => {
    if (prevPathRef.current === pathname) return;
    prevPathRef.current = pathname;
    const down = lastDownRef.current;
    const recent = performance.now() - down.t < ROUTE_PULSE_WINDOW_MS;
    emitAmbientPulse(recent ? { kind: "route", x: down.x, y: down.y } : { kind: "route" });
  }, [pathname]);

  // immersive: WebGL scene loop.
  useEffect(() => {
    if (mode !== "gl") return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const renderer = createRenderer(canvas);
    if (!renderer) {
      downgradeMotionTier("lite");
      return;
    }

    const t0 = performance.now();
    const pulses = new Float32Array(PULSE_SLOTS * 4);
    for (let i = 0; i < PULSE_SLOTS; i++) pulses[i * 4 + 2] = EMPTY_PULSE_START;
    const pointerTarget = { x: 0, y: 0 };
    const pointer = { x: 0, y: 0 };
    let pulseSlot = 0;
    let scale = RENDER_SCALE;
    let governor = createGovernor(t0);
    let attention = createAttention();
    let activeScene: SceneId | null = null;
    let program: SceneProgram | null = null;
    let introStart = t0;
    let introMs = 1;
    let scroll = 0;
    let focus = 0;
    let focusTarget = 0;
    let drift = 0;
    let lastFrame = t0;
    let lastRender = Number.NEGATIVE_INFINITY;
    let raf = 0;
    let alive = true;

    const fail = (reason: unknown) => {
      console.warn("[ambient] WebGL stage disabled:", reason);
      alive = false;
      downgradeMotionTier("lite");
    };

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
      renderer.resize(
        Math.max(1, Math.floor(window.innerWidth * dpr * scale)),
        Math.max(1, Math.floor(window.innerHeight * dpr * scale)),
      );
      lastRender = Number.NEGATIVE_INFINITY;
    };

    const switchScene = (id: SceneId) => {
      program = renderer.use(id, SCENES[id].fragment);
      activeScene = id;
      introStart = performance.now();
      introMs = SCENES[id].introMs;
      governor = resetGovernorWindow(governor, introStart);
    };

    const onPulse = (p: AmbientPulse) => {
      if (p.kind === "focus-on") {
        focusTarget = 1;
        return;
      }
      if (p.kind === "focus-off") {
        focusTarget = 0;
        return;
      }
      if (p.kind === "intro") {
        introStart = performance.now();
        return;
      }
      const o = pulseSlot * 4;
      pulses[o] = (p.x ?? window.innerWidth / 2) / window.innerWidth;
      pulses[o + 1] = 1 - (p.y ?? window.innerHeight / 2) / window.innerHeight;
      pulses[o + 2] = (performance.now() - t0) / 1000;
      pulses[o + 3] = PULSE_KIND_CODE[p.kind];
      pulseSlot = (pulseSlot + 1) % PULSE_SLOTS;
    };

    const onPointerMove = (e: PointerEvent) => {
      pointerTarget.x = (e.clientX / window.innerWidth) * 2 - 1;
      pointerTarget.y = 1 - (e.clientY / window.innerHeight) * 2;
    };
    const onScroll = (e: Event) => {
      const el = e.target instanceof Element ? e.target : document.scrollingElement;
      if (el) scroll = el.scrollTop / window.innerHeight;
    };
    const onLost = () => fail("context lost");

    const frame = (now: number) => {
      if (!alive) return;
      raf = requestAnimationFrame(frame);

      const verdict = governorTick(governor, now);
      governor = verdict.state;
      if (verdict.action === "reduce-scale") {
        scale = REDUCED_SCALE;
        resize();
      } else if (verdict.action === "downgrade") {
        fail("frame rate below budget");
        return;
      }

      const dt = Math.min(now - lastFrame, 100);
      lastFrame = now;
      attention = stepAttention(attention, now, dt);
      const presence = attention.presence;
      const presenceN = (presence - PRESENCE_BUSY) / (1 - PRESENCE_BUSY);

      const wanted = sceneRef.current;
      if (wanted !== activeScene) {
        try {
          switchScene(wanted);
        } catch (err) {
          fail(err);
          return;
        }
      }
      const scene = SCENES[wanted];

      // The atelier lamp stops following while the user is busy.
      if (!(wanted === "atelier" && presenceN < 0.5)) {
        const k = 1 - Math.exp(-dt / scene.pointerTauMs);
        pointer.x += (pointerTarget.x - pointer.x) * k;
        pointer.y += (pointerTarget.y - pointer.y) * k;
      }
      focus += (focusTarget - focus) * (1 - Math.exp(-dt / FOCUS_TAU_MS));
      drift += (dt / 1000) * (0.5 + 0.5 * presenceN) * (1 - 0.6 * focus);

      const t = (now - t0) / 1000;
      const introP = Math.min((now - introStart) / introMs, 1);
      let hot = introP < 1 || Math.abs(focusTarget - focus) > 0.01;
      for (let i = 0; i < PULSE_SLOTS && !hot; i++) hot = t - pulses[i * 4 + 2] < PULSE_HOT_S;
      if (!hot && now - lastRender < AMBIENT_FRAME_MS - 1) return;
      lastRender = now;

      if (!program) return;
      program.set2f("uResolution", canvas.width, canvas.height);
      program.set1f("uTime", t);
      program.set1f("uDrift", drift);
      program.set2f("uPointer", pointer.x, pointer.y);
      program.set1f("uScroll", scroll);
      program.set1f("uPresence", presence);
      program.set1f("uFocus", focus);
      program.set1f("uIntro", easeDrift(introP));
      program.set4fv("uPulse", pulses);
      renderer.draw();
      if (!canvas.dataset.ready) canvas.dataset.ready = "1";
    };

    const onVisibility = () => {
      cancelAnimationFrame(raf);
      if (document.hidden) return;
      const now = performance.now();
      governor = resetGovernorWindow(governor, now);
      lastFrame = now;
      raf = requestAnimationFrame(frame);
    };

    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    window.addEventListener("scroll", onScroll, { capture: true, passive: true });
    document.addEventListener("visibilitychange", onVisibility);
    canvas.addEventListener("webglcontextlost", onLost);
    const unbindAttention = bindAttention(() => {
      attention = markActive(attention, performance.now());
    });
    const offPulse = onAmbientPulse(onPulse);
    raf = requestAnimationFrame(frame);

    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("scroll", onScroll, { capture: true });
      document.removeEventListener("visibilitychange", onVisibility);
      // Detach before dispose(): loseContext() fires `webglcontextlost`.
      canvas.removeEventListener("webglcontextlost", onLost);
      unbindAttention();
      offPulse();
      renderer.dispose();
    };
  }, [mode]);

  // lite / still: lunar uses the Canvas 2D starfield; atelier is pure CSS.
  useEffect(() => {
    if (mode !== "fallback") return;
    const canvas = canvasRef.current;
    if (!canvas || sceneOf(document.documentElement.dataset.theme) !== "lunar") return;
    return startStarfield2D(canvas, tier === "lite");
  }, [mode, theme, tier]);

  return (
    <div aria-hidden="true" className="ambient-stage">
      <canvas key={mode} ref={canvasRef} />
    </div>
  );
}
```

- [ ] **Step 2: 样式**

`web/src/app/globals.css` 第 297-302 行（moonshot 的 `.page-surface` 规则）整体替换为：

```css
/* Page roots are transparent in both skins so the Ambient Stage (paper or
 * starfield) shows through. Only full-page containers carry the marker
 * class; small widgets with bg-background (switch knob, tabs) stay opaque. */
:where(.page-surface) {
  background-color: transparent;
}
```

第 333-350 行（`/* Starfield container ... */` 到 `.particle-bg canvas { ... }`）整体替换为：

```css
/* Ambient Stage (components/ambient): the fixed backdrop under all content.
 * Its own background is the skin's --background, so nothing gaps before the
 * first GL frame; the canvas fades in once it has drawn (data-ready). */
.ambient-stage {
  position: fixed;
  inset: 0;
  z-index: -10;
  pointer-events: none;
  overflow: hidden;
  background-color: var(--background);
}
.ambient-stage canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  transition: opacity 500ms ease;
}
.ambient-stage canvas[data-ready] {
  opacity: 1;
}

/* Atelier lite/still: static paper grain (sepia fractal noise, ≤6% alpha,
 * fibers stretched horizontally like the GL scene). */
html[data-theme="anthropic"]:not([data-motion-tier="immersive"]) .ambient-stage {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='240' height='240'%3E%3Cfilter id='p'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.2 0.9' numOctaves='3' stitchTiles='stitch'/%3E%3CfeColorMatrix values='0 0 0 0 0.43 0 0 0 0 0.33 0 0 0 0 0.25 0 0 0 0.06 0'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23p)'/%3E%3C/svg%3E");
}

/* Lunar lite/still: static moon silhouette matching the GL geometry
 * (radius 90vh; center 30vh in from the right edge, 5vh above the bottom;
 * lit from the upper right; brightness kept under the 0.22 cap). */
html[data-theme="moonshot"]:not([data-motion-tier="immersive"]) .ambient-stage::after {
  content: "";
  position: absolute;
  right: -60vh;
  bottom: -85vh;
  width: 180vh;
  height: 180vh;
  border-radius: 50%;
  background: radial-gradient(
    circle at 62% 28%,
    rgb(255 238 217 / 0.16),
    rgb(140 140 150 / 0.07) 42%,
    rgb(12 12 14 / 0.96) 68%,
    #050505
  );
}
```

- [ ] **Step 3: 根布局换挂载点，删除旧组件**

`web/src/app/layout.tsx`：

```tsx
// 原：import { ParticleBackground } from "@/components/theme/particle-background";
import { AmbientStage } from "@/components/ambient/ambient-stage";
```

```tsx
// 原：<ParticleBackground />
<AmbientStage />
```

```bash
cd "$WT" && git rm -q web/src/components/theme/particle-background.tsx
grep -rn "particle-background\|ParticleBackground\|particle-bg" web/src || echo "no references"
```

Expected: 输出 `no references`。

- [ ] **Step 4: 验证**

```bash
cd "$WT/web" && npx tsc --noEmit -p . && npm run lint && npm run test
env -u NODE_OPTIONS npm run build > /tmp/aris-build.log 2>&1; echo "exit=$?"
grep -c 'class="ambient-stage"' out/login/index.html
```

Expected: tsc 无输出；lint 0 error（warning 不阻塞，但本任务新增文件不应产生 warning）；test 全部通过；`exit=0`；grep 输出 `1`。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && npx --prefix web prettier --write web/src/components/ambient/ambient-stage.tsx web/src/app/layout.tsx web/src/app/globals.css
git add web/src/components/ambient/ambient-stage.tsx web/src/app/layout.tsx web/src/app/globals.css
git commit -m "feat(web): AmbientStage 替换 ParticleBackground，两主题页面底板透明"
```

---

### Task 11: 运行时矩阵验证 + 包体积核对

**Files:** 无代码改动（临时文件：`/tmp/aris-verify/run.sh`、`$WT/web/out/__theme.html`，均不提交）

验证两主题 × 三种环境在真实构建产物上的档位、首帧与降级路径。期望矩阵：

| 主题 | 环境参数 | `data-motion-tier` | canvas `data-ready` | `[ambient]` 警告 |
|---|---|---|---|---|
| anthropic | 默认 | `immersive` | 有 | 无 |
| moonshot | 默认 | `immersive` | 有 | 无 |
| anthropic | `--force-prefers-reduced-motion` | `still` | 无（纯 CSS 纸纹） | 无 |
| moonshot | `--force-prefers-reduced-motion` | `still` | 有（2D 静态帧） | 无 |
| anthropic | `--disable-webgl` | `lite` | 无（纯 CSS 纸纹） | 无 |
| moonshot | `--disable-webgl` | `lite` | 有（2D 动态星空） | 无 |

- [ ] **Step 1: 启动静态服务**

```bash
cd "$WT/web/out" && ln -sfn . web
printf '<!doctype html><script>localStorage.setItem("theme","moonshot");location.replace("/web/login/")</script>' > __theme.html
(python3 -m http.server 4173 > /tmp/aris-http.log 2>&1 &)
sleep 1 && curl -s -o /dev/null -w "%{http_code}\n" http://localhost:4173/web/login/
```

Expected: `200`。

- [ ] **Step 2: 写并运行矩阵脚本**

`/tmp/aris-verify/run.sh`：

```bash
#!/bin/bash
CH=~/Library/Caches/ms-playwright/chromium_headless_shell-1228/chrome-headless-shell-mac-arm64/chrome-headless-shell
BASE=http://localhost:4173/web
run() {
  local label=$1 url=$2; shift 2
  local ud; ud=$(mktemp -d)
  "$CH" --headless --use-angle=swiftshader --enable-unsafe-swiftshader --window-size=1280,800 \
    --user-data-dir="$ud" --enable-logging=stderr --v=0 --virtual-time-budget=5000 "$@" \
    --dump-dom "$url" > /tmp/aris-verify/$label.html 2> /tmp/aris-verify/$label.err
  local tier ready warn errors
  tier=$(grep -o 'data-motion-tier="[a-z]*"' /tmp/aris-verify/$label.html | head -1)
  ready=$(grep -c '<canvas[^>]*data-ready="1"' /tmp/aris-verify/$label.html)
  warn=$(grep -c '\[ambient\]' /tmp/aris-verify/$label.err)
  errors=$(grep -c 'Uncaught' /tmp/aris-verify/$label.err)
  echo "$label | $tier | ready=$ready | ambientWarn=$warn | uncaught=$errors"
}
run anthropic-default "$BASE/login/"
run moonshot-default "$BASE/__theme.html"
run anthropic-reduced "$BASE/login/" --force-prefers-reduced-motion
run moonshot-reduced "$BASE/__theme.html" --force-prefers-reduced-motion
run anthropic-nowebgl "$BASE/login/" --disable-webgl
run moonshot-nowebgl "$BASE/__theme.html" --disable-webgl
```

Run: `mkdir -p /tmp/aris-verify && bash /tmp/aris-verify/run.sh`

Expected（与上表一致）：

```
anthropic-default | data-motion-tier="immersive" | ready=1 | ambientWarn=0 | uncaught=0
moonshot-default | data-motion-tier="immersive" | ready=1 | ambientWarn=0 | uncaught=0
anthropic-reduced | data-motion-tier="still" | ready=0 | ambientWarn=0 | uncaught=0
moonshot-reduced | data-motion-tier="still" | ready=1 | ambientWarn=0 | uncaught=0
anthropic-nowebgl | data-motion-tier="lite" | ready=0 | ambientWarn=0 | uncaught=0
moonshot-nowebgl | data-motion-tier="lite" | ready=1 | ambientWarn=0 | uncaught=0
```

排障：
- `default` 行出现 `lite` 且 `ambientWarn=1`：看 `/tmp/aris-verify/<label>.err` 中 `[ambient]` 那一行的原因。若是 `shader compile failed`，修 shader 后重跑 Task 7 / 8 的脚手架；若是 `frame rate below budget`，说明 swiftshader 软渲染偏慢，改用 `--window-size=800,600` 重跑确认（真实 GPU 不受影响）。
- `moonshot-*` 的 tier 正确但 `ready=0`：检查 `__theme.html` 是否已重定向到 `/web/login/`（dump 内容里应有登录页 DOM）。

- [ ] **Step 3: 包体积增量**

```bash
cd "$WT/web" && for f in $(find out/_next/static/chunks -name '*.js'); do gzip -9c "$f" | wc -c; done | awk '{s+=$1} END {print s}' > /tmp/aris-bundle-after.txt
echo "delta=$(( $(cat /tmp/aris-bundle-after.txt) - $(cat /tmp/aris-bundle-baseline.txt) )) bytes"
```

Expected: `delta` ≤ 12288。

- [ ] **Step 4: 停止服务并清理临时文件**

```bash
lsof -nP -ti tcp:4173 -sTCP:LISTEN | xargs kill
cd "$WT/web/out" && unlink web && unlink __theme.html
```

（`out/` 是构建产物、已被 gitignore，无需提交。不要用 `pkill -f`：它会匹配调用者自身的命令行。）

---

### Task 12: 文档同步 + 收尾

**Files:**
- Modify: `web/CONTEXT.md`（Theme 词条 + 新增 Motion 小节）
- Modify: `docs/agents/web-frontend.md`（新增「动效与环境舞台契约」小节）
- Modify: `docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md`（§4.2、§7.4 与新增 §12）

- [ ] **Step 1: `web/CONTEXT.md`**

Theme 词条中这一句：

```
整页根容器携带 `page-surface` 标记类，moonshot 下背景透明以透出星空；小组件不携带。
```

改为：

```
整页根容器携带 `page-surface` 标记类，两套主题下背景均透明，以透出 Ambient Stage（纸面或星空）；小组件不携带。
```

文件末尾追加：

```markdown
## Motion（动效）

**Motion World（动效世界）**:
一个主题的完整动效人格，含 token、组件语法、环境场景与签名时刻。anthropic → `atelier`（纸与墨），moonshot → `lunar`（月光与引力）。
_Avoid_: animation theme, motion skin

**Motion Tier（动效档位）**:
`immersive | lite | still`，反映到 `<html data-motion-tier>`，是所有降级行为的单一事实来源。首帧前由内联脚本按减弱动态 / 省流量 / 低配 / 粗指针判定，Ambient Stage 挂载后补测 WebGL2；运行时只降不升，恢复需刷新。
_Avoid_: animation level, quality setting

**Ambient Stage（环境舞台）**:
根布局里唯一的 fixed 背景层（`components/ambient/ambient-stage.tsx`），`immersive` 档绘制当前世界的 WebGL 场景，`lite / still` 档退回 CSS 纸纹或 Canvas 2D 星空。
_Avoid_: particle background, background canvas

**Ambient Presence（环境存在感）**:
注意力信号输出的 `0.35..1`：用户打字 / 滚动 / 点击时背景退后，空闲 8s 后浮现。移动指针不算操作。
_Avoid_: focus mode, idle dimming

**Ambient Pulse（环境脉冲）**:
UI 事件经 `emitAmbientPulse()` 发给舞台的一次性效果请求（`primary` 墨滴 / 引力波纹、`route` 墨滴 / 跃迁、`focus-on/off` 聚焦、`intro` 重放入场）。路由切换由舞台自动发出 `route`。
_Avoid_: background event, fx trigger

**Signature Moment（签名时刻）**:
为单个高光场景专门编排的动效序列（主题转场、登录页、Dashboard、侧栏指示器、Playground 流式输出）。
_Avoid_: hero animation, special effect
```

- [ ] **Step 2: `docs/agents/web-frontend.md`**

在 `## i18n 布局稳定性契约` 之前插入：

```markdown
## 动效与环境舞台契约

> 设计见 `docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md`，术语见 [web/CONTEXT.md](../../web/CONTEXT.md)「Motion」。

- **档位唯一来源**：`<html data-motion-tier>`（`immersive | lite | still`）。JS 读 `useMotionTier()`（`@/lib/motion/tier-store`），CSS 用 `html[data-motion-tier="…"]`；新代码不要再自行 `matchMedia("(prefers-reduced-motion)")`。全局 reduced-motion 守卫保留作底线。
- **只降不升**：运行时只能调用 `downgradeMotionTier()`，不要提供"恢复"路径。
- **背景只有一个**：所有背景效果都在 `AmbientStage` 里实现；业务组件需要背景反馈时调用 `emitAmbientPulse()`（`@/lib/motion/pulse`），不要新建 canvas。路由切换的 `route` 脉冲由舞台自动发出。
- **`components/ambient/` 只用同目录相对导入**：shader 验证脚手架用 esbuild 直接打包这些模块，不解析 `@/` 别名。
- **shader 改动必须跑编译与像素断言**：用 chrome-headless-shell（`--use-angle=swiftshader --enable-unsafe-swiftshader`）编译并读回像素，断言 atelier 纸面亮度起伏 ≤ ±3%、lunar 月球亮度 ≤ 0.22。
```

- [ ] **Step 3: spec 回写**

`docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md`：

§4.2 中：

```
- 判定为纯函数 `resolveMotionTier(signals)`，内联脚本与运行时共用同一份逻辑的字符串化版本（与 `THEME_INIT_SCRIPT` 同样以常量字符串导出）。
```

改为：

```
- 判定为纯函数 `resolveMotionTier(signals)`；内联脚本 `MOTION_TIER_INIT_SCRIPT` 是它的手写 ES5 镜像（生产压缩使 `toString` 不可靠），由全组合对拍单测保证一致。
```

§5.3 中：

```
- **lite / still**：`body` 叠一层静态 SVG `feTurbulence` 纸纹（低透明度），无动画。
```

改为：

```
- **lite / still**：`.ambient-stage` 上叠一层静态 SVG `feTurbulence` 纸纹（低透明度），无动画。
```

§7.4 中：

```
- 导航点击 → `emitAmbientPulse('route', 导航项中心坐标)`。
```

改为：

```
- `route` 脉冲由 AmbientStage 在 `pathname` 变化时自动发出，位置取 1s 内最后一次 `pointerdown`（导航项点击即落在该项上），无近期点击时取视口中心。
```

文件末尾追加：

```markdown
## 12. 实现偏差记录

| PR | 偏差 | 原因 |
|---|---|---|
| 1 | 内联脚本改为 `resolveMotionTier` 的手写镜像 + 对拍单测 | 生产压缩使 `Function.prototype.toString` 不可靠 |
| 1 | 档位 DOM 存储与 hook 拆到 `tier-store.ts` | 保持 `tier.ts` 可在 node 下单测 |
| 1 | `route` 脉冲由舞台监听路由自动发出 | 覆盖所有跳转来源，PR3 无需接线 |
| 1 | 新增 `uDrift` 漂移时钟 uniform | 操作中减速、聚焦时再减速，避免缩放 `uTime` 造成跳变 |
| 1 | atelier 降级纸纹挂在 `.ambient-stage` 而非 `body` | 与 GL 场景同层，切换档位不叠加两层纹理 |
```

- [ ] **Step 4: 全量验证**

```bash
cd "$WT/web" && npm run lint && npm run test && npm run format:check
env -u NODE_OPTIONS npm run build > /tmp/aris-build.log 2>&1; echo "exit=$?"
```

Expected: lint 0 error；test 全部通过（含 Task 1-4 新增的 4 个测试文件）；format:check 通过；`exit=0`。

- [ ] **Step 5: 提交**

```bash
cd "$WT" && git add web/CONTEXT.md docs/agents/web-frontend.md docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md
git commit -m "docs(web): 同步动效档位与环境舞台术语、契约及 spec 偏差记录"
git status --short
```

Expected: 提交成功，`git status` 无输出。

- [ ] **Step 6: 交给用户真机复验**

请用户在本机浏览器打开 dev server 或构建产物，按下面清单验收：

1. anthropic：页面底下能看到细腻纸纹，移动鼠标时光线缓慢跟随；打字或滚动时停止跟随，静置 8 秒后恢复。
2. moonshot：右下角是被裁切的巨大月球，左上标题区保持纯黑；鼠标附近星点被「推开」；切换页面时星点短暂拉丝。
3. 两主题之间来回切换：背景场景随之切换并重放入场（台灯亮起 / 月出）。
4. 系统开启「减弱动态」后刷新：背景静止，没有任何动画。
5. 停留 5 分钟：风扇无明显变化，活动监视器里浏览器 GPU 占用没有持续走高。

然后询问用户：直接合并到 `master`，还是先提 PR。
