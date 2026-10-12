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
              const s = {
                reducedMotion,
                saveData,
                coarsePointer,
                deviceMemory,
                hardwareConcurrency,
              };
              expect(runInitScript(s), JSON.stringify(s)).toBe(resolveMotionTier(s));
            }
  });

  it("falls back to lite when detection throws", () => {
    expect(runInitScript(BASE, { throwOnMatchMedia: true })).toBe("lite");
  });
});
