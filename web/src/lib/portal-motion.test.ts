import { describe, expect, it } from "vitest";
import { breakerFrames, curvePath, shouldRunLoop } from "./portal-motion";

describe("shouldRunLoop", () => {
  it("stops loops in still tier or when hidden", () => {
    expect(shouldRunLoop("immersive", false)).toBe(true);
    expect(shouldRunLoop("lite", false)).toBe(true);
    expect(shouldRunLoop("still", false)).toBe(false);
    expect(shouldRunLoop("immersive", true)).toBe(false);
  });
});

describe("curvePath", () => {
  it("builds a horizontal-tangent cubic", () =>
    expect(curvePath(0, 10, 100, 50)).toBe("M0 10C50 10 50 50 100 50"));
});

describe("breakerFrames", () => {
  const f = breakerFrames();

  it("opens right after the error rate reaches 60%", () => {
    const open = f.findIndex((x) => x.state === "open");
    expect(open).toBeGreaterThan(0);
    expect(f[open - 1].errorPct).toBe(60);
  });

  it("recovers through exactly one probe and ends closed", () => {
    const open = f.findIndex((x) => x.state === "open");
    const half = f.slice(open).find((x) => x.state === "half_open");
    expect(half?.cells.filter((c) => c === "probe")).toHaveLength(1);
    expect(f.at(-1)?.state).toBe("closed");
  });

  it("always renders ten cells with a caption", () =>
    f.forEach((x) => {
      expect(x.cells).toHaveLength(10);
      expect(x.captionKey).toMatch(/^portal\.cap\.resilience\.caption_/);
      expect(x.holdMs).toBeGreaterThan(0);
    }));
});
