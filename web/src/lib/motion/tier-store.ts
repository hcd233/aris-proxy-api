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
  const next = lowerTier(readTier(), proposed);
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
