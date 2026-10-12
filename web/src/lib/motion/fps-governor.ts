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
