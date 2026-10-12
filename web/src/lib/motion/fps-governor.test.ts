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
