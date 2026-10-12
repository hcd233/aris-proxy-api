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

export function isMotionTier(x: unknown): x is MotionTier {
  return x === "immersive" || x === "lite" || x === "still";
}

/* Inline init script (FOUC prevention, same pattern as THEME_INIT_SCRIPT):
 * runs synchronously before first paint and writes <html data-motion-tier>.
 * A hand-written ES5 mirror of resolveMotionTier — production minification
 * makes Function.prototype.toString unreliable — kept in lockstep by the
 * full-matrix parity test in tier.test.ts. The tier controls motion intensity
 * only; the page backdrop remains the theme's solid --background color. */
export const MOTION_TIER_INIT_SCRIPT =
  "(function(){var d=document.documentElement;try{var n=navigator,c=n.connection," +
  "m=function(q){return window.matchMedia(q).matches};" +
  "d.dataset.motionTier=m('(prefers-reduced-motion: reduce)')?'still':" +
  "((c&&c.saveData)||m('(pointer: coarse)')||" +
  "(n.deviceMemory!==undefined&&n.deviceMemory<=4)||" +
  "(n.hardwareConcurrency!==undefined&&n.hardwareConcurrency<=4))?'lite':'immersive';" +
  "}catch(e){d.dataset.motionTier='lite';}})();";
