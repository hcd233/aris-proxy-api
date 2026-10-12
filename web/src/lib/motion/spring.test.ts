import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  dampingRatio,
  sampleSpringEasing,
  springAtRest,
  springStep,
  toCssLinear,
  type SpringState,
} from "./spring";
import { ATELIER_SPRING, LUNAR_SPRING, MAGNETIC_SPRING } from "./tokens";

function settle(params: typeof LUNAR_SPRING, target: number, ms: number) {
  let s: SpringState = { x: 0, v: 0 };
  let peak = 0;
  for (let t = 0; t < ms; t += 16) {
    s = springStep(s, target, 0.016, params);
    peak = Math.max(peak, s.x);
  }
  return { s, peak };
}

describe("spring tokens", () => {
  it("atelier is critically damped, lunar and magnetic are underdamped", () => {
    expect(dampingRatio(ATELIER_SPRING)).toBeCloseTo(0.99, 2);
    expect(dampingRatio(LUNAR_SPRING)).toBeCloseTo(0.69, 2);
    expect(dampingRatio(MAGNETIC_SPRING)).toBeCloseTo(0.58, 2);
  });
});

describe("springStep", () => {
  it("reaches the target and comes to rest", () => {
    const { s } = settle(LUNAR_SPRING, 4, 2000);
    expect(springAtRest(s, 4)).toBe(true);
  });

  it("critically damped spring never overshoots", () => {
    expect(settle(ATELIER_SPRING, 1, 2000).peak).toBeLessThanOrEqual(1.0005);
  });

  it("lunar spring overshoots about 5%", () => {
    const { peak } = settle(LUNAR_SPRING, 1, 2000);
    expect(peak).toBeGreaterThan(1.04);
    expect(peak).toBeLessThan(1.06);
  });

  it("is stable for long frames (tab resume)", () => {
    const s = springStep({ x: 0, v: 0 }, 1, 0.25, MAGNETIC_SPRING);
    expect(Number.isFinite(s.x)).toBe(true);
    expect(Math.abs(s.x)).toBeLessThan(2);
  });
});

describe("sampleSpringEasing", () => {
  it("starts at 0, ends at 1 and keeps the ~5% lunar overshoot", () => {
    const samples = sampleSpringEasing(LUNAR_SPRING, 40);
    expect(samples).toHaveLength(40);
    expect(samples[0]).toBe(0);
    expect(samples[samples.length - 1]).toBe(1);
    const peak = Math.max(...samples);
    expect(peak).toBeGreaterThanOrEqual(1.04);
    expect(peak).toBeLessThanOrEqual(1.06);
  });

  it("--ease-orbit in motion-moonshot.css is generated from LUNAR_SPRING", () => {
    const css = readFileSync(new URL("../../styles/motion-moonshot.css", import.meta.url), "utf-8")
      .replace(/\s+/g, " ")
      .replace(/\(\s+/g, "(")
      .replace(/,?\s+\)/g, ")");
    expect(css).toContain(`--ease-orbit: ${toCssLinear(sampleSpringEasing(LUNAR_SPRING, 40))};`);
  });
});
