/** 门户动效的纯逻辑：循环守卫、链路曲线、熔断演示脚本。组件只负责把它们画出来。 */
import type { MotionTier } from "./motion/tier";

/** still 档或页面不可见时，JS 驱动的演示循环一律停下。 */
export function shouldRunLoop(tier: MotionTier, hidden: boolean): boolean {
  return tier !== "still" && !hidden;
}

/** 两端切线水平的三次贝塞尔，用于链路虚线。 */
export function curvePath(x1: number, y1: number, x2: number, y2: number): string {
  const mx = (x1 + x2) / 2;
  return `M${x1} ${y1}C${mx} ${y1} ${mx} ${y2} ${x2} ${y2}`;
}

export type BreakerCell = "idle" | "ok" | "err" | "rejected" | "probe";

export type BreakerFrame = {
  state: "closed" | "open" | "half_open";
  cells: readonly BreakerCell[];
  errorPct: number | null;
  captionKey: string;
  holdMs: number;
};

const CELLS = 10;
const PATTERN = "ggrgrrgrrr";
const CAPTION = "portal.cap.resilience.caption_";

/** spec §5.6 容错图解：逐格成败 → 60% 打开 → 503 快速失败 → 半开单探测 → 恢复。 */
export function breakerFrames(): readonly BreakerFrame[] {
  const frames: BreakerFrame[] = [];
  const idle = Array<BreakerCell>(CELLS).fill("idle");
  frames.push({
    state: "closed",
    cells: idle,
    errorPct: 0,
    captionKey: `${CAPTION}healthy`,
    holdMs: 900,
  });

  const cells = [...idle];
  let errors = 0;
  for (let i = 0; i < CELLS; i++) {
    const err = PATTERN[i] === "r";
    cells[i] = err ? "err" : "ok";
    if (err) errors++;
    frames.push({
      state: "closed",
      cells: [...cells],
      errorPct: Math.round((errors / (i + 1)) * 100),
      captionKey: `${CAPTION}${errors === 0 ? "healthy" : "counting"}`,
      holdMs: 300,
    });
  }

  frames.push({
    state: "open",
    cells: [...cells],
    errorPct: frames.at(-1)!.errorPct,
    captionKey: `${CAPTION}opened`,
    holdMs: 1300,
  });
  const rejected = Array<BreakerCell>(CELLS).fill("rejected");
  frames.push({
    state: "open",
    cells: rejected,
    errorPct: null,
    captionKey: `${CAPTION}rejecting`,
    holdMs: 1800,
  });
  const probing = [...rejected];
  probing[0] = "probe";
  frames.push({
    state: "half_open",
    cells: probing,
    errorPct: null,
    captionKey: `${CAPTION}half_open`,
    holdMs: 1500,
  });
  const recovered = Array<BreakerCell>(CELLS).fill("idle");
  recovered[0] = "ok";
  frames.push({
    state: "closed",
    cells: recovered,
    errorPct: 0,
    captionKey: `${CAPTION}recovered`,
    holdMs: 1400,
  });
  return frames;
}
